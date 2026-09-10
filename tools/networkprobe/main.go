// Networkprobe serves a loopback TLS / HTTP/3 origin for browser wire captures.
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	stdtls "crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	quic "github.com/bogdanfinn/quic-go-utls"
	"github.com/bogdanfinn/quic-go-utls/http3"
	"github.com/bogdanfinn/quic-go-utls/quicvarint"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	utls "github.com/bogdanfinn/utls"
)

func main() {
	port := flag.Int("port", 19453, "loopback TCP and UDP port")
	certPath := flag.String("cert", "", "write temporary public certificate as DER")
	rawH3 := flag.Bool("raw-h3", false, "capture control stream bytes with a minimal HTTP/3 peer")
	clientOrigin := flag.String("client-origin", "", "run Mimic's transport against this origin")
	rootFile := flag.String("root", "", "DER certificate trusted by the probe client")
	tcpOnly := flag.Bool("tcp-only", false, "serve closing HTTP/1.1 responses to exercise TCP TLS resumption")
	flag.Parse()
	if *clientOrigin != "" {
		runClient(*clientOrigin, *rootFile)
		return
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(err)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	must(err)
	if *certPath != "" {
		must(os.WriteFile(*certPath, der, 0600))
	}
	spki, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	must(err)
	hash := sha256.Sum256(spki)
	var mu sync.Mutex
	emit := func(v any) { mu.Lock(); defer mu.Unlock(); _ = json.NewEncoder(os.Stdout).Encode(v) }
	address := fmt.Sprintf("127.0.0.1:%d", *port)
	tcp, err := net.Listen("tcp4", address)
	must(err)
	udp, err := net.ListenPacket("udp4", address)
	must(err)
	handler := func(proto, path string, resumed bool) string {
		emit(map[string]any{"protocol": proto, "path": path, "resumed": resumed})
		if path == "/" {
			return `<!doctype html><title>Network probe</title><script>let n=0;setInterval(()=>fetch('/probe?'+n++,{cache:'no-store'}).then(r=>r.text()).then(t=>document.title=t),1000)</script>`
		}
		return proto
	}
	h3 := &http3.Server{TLSConfig: &utls.Config{Certificates: []utls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}, EnableDatagrams: true, Handler: fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, handler(r.Proto, r.URL.Path, r.TLS.DidResume))
	})}
	serve := func(conn net.PacketConn) {
		if *rawH3 {
			serveRawH3(packetCapture{conn, emit}, h3.TLSConfig, emit)
		} else {
			must(h3.Serve(packetCapture{conn, emit}))
		}
	}
	go serve(udp)
	udp6, err := net.ListenPacket("udp6", fmt.Sprintf("[::1]:%d", *port))
	must(err)
	go serve(udp6)
	server := &http.Server{TLSConfig: &stdtls.Config{Certificates: []stdtls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if *tcpOnly {
			w.Header().Set("Connection", "close")
		} else {
			w.Header().Set("Alt-Svc", fmt.Sprintf(`h3=":%d"; ma=60`, *port))
		}
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, handler(r.Proto, r.URL.Path, r.TLS.DidResume))
	})}
	if *tcpOnly {
		server.TLSNextProto = map[string]func(*http.Server, *stdtls.Conn, http.Handler){}
	}
	emit(map[string]any{"url": fmt.Sprintf("https://localhost:%d/", *port), "spki": base64.StdEncoding.EncodeToString(hash[:])})
	must(server.ServeTLS(tcp, "", ""))
}

func runClient(origin, rootFile string) {
	der, err := os.ReadFile(rootFile)
	must(err)
	cert, err := x509.ParseCertificate(der)
	must(err)
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	client, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), tlsclient.WithClientProfile(profiles.Chrome_152_PSK), tlsclient.WithProtocolRacing(), tlsclient.WithTransportOptions(&tlsclient.TransportOptions{RootCAs: roots}))
	must(err)
	defer client.CloseIdleConnections()
	for i := 0; i < 3; i++ {
		if i == 2 {
			client.CloseIdleConnections()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		req, err := fhttp.NewRequestWithContext(ctx, "GET", origin, nil)
		must(err)
		resp, err := client.Do(req)
		must(err)
		_, err = io.Copy(io.Discard, resp.Body)
		must(err)
		resp.Body.Close()
		cancel()
		json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol": resp.Proto, "request": i})
		time.Sleep(100 * time.Millisecond)
	}
}

type packetCapture struct {
	net.PacketConn
	emit func(any)
}

func (p packetCapture) ReadFrom(b []byte) (int, net.Addr, error) {
	n, a, e := p.PacketConn.ReadFrom(b)
	if n > 0 && b[0]&0xf0 == 0xc0 {
		p.emit(map[string]any{"initial": base64.StdEncoding.EncodeToString(b[:n]), "peer": a.String()})
	}
	return n, a, e
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}

func serveRawH3(socket net.PacketConn, config *utls.Config, emit func(any)) {
	config = config.Clone()
	config.NextProtos = []string{"h3"}
	listener, err := quic.Listen(socket, config, &quic.Config{EnableDatagrams: true})
	must(err)
	for {
		conn, err := listener.Accept(context.Background())
		must(err)
		go func() {
			var requests atomic.Int32
			control, err := conn.OpenUniStream()
			if err != nil {
				return
			}
			control.Write([]byte{0, 4, 0})
			go func() {
				for {
					s, err := conn.AcceptUniStream(context.Background())
					if err != nil {
						return
					}
					go func() {
						b := make([]byte, 4096)
						for {
							n, err := s.Read(b)
							if n > 0 {
								emit(map[string]any{"controlStream": s.StreamID(), "peer": conn.RemoteAddr().String(), "bytes": base64.StdEncoding.EncodeToString(b[:n])})
							}
							if err != nil {
								return
							}
						}
					}()
				}
			}()
			for {
				s, err := conn.AcceptStream(context.Background())
				if err != nil {
					return
				}
				go func() {
					r := quicvarint.NewReader(s)
					kind, err := quicvarint.Read(r)
					if err != nil || kind != 1 {
						return
					}
					n, err := quicvarint.Read(r)
					if err != nil {
						return
					}
					if _, err = io.CopyN(io.Discard, r, int64(n)); err != nil {
						return
					}
					emit(map[string]any{"protocol": "HTTP/3.0", "resumed": conn.ConnectionState().TLS.DidResume})
					body := []byte(`<!doctype html><title>H3 probe</title><script>setInterval(()=>fetch('/probe',{cache:'no-store'}),1000)</script>`)
					response := []byte{1, 3, 0, 0, 0xd9, 0}
					response = quicvarint.Append(response, uint64(len(body)))
					response = append(response, body...)
					s.Write(response)
					s.Close()
					if requests.Add(1) == 3 {
						go func() { time.Sleep(200 * time.Millisecond); conn.CloseWithError(0, "probe reconnect") }()
					}
				}()
			}
		}()
	}
}
