package network

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/quic-go-utls/http3"
	"github.com/bogdanfinn/quic-go-utls/quicvarint"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	utls "github.com/bogdanfinn/utls"
)

func TestChrome152QUICClientHelloEvent(t *testing.T) {
	q := utls.UQUICClient(&utls.QUICConfig{TLSConfig: &utls.Config{ServerName: "localhost", MinVersion: utls.VersionTLS13, OmitEmptyPsk: true}}, utls.HelloCustom)
	spec, err := profiles.Chrome_152_PSK.GetQUICClientHelloSpec()()
	if err != nil {
		t.Fatal(err)
	}
	if err = q.ApplyPreset(&spec); err != nil {
		t.Fatal(err)
	}
	q.SetTransportParameters([]byte{0x0f, 0})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	defer q.Close()
	if err = q.Start(ctx); err != nil {
		t.Fatal(err)
	}
	for {
		e := q.NextEvent()
		t.Logf("event %v bytes %d", e.Kind, len(e.Data))
		if e.Kind == utls.QUICWriteData {
			if len(e.Data) == 0 {
				t.Fatal("empty ClientHello")
			}
			return
		}
		if e.Kind == utls.QUICNoEvent {
			t.Fatal("no ClientHello")
		}
	}
}

func TestChrome152QUICColdReuseAndResumption(t *testing.T) {
	for _, tc := range []struct {
		name, network, address string
		size                   int
	}{{"IPv4", "udp4", "127.0.0.1:0", 1250}, {"IPv6", "udp6", "[::1]:0", 1230}} {
		t.Run(tc.name, func(t *testing.T) { testChrome152QUICColdReuseAndResumption(t, tc.network, tc.address, tc.size) })
	}
}

func testChrome152QUICColdReuseAndResumption(t *testing.T, network, address string, initialSize int) {
	certServer := httptest.NewTLSServer(nil)
	cert := certServer.TLS.Certificates[0]
	roots := x509.NewCertPool()
	roots.AddCert(certServer.Certificate())
	certServer.Close()
	udp, err := net.ListenPacket(network, address)
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	type observed struct {
		resumed bool
		peer    string
	}
	observations := make(chan observed, 3)
	server := &http3.Server{TLSConfig: &utls.Config{Certificates: []utls.Certificate{{Certificate: cert.Certificate, PrivateKey: cert.PrivateKey}}}, EnableDatagrams: true, Handler: fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
		observations <- observed{r.TLS.DidResume, r.RemoteAddr}
		io.WriteString(w, "h3 ok")
	})}
	capture := &initialCapture{PacketConn: udp}
	go server.Serve(capture)
	defer server.Close()
	transport, err := NewTLSClientTransport(profiles.Chrome_152_PSK, tlsclient.WithProtocolRacing(), tlsclient.WithTransportOptions(&tlsclient.TransportOptions{RootCAs: roots}))
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var first observed
	for i := 0; i < 3; i++ {
		if i == 2 {
			transport.CloseIdleConnections()
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", "https://"+udp.LocalAddr().String()+"/", nil)
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || string(data) != "h3 ok" || resp.ProtoMajor != 3 {
			t.Fatalf("request %d: %s %q %v", i, resp.Proto, data, err)
		}
		got := <-observations
		if i == 0 {
			first = got
			if got.resumed {
				t.Fatal("cold connection resumed")
			}
		}
		if i == 1 && got.peer != first.peer {
			t.Fatal("same-origin request opened a second connection")
		}
		if i == 2 && !got.resumed {
			t.Fatal("new QUIC connection did not resume")
		}
	}
	hellos := capture.hellos(t)
	capture.mu.Lock()
	firstPacketSize := len(capture.packets[0])
	capture.mu.Unlock()
	if firstPacketSize != initialSize {
		t.Fatalf("Initial UDP size %d, want %d", firstPacketSize, initialSize)
	}
	if len(hellos) != 2 {
		t.Fatalf("handshakes=%d, want cold + resumed", len(hellos))
	}
	for i, h := range hellos {
		assertChromeWireExtensions(t, h, "quic", i == 1)
		want := "q13i0312h3_55b375c5d22e_eb028bd37c08"
		if i == 1 {
			want = "q13i0313h3_55b375c5d22e_40246181ac92"
			if h.order[len(h.order)-1] != 41 {
				t.Fatal("PSK must be last")
			}
		}
		if got := wireJA4(t, h, 'q'); got != want {
			t.Fatalf("JA4 %s, want %s", got, want)
		}
		if len(h.sessionID) != 0 {
			t.Fatal("QUIC sent a legacy session ID")
		}
		if !slices.Equal(h.ciphers, []uint16{0x1301, 0x1302, 0x1303}) {
			t.Fatal("QUIC cipher order changed")
		}
		if hex.EncodeToString(h.extensions[13]) != "0012040308040401050308050501080606010201" {
			t.Fatal("QUIC signature algorithm order changed")
		}
		params := map[uint64]string{}
		b := h.extensions[57]
		for len(b) > 0 {
			id, n, err := quicvarint.Parse(b)
			if err != nil {
				t.Fatal(err)
			}
			length, m, err := quicvarint.Parse(b[n:])
			if err != nil {
				t.Fatal(err)
			}
			end := n + m + int(length)
			if end > len(b) {
				t.Fatal("truncated transport parameter")
			}
			if _, ok := params[id]; ok {
				t.Fatal("duplicate transport parameter")
			}
			params[id] = hex.EncodeToString(b[n+m : end])
			b = b[end:]
		}
		for id, want := range map[uint64]string{1: "800493e0", 3: "45c0", 4: "80f00000", 5: "80600000", 6: "80600000", 7: "80600000", 8: "4064", 9: "4067", 15: "", 32: "80010000", 0x3128: "4f5249474e4f4950"} {
			got, ok := params[id]
			if !ok || got != want {
				t.Fatalf("parameter %#x=%q present=%v, want %q", id, got, ok, want)
			}
			delete(params, id)
		}
		version, err := hex.DecodeString(params[17])
		if err != nil || len(version) != 12 || hex.EncodeToString(version[:4]) != "00000001" {
			t.Fatal("invalid QUIC version information")
		}
		delete(params, 17)
		if i == 1 {
			encoded, err := hex.DecodeString(params[0x3127])
			if err != nil {
				t.Fatal(err)
			}
			rtt, n, err := quicvarint.Parse(encoded)
			if err != nil || n != len(encoded) || rtt == 0 || rtt > 8_000_000 {
				t.Fatalf("invalid remembered RTT: %x", encoded)
			}
			delete(params, 0x3127)
		}
		if len(params) != 1 {
			t.Fatalf("unexpected transport parameters: %v", params)
		}
		for id := range params {
			if id%31 != 27 {
				t.Fatalf("invalid GREASE transport parameter %#x", id)
			}
		}
	}
}
