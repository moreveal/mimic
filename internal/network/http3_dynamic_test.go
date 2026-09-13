package network

import (
	"context"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	quic "github.com/bogdanfinn/quic-go-utls"
	"github.com/bogdanfinn/quic-go-utls/quicvarint"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	utls "github.com/bogdanfinn/utls"
)

func TestHTTP3DynamicResponseHeadersAndTrailers(t *testing.T) {
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
	failures := make(chan error, 8)
	feedback := make(chan byte, 16)
	go func() {
		conn, err := listener.Accept(ctx)
		if err != nil {
			failures <- err
			return
		}
		defer conn.CloseWithError(0, "")
		control, err := conn.OpenUniStream()
		if err != nil {
			failures <- err
			return
		}
		if _, err = control.Write([]byte{0, 4, 0}); err != nil {
			failures <- err
			return
		}
		go func() {
			for {
				stream, err := conn.AcceptUniStream(ctx)
				if err != nil {
					return
				}
				go func() {
					reader := quicvarint.NewReader(stream)
					kind, err := quicvarint.Read(reader)
					if err != nil {
						return
					}
					if kind != 3 {
						io.Copy(io.Discard, reader)
						return
					}
					for {
						b, err := reader.ReadByte()
						if err != nil {
							return
						}
						feedback <- b
					}
				}()
			}
		}()
		var encoder *quic.SendStream
		for i := 0; i < 2; i++ {
			stream, err := conn.AcceptStream(ctx)
			if err != nil {
				failures <- err
				return
			}
			// Reading to FIN verifies that no POST body is retried to negotiate QPACK.
			if _, err = io.Copy(io.Discard, stream); err != nil {
				failures <- err
				return
			}
			// Response references absolute entry 0 before its encoder instructions arrive.
			if _, err = stream.Write([]byte{1, 4, 2, 0, 0xd9, 0x80}); err != nil {
				failures <- err
				return
			}
			if i == 0 {
				time.Sleep(20 * time.Millisecond)
				encoder, err = conn.OpenUniStream()
				if err != nil {
					failures <- err
					return
				}
				if _, err = encoder.Write([]byte{2, 0x3f, 0x61, 0x46, 'x', '-', 'd', 'e', 'm', 'o', 2, 'o', 'k'}); err != nil {
					failures <- err
					return
				}
			}
			if _, err = stream.Write([]byte{0, 2, 'o', 'k', 1, 3, 2, 0, 0x80}); err != nil {
				failures <- err
				return
			}
			stream.Close()
		}
		<-ctx.Done()
	}()
	transport, err := NewTLSClientTransport(profiles.Chrome_152_PSK, tlsclient.WithProtocolRacing(), tlsclient.WithTransportOptions(&tlsclient.TransportOptions{RootCAs: roots}))
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequestWithContext(ctx, "POST", "https://"+udp.LocalAddr().String()+"/", strings.NewReader("one request body"))
		response, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || string(body) != "ok" || response.ProtoMajor != 3 || response.Header.Get("X-Demo") != "ok" || response.Trailer.Get("X-Demo") != "ok" {
			t.Fatalf("dynamic response: body=%q headers=%v trailers=%v error=%v", body, response.Header, response.Trailer, err)
		}
	}
	// One insertion, then header and trailer acknowledgments for streams 0 and 4.
	for _, want := range []byte{1, 0x80, 0x80, 0x84, 0x84} {
		select {
		case got := <-feedback:
			if got != want {
				t.Fatalf("feedback %x, want %x", got, want)
			}
		case err := <-failures:
			t.Fatal(err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}
