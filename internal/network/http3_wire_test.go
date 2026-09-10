package network

import (
	"context"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	quic "github.com/bogdanfinn/quic-go-utls"
	"github.com/bogdanfinn/quic-go-utls/quicvarint"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	utls "github.com/bogdanfinn/utls"
)

func TestChrome152HTTP3ControlWireOrderAndRequestPriorities(t *testing.T) {
	certServer := httptest.NewTLSServer(nil)
	cert := certServer.TLS.Certificates[0]
	roots := x509.NewCertPool()
	roots.AddCert(certServer.Certificate())
	certServer.Close()
	udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	listener, err := quic.Listen(udp, &utls.Config{Certificates: []utls.Certificate{{Certificate: cert.Certificate, PrivateKey: cert.PrivateKey}}, NextProtos: []string{"h3"}}, &quic.Config{EnableDatagrams: true})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type frame struct {
		typ  uint64
		data []byte
	}
	frames := make(chan frame, 8)
	errors := make(chan error, 4)
	go func() {
		conn, err := listener.Accept(ctx)
		if err != nil {
			errors <- err
			return
		}
		defer conn.CloseWithError(0, "")
		control, err := conn.OpenUniStream()
		if err != nil {
			errors <- err
			return
		}
		if _, err = control.Write([]byte{0, 4, 0}); err != nil {
			errors <- err
			return
		}
		go func() {
			s, err := conn.AcceptUniStream(ctx)
			if err != nil {
				errors <- err
				return
			}
			r := quicvarint.NewReader(s)
			typ, err := quicvarint.Read(r)
			if err != nil || typ != 0 {
				errors <- io.ErrUnexpectedEOF
				return
			}
			for j := 0; j < 4; j++ {
				typ, err := quicvarint.Read(r)
				if err != nil {
					errors <- err
					return
				}
				n, err := quicvarint.Read(r)
				if err != nil || n > 4096 {
					errors <- io.ErrUnexpectedEOF
					return
				}
				data := make([]byte, n)
				if _, err = io.ReadFull(r, data); err != nil {
					errors <- err
					return
				}
				frames <- frame{typ, data}
			}
		}()
		for i := 0; i < 2; i++ {
			s, err := conn.AcceptStream(ctx)
			if err != nil {
				errors <- err
				return
			}
			r := quicvarint.NewReader(s)
			typ, err := quicvarint.Read(r)
			if err != nil || typ != 1 {
				errors <- io.ErrUnexpectedEOF
				return
			}
			n, err := quicvarint.Read(r)
			if err != nil {
				errors <- err
				return
			}
			if _, err = io.CopyN(io.Discard, r, int64(n)); err != nil {
				errors <- err
				return
			}
			if _, err = s.Write([]byte{1, 3, 0, 0, 0xd9, 0, 2, 'o', 'k'}); err != nil {
				errors <- err
				return
			}
			s.Close()
		}
		<-ctx.Done()
	}()
	transport, err := NewTLSClientTransport(profiles.Chrome_152_PSK, tlsclient.WithProtocolRacing(), tlsclient.WithTransportOptions(&tlsclient.TransportOptions{RootCAs: roots}))
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	for _, priority := range []string{"u=1, i", "u=0"} {
		req, _ := http.NewRequestWithContext(ctx, "GET", "https://"+udp.LocalAddr().String()+"/", nil)
		req.Header.Set("Priority", priority)
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || string(data) != "ok" || resp.ProtoMajor != 3 {
			t.Fatalf("response %q %v", data, err)
		}
	}
	read := func() frame {
		t.Helper()
		select {
		case f := <-frames:
			return f
		case err := <-errors:
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		return frame{}
	}
	settings := read()
	if settings.typ != 4 {
		t.Fatalf("first frame %x", settings.typ)
	}
	var ids []uint64
	values := map[uint64]uint64{}
	for b := settings.data; len(b) > 0; {
		id, n, e := quicvarint.Parse(b)
		if e != nil {
			t.Fatal(e)
		}
		v, m, e := quicvarint.Parse(b[n:])
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
		values[id] = v
		b = b[n+m:]
	}
	if len(ids) != 5 || !slices.Equal(ids[:4], []uint64{1, 6, 7, 0x33}) || ids[4]%31 != 2 {
		t.Fatalf("SETTINGS order %v", ids)
	}
	for id, want := range map[uint64]uint64{1: 65536, 6: 262144, 7: 100, 0x33: 1} {
		if values[id] != want {
			t.Fatalf("SETTINGS %x=%d, want %d", id, values[id], want)
		}
	}
	grease := read()
	if grease.typ < 33 || grease.typ%31 != 2 || uint64(len(grease.data)) != (grease.typ-33)/31%4 {
		t.Fatalf("GREASE frame %x length %d", grease.typ, len(grease.data))
	}
	for i, want := range []string{"u=1, i", "u=0"} {
		f := read()
		id, n, e := quicvarint.Parse(f.data)
		if e != nil || f.typ != 0xf0700 || id != uint64(4*i) || string(f.data[n:]) != want {
			t.Fatalf("priority frame %x %x, want stream %d %s", f.typ, f.data, 4*i, want)
		}
	}
}
