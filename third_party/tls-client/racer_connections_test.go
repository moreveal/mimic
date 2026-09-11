package tls_client

import (
	"context"
	"crypto/x509"
	"errors"
	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/quic-go-utls/http3"
	tls "github.com/bogdanfinn/utls"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type connectionCandidate struct {
	connect func(context.Context) error
	trip    func(*http.Request) (*http.Response, error)
	calls   atomic.Int32
	closed  atomic.Bool
}

func (c *connectionCandidate) Preconnect(ctx context.Context, _ string) error { return c.connect(ctx) }
func (c *connectionCandidate) RoundTrip(r *http.Request) (*http.Response, error) {
	c.calls.Add(1)
	return c.trip(r)
}
func (c *connectionCandidate) Close() error { c.closed.Store(true); return nil }
func testRacer() *protocolRacer {
	return &protocolRacer{protocolCache: map[string]string{}, cachedTransports: map[string]http.RoundTripper{}, cachedTransportsLck: &sync.Mutex{}}
}
func TestConnectionRaceSlowResponseSendsPOSTOnce(t *testing.T) {
	r := testRacer()
	var tcpDials atomic.Int32
	h3 := &connectionCandidate{connect: func(context.Context) error { return nil }, trip: func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil || string(body) != "mutation" {
			t.Errorf("body %q, %v", body, err)
		}
		time.Sleep(450 * time.Millisecond)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("response"))}, nil
	}}
	req, _ := http.NewRequest("POST", "https://example.test/", strings.NewReader("mutation"))
	resp, err := r.raceConnections(req, "example.test:443", h3, func(*http.Request, string) error { tcpDials.Add(1); return errors.New("unexpected TCP dial") })
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil || string(b) != "response" || req.Context().Err() != nil {
		t.Fatalf("response lifetime: %q %v", b, err)
	}
	if h3.calls.Load() != 1 || tcpDials.Load() != 0 || h3.closed.Load() {
		t.Fatalf("calls=%d TCP=%d closed=%v", h3.calls.Load(), tcpDials.Load(), h3.closed.Load())
	}
}
func TestConnectionRaceCancelsLosingHandshake(t *testing.T) {
	r := testRacer()
	stopped := make(chan struct{})
	h3 := &connectionCandidate{connect: func(ctx context.Context) error { defer close(stopped); <-ctx.Done(); return ctx.Err() }}
	tcp := &connectionCandidate{trip: func(req *http.Request) (*http.Response, error) {
		return &http.Response{Body: io.NopCloser(strings.NewReader("ok"))}, nil
	}}
	req, _ := http.NewRequest("GET", "https://example.test/", nil)
	resp, err := r.raceConnections(req, "example.test:443", h3, func(_ *http.Request, addr string) error { r.cachedTransports[addr] = tcp; return nil })
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("loser not canceled")
	}
	if h3.calls.Load() != 0 || !h3.closed.Load() || tcp.calls.Load() != 1 {
		t.Fatal("loser request or transport leak")
	}
}

type trackedBody struct{ reads, closes atomic.Int32 }

func (b *trackedBody) Read([]byte) (int, error) { b.reads.Add(1); return 0, io.EOF }
func (b *trackedBody) Close() error             { b.closes.Add(1); return nil }
func TestConnectionRaceCallerCancellationBeforeHandshake(t *testing.T) {
	r := testRacer()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	body := &trackedBody{}
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://example.test/", body)
	h3 := &connectionCandidate{connect: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }}
	_, err := r.raceConnections(req, "example.test:443", h3, func(*http.Request, string) error { t.Error("TCP dial after cancellation"); return nil })
	if !errors.Is(err, context.Canceled) || body.reads.Load() != 0 || body.closes.Load() != 1 || !h3.closed.Load() {
		t.Fatalf("err=%v reads=%d closes=%d closed=%v", err, body.reads.Load(), body.closes.Load(), h3.closed.Load())
	}
}
func TestCachedRequestErrorDoesNotReplayConsumedBody(t *testing.T) {
	r := testRacer()
	want := errors.New("server disconnected after processing")
	c := &connectionCandidate{trip: func(req *http.Request) (*http.Response, error) {
		io.Copy(io.Discard, req.Body)
		req.Body.Close()
		return nil, want
	}}
	r.protocolCache["example.test:443"] = "h3"
	r.cachedTransports["example.test:443:h3"] = c
	req, _ := http.NewRequest("POST", "https://example.test/", strings.NewReader("mutation"))
	_, err := r.race(req, "example.test:443", func(*http.Request, string) error { t.Error("replayed request"); return nil })
	if !errors.Is(err, want) || c.calls.Load() != 1 {
		t.Fatalf("err=%v calls=%d", err, c.calls.Load())
	}
}

func TestCachedHTTP3FailureRetriesSafeRequestOverTCP(t *testing.T) {
	r := testRacer()
	h3Err := errors.New("http3: invalid response: qpack decode failed")
	h3 := &connectionCandidate{connect: func(context.Context) error { return nil }, trip: func(*http.Request) (*http.Response, error) {
		return nil, h3Err
	}}
	tcp := &connectionCandidate{connect: func(context.Context) error { return nil }, trip: func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("fallback"))}, nil
	}}
	r.protocolCache["example.test:443"] = "h3"
	r.cachedTransports["example.test:443:h3"] = h3
	req, _ := http.NewRequest("GET", "https://example.test/", nil)
	resp, err := r.race(req, "example.test:443", func(_ *http.Request, addr string) error {
		r.cachedTransports[addr] = tcp
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "fallback" || h3.calls.Load() != 1 || tcp.calls.Load() != 1 || !h3.closed.Load() {
		t.Fatalf("body=%q h3=%d tcp=%d h3Closed=%v", body, h3.calls.Load(), tcp.calls.Load(), h3.closed.Load())
	}
	if got := r.protocolCache["example.test:443"]; got != "h2" {
		t.Fatalf("cached protocol = %q, want h2", got)
	}
}

func TestConnectionRaceRealQUICSlowPOSTAndStreamingBody(t *testing.T) {
	certServer := httptest.NewTLSServer(nil)
	cert := certServer.TLS.Certificates[0]
	leaf := certServer.Certificate()
	certServer.Close()
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	release := make(chan struct{})
	server := &http3.Server{TLSConfig: &tls.Config{Certificates: []tls.Certificate{{Certificate: cert.Certificate, PrivateKey: cert.PrivateKey}}}, Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		b, err := io.ReadAll(req.Body)
		if err != nil || string(b) != "mutation" {
			t.Errorf("server body %q %v", b, err)
		}
		time.Sleep(450 * time.Millisecond)
		w.Write([]byte("first"))
		w.(http.Flusher).Flush()
		<-release
		w.Write([]byte("last"))
	})}
	go server.Serve(udp)
	defer server.Close()
	defer udp.Close()
	transport := &http3.Transport{TLSClientConfig: &tls.Config{RootCAs: roots}}
	defer transport.Close()
	r := testRacer()
	var tcpDials atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://"+udp.LocalAddr().String()+"/", strings.NewReader("mutation"))
	resp, err := r.raceConnections(req, udp.LocalAddr().String(), transport, func(*http.Request, string) error { tcpDials.Add(1); return errors.New("unexpected TCP") })
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil || string(b) != "firstlast" {
		t.Fatalf("body after selection context canceled %q %v", b, err)
	}
	if calls.Load() != 1 || tcpDials.Load() != 0 {
		t.Fatalf("requests %d TCP %d", calls.Load(), tcpDials.Load())
	}
}

func TestConnectionSelectionWaitersDispatchConcurrently(t *testing.T) {
	r := testRacer()
	pending := make(chan struct{})
	r.connectionSelections = map[string]chan struct{}{"example.test:443": pending}
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	done := make(chan error, 8)
	transport := &connectionCandidate{trip: func(req *http.Request) (*http.Response, error) {
		entered <- struct{}{}
		<-release
		return &http.Response{Body: io.NopCloser(strings.NewReader("ok"))}, nil
	}}
	for i := 0; i < 8; i++ {
		go func() {
			req, _ := http.NewRequest("GET", "https://example.test/", nil)
			resp, err := r.startRace(req, "example.test:443", func(*http.Request, string) error { return errors.New("unexpected fresh dial") })
			if resp != nil {
				resp.Body.Close()
			}
			done <- err
		}()
	}
	r.cacheWinningProtocol("example.test:443", "h3", transport)
	r.protocolCacheMu.Lock()
	delete(r.connectionSelections, "example.test:443")
	close(pending)
	r.protocolCacheMu.Unlock()
	for i := 0; i < 8; i++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("application requests serialized with connection selection")
		}
	}
	close(release)
	for i := 0; i < 8; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if transport.calls.Load() != 8 {
		t.Fatal("wrong number of requests")
	}
}

func TestRealQUICPreconnectRejectsCertificateWithoutRequest(t *testing.T) {
	certServer := httptest.NewTLSServer(nil)
	cert := certServer.TLS.Certificates[0]
	certServer.Close()
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := &http3.Server{TLSConfig: &tls.Config{Certificates: []tls.Certificate{{Certificate: cert.Certificate, PrivateKey: cert.PrivateKey}}}, Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) })}
	go server.Serve(udp)
	defer server.Close()
	defer udp.Close()
	transport := &http3.Transport{TLSClientConfig: &tls.Config{RootCAs: x509.NewCertPool()}}
	defer transport.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = transport.Preconnect(ctx, udp.LocalAddr().String())
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected certificate rejection, got %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("application request before certificate validation")
	}
}
func TestConnectionRaceRealQUICHandshakeCancellation(t *testing.T) {
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	transport := &http3.Transport{}
	defer transport.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	body := &trackedBody{}
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://"+udp.LocalAddr().String()+"/", body)
	_, err = testRacer().raceConnections(req, udp.LocalAddr().String(), transport, func(*http.Request, string) error {
		t.Error("TCP started after canceled handshake")
		return errors.New("unexpected")
	})
	if !errors.Is(err, context.DeadlineExceeded) || body.reads.Load() != 0 || body.closes.Load() != 1 {
		t.Fatalf("err %v body reads %d closes %d", err, body.reads.Load(), body.closes.Load())
	}
}

func TestCanceledApplicationRequestPreservesSelectedTransport(t *testing.T) {
	r := testRacer()
	c := &connectionCandidate{trip: func(req *http.Request) (*http.Response, error) {
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		return &http.Response{Body: io.NopCloser(strings.NewReader("ok"))}, nil
	}}
	r.cacheWinningProtocol("example.test:443", "h3", c)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://example.test/", nil)
	_, err := r.race(req, "example.test:443", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	req, _ = http.NewRequest("GET", "https://example.test/", nil)
	resp, err := r.race(req, "example.test:443", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if c.calls.Load() != 2 || c.closed.Load() {
		t.Fatal("canceled request discarded the connection owner")
	}
}
func TestCloseIdleConnectionsClosesPendingHandshake(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	rt := &roundTripper{cachedConnections: map[string]net.Conn{"example.test:443": client}, cachedTransports: map[string]http.RoundTripper{}}
	rt.CloseIdleConnections()
	server.SetReadDeadline(time.Now().Add(time.Second))
	_, err := server.Read(make([]byte, 1))
	if !errors.Is(err, io.EOF) {
		t.Fatalf("pending connection not closed: %v", err)
	}
	if len(rt.cachedConnections) != 0 {
		t.Fatal("pending handshake retained")
	}
	rt.CloseIdleConnections()
}
