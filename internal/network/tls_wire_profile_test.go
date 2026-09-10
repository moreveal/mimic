package network

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"golang.org/x/crypto/cryptobyte"
)

func assertChromeWireExtensions(t *testing.T, h wireHello, kind string, resumed bool) {
	t.Helper()
	raw, err := os.ReadFile("testdata/chrome152_network_wire.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Hello struct{ Extensions map[string]any }
	}
	if err = json.Unmarshal(fixture[kind], &rows); err != nil {
		t.Fatal(err)
	}
	i := 0
	if resumed {
		i = 1
	}
	want := rows[i].Hello.Extensions
	if _, ok := h.extensions[0]; !ok {
		delete(want, "0")
	}
	got := map[string]any{}
	for id, raw := range h.extensions {
		if isGREASE(id) {
			continue
		}
		var value any = hex.EncodeToString(raw)
		s := cryptobyte.String(raw)
		switch id {
		case 10, 13, 43:
			var list cryptobyte.String
			ok := false
			if id == 43 {
				ok = s.ReadUint8LengthPrefixed(&list)
			} else {
				ok = s.ReadUint16LengthPrefixed(&list)
			}
			if !ok || !s.Empty() {
				t.Fatal("invalid algorithm vector")
			}
			items := []uint16{}
			for !list.Empty() {
				var n uint16
				if !list.ReadUint16(&n) {
					t.Fatal("invalid algorithm")
				}
				if !isGREASE(n) {
					items = append(items, n)
				}
			}
			value = items
		case 51:
			var list cryptobyte.String
			if !s.ReadUint16LengthPrefixed(&list) || !s.Empty() {
				t.Fatal("invalid key shares")
			}
			items := [][2]uint16{}
			for !list.Empty() {
				var group uint16
				var key cryptobyte.String
				if !list.ReadUint16(&group) || !list.ReadUint16LengthPrefixed(&key) {
					t.Fatal("invalid keyshare")
				}
				if !isGREASE(group) {
					items = append(items, [2]uint16{group, uint16(len(key))})
				}
			}
			value = items
		case 51764:
			var list cryptobyte.String
			if !s.ReadUint16LengthPrefixed(&list) || !s.Empty() {
				t.Fatal("invalid trust anchors")
			}
			items := []string{}
			for !list.Empty() {
				var id cryptobyte.String
				if !list.ReadUint8LengthPrefixed(&id) {
					t.Fatal("invalid trust anchor")
				}
				items = append(items, hex.EncodeToString(id))
			}
			slices.Sort(items)
			value = items
		case 65037:
			if len(raw) != 186 && len(raw) != 218 && len(raw) != 250 && len(raw) != 282 {
				t.Fatalf("ECH length %d", len(raw))
			}
			value = map[string]string{"kdfAEAD": hex.EncodeToString(raw[:5])}
		case 41:
			if h.order[len(h.order)-1] != 41 {
				t.Fatal("PSK is not last")
			}
			value = "session-specific PSK"
		case 57:
			// Live connection parameters have a separate on-wire regression.
			delete(want, "57")
			continue
		}
		got[strconv.Itoa(int(id))] = value
	}
	// Convert integer slices to JSON number arrays for comparison with the fixture.
	b, _ := json.Marshal(got)
	var normalized map[string]any
	json.Unmarshal(b, &normalized)
	if !reflect.DeepEqual(normalized, want) {
		for id, value := range want {
			if !reflect.DeepEqual(normalized[id], value) {
				t.Errorf("%s extension %s: got %v want %v", kind, id, normalized[id], value)
			}
		}
		for id := range normalized {
			if _, ok := want[id]; !ok {
				t.Errorf("unexpected extension %s", id)
			}
		}
	}
}

type helloListener struct {
	net.Listener
	mu          sync.Mutex
	connections []*helloConn
}
type helloConn struct {
	net.Conn
	mu   sync.Mutex
	data []byte
}

func (l *helloListener) Accept() (net.Conn, error) {
	c, e := l.Listener.Accept()
	if e != nil {
		return nil, e
	}
	r := &helloConn{Conn: c}
	l.mu.Lock()
	l.connections = append(l.connections, r)
	l.mu.Unlock()
	return r, nil
}
func (c *helloConn) Read(b []byte) (int, error) {
	n, e := c.Conn.Read(b)
	c.mu.Lock()
	if len(c.data) < 65536 {
		c.data = append(c.data, b[:n]...)
	}
	c.mu.Unlock()
	return n, e
}
func (l *helloListener) hellos(t *testing.T) []wireHello {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	var hs []wireHello
	for _, c := range l.connections {
		c.mu.Lock()
		data := slices.Clone(c.data)
		c.mu.Unlock()
		var raw []byte
		for len(data) >= 5 && data[0] == 22 {
			n := int(data[3])<<8 | int(data[4])
			if len(data) < 5+n {
				t.Fatal("truncated TLS record")
			}
			raw = append(raw, data[5:5+n]...)
			data = data[5+n:]
			if len(raw) >= 4 && len(raw) >= 4+int(raw[1])<<16+int(raw[2])<<8+int(raw[3]) {
				break
			}
		}
		hs = append(hs, parseWireHello(t, raw))
	}
	return hs
}

func TestChrome152TLSColdAndResumedWire(t *testing.T) {
	resumed := make(chan bool, 2)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { resumed <- r.TLS.DidResume; io.WriteString(w, "ok") }))
	capture := &helloListener{Listener: server.Listener}
	server.Listener = capture
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	tr, err := NewTLSClientTransport(profiles.Chrome_152_PSK, tlsclient.WithDisableHttp3(), tlsclient.WithTransportOptions(&tlsclient.TransportOptions{RootCAs: roots}))
	if err != nil {
		t.Fatal(err)
	}
	defer tr.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		_, err = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if got := <-resumed; got != (i == 1) {
			t.Fatalf("request %d resumed=%v", i, got)
		}
		tr.CloseIdleConnections()
	}
	hs := capture.hellos(t)
	if len(hs) != 2 {
		t.Fatalf("connections %d", len(hs))
	}
	for i, h := range hs {
		assertChromeWireExtensions(t, h, "tls", i == 1)
		want := "t13i1517h2_8daaf6152771_4980c97edce0"
		if i == 1 {
			want = "t13i1518h2_8daaf6152771_3d1b1b7bef36"
		}
		if got := wireJA4(t, h, 't'); got != want {
			t.Fatalf("JA4 %s want %s", got, want)
		}
	}
}
