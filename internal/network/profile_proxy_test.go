package network

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

func TestProfileHTTPAndHTTPSProxy(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(fmt.Sprint(secure), func(t *testing.T) {
			seen := make(chan string, 4)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "CONNECT" {
					t.Error("not tunneled")
					w.WriteHeader(400)
					return
				}
				if r.Header.Get("Proxy-Authorization") != "Basic dXNlcjpwYXNz" {
					w.WriteHeader(407)
					return
				}
				c, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					return
				}
				defer c.Close()
				c.SetDeadline(time.Now().Add(5 * time.Second))
				fmt.Fprint(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
				req, err := http.ReadRequest(bufio.NewReader(c))
				if err != nil {
					return
				}
				seen <- req.Host
				if req.Header.Get("Proxy-Authorization") != "" {
					t.Error("credential forwarded to origin")
				}
				fmt.Fprint(c, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok")
			})
			server := httptest.NewUnstartedServer(handler)
			if secure {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()
			var config *tls.Config
			if secure {
				roots := x509.NewCertPool()
				roots.AddCert(server.Certificate())
				config = &tls.Config{RootCAs: roots}
			}
			raw := strings.Replace(server.URL, "://", "://user:pass@", 1)
			dialer, err := newProfileProxyDialer(raw, config)
			if err != nil {
				t.Fatal(err)
			}
			transport, err := newProxyTLSClientTransport(profiles.Chrome_152_PSK, dialer, tlsclient.WithNotFollowRedirects())
			if err != nil {
				t.Fatal(err)
			}
			defer transport.CloseIdleConnections()
			request, _ := http.NewRequest("GET", "http://not-resolvable.invalid/", nil)
			response, err := transport.RoundTrip(request)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if string(body) != "ok" {
				t.Fatal(string(body))
			}
			select {
			case host := <-seen:
				if host != "not-resolvable.invalid" {
					t.Fatal(host)
				}
			default:
				t.Fatal("no tunneled request")
			}
			if secure {
				untrusted, err := NewProxyTLSClientTransport(profiles.Chrome_152_PSK, raw)
				if err != nil {
					t.Fatal(err)
				}
				defer untrusted.CloseIdleConnections()
				if _, err = untrusted.RoundTrip(request); err == nil {
					t.Fatal("accepted untrusted proxy")
				}
			}
		})
	}
}

func TestProfileSOCKSRemoteDNSAndAuthentication(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	seen := make(chan string, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := listener.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(5 * time.Second))
		r := bufio.NewReader(c)
		read := func(n int) []byte {
			b := make([]byte, n)
			if _, err := io.ReadFull(r, b); err != nil {
				return nil
			}
			return b
		}
		h := read(2)
		if h == nil {
			return
		}
		if read(int(h[1])) == nil {
			return
		}
		c.Write([]byte{5, 2})
		h = read(2)
		if h == nil {
			return
		}
		user := read(int(h[1]))
		h = read(1)
		if h == nil {
			return
		}
		pass := read(int(h[0]))
		if string(user) != "user" || string(pass) != "pass" {
			c.Write([]byte{1, 1})
			return
		}
		c.Write([]byte{1, 0})
		h = read(4)
		if h == nil || h[3] != 3 {
			return
		}
		h = read(1)
		if h == nil {
			return
		}
		host := read(int(h[0]))
		port := read(2)
		if port == nil || binary.BigEndian.Uint16(port) != 80 {
			return
		}
		seen <- string(host)
		c.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 80})
		if _, err := http.ReadRequest(r); err != nil {
			return
		}
		fmt.Fprint(c, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok")
	}()
	transport, err := NewProxyTLSClientTransport(profiles.Chrome_152_PSK, "socks5://user:pass@"+listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	req, _ := http.NewRequest("GET", "http://remote-dns.invalid/", nil)
	response, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	<-done
	select {
	case host := <-seen:
		if host != "remote-dns.invalid" {
			t.Fatal(host)
		}
	default:
		t.Fatal("destination not sent as hostname")
	}
}

func TestProfileProxyFailureDoesNotConnectDirectly(t *testing.T) {
	var mu sync.Mutex
	hits := 0
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); hits++; mu.Unlock() }))
	defer origin.Close()
	denied := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(407) }))
	defer denied.Close()
	transport, err := NewProxyTLSClientTransport(profiles.Chrome_152_PSK, strings.Replace(denied.URL, "://", "://secret-user:secret-password@", 1))
	if err != nil {
		t.Fatal(err)
	}
	defer transport.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", origin.URL, nil)
	_, err = transport.RoundTrip(req)
	if err == nil || strings.Contains(err.Error(), "secret-") || !strings.Contains(err.Error(), "authentication") {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if hits != 0 {
		t.Fatal("direct fallback")
	}
}
