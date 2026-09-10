package tls_client

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/http2"
	"github.com/bogdanfinn/tls-client/bandwidth"
	tls "github.com/bogdanfinn/utls"
)

type protocolRacer struct {
	protocolCache        map[string]string
	protocolCacheMu      sync.RWMutex
	connectionSelections map[string]chan struct{}

	clientSessionCache  tls.ClientSessionCache
	insecureSkipVerify  bool
	serverNameOverwrite string
	transportOptions    *TransportOptions
	settings            map[http2.SettingID]uint32
	cachedTransports    map[string]http.RoundTripper
	cachedTransportsLck *sync.Mutex
	certificatePinner   CertificatePinner
	badPinHandlerFunc   BadPinHandlerFunc
	bandwidthTracker    bandwidth.BandwidthTracker

	// dropTransport forgets the transport cached for an address so the next
	// attempt builds one from a fresh handshake. It is the round tripper's own
	// eviction, handed over because a raced request returns from RoundTrip
	// before the branch that would otherwise do it.
	dropTransport func(addr string, stale http.RoundTripper)

	// HTTP/3 specific settings
	http3Settings          map[uint64]uint64
	http3SettingsOrder     []uint64
	http3PriorityParam     uint32
	http3PseudoHeaderOrder []string
	http3SendGreaseFrames  bool

	proxyURL string
}

func newProtocolRacer(
	clientSessionCache tls.ClientSessionCache,
	insecureSkipVerify bool,
	serverNameOverwrite string,
	transportOptions *TransportOptions,
	settings map[http2.SettingID]uint32,
	cachedTransports map[string]http.RoundTripper,
	cachedTransportsLck *sync.Mutex,
	dropTransport func(addr string, stale http.RoundTripper),
	certificatePinner CertificatePinner,
	badPinHandlerFunc BadPinHandlerFunc,
	bandwidthTracker bandwidth.BandwidthTracker,
	http3Settings map[uint64]uint64,
	http3SettingsOrder []uint64,
	http3PriorityParam uint32,
	http3PseudoHeaderOrder []string,
	http3SendGreaseFrames bool,
	proxyURL string,
) *protocolRacer {
	return &protocolRacer{
		protocolCache:          make(map[string]string),
		clientSessionCache:     clientSessionCache,
		insecureSkipVerify:     insecureSkipVerify,
		serverNameOverwrite:    serverNameOverwrite,
		transportOptions:       transportOptions,
		settings:               settings,
		cachedTransports:       cachedTransports,
		cachedTransportsLck:    cachedTransportsLck,
		dropTransport:          dropTransport,
		certificatePinner:      certificatePinner,
		badPinHandlerFunc:      badPinHandlerFunc,
		bandwidthTracker:       bandwidthTracker,
		http3Settings:          http3Settings,
		http3SettingsOrder:     http3SettingsOrder,
		http3PriorityParam:     http3PriorityParam,
		http3PseudoHeaderOrder: http3PseudoHeaderOrder,
		http3SendGreaseFrames:  http3SendGreaseFrames,
		proxyURL:               proxyURL,
	}
}

// race selects a connected protocol before submitting the application request.
// Racing responses would execute a slow request twice, including POST bodies.
func (pr *protocolRacer) race(req *http.Request, addr string, getTransportFunc func(*http.Request, string) error) (*http.Response, error) {
	pr.protocolCacheMu.RLock()
	protocol, found := pr.protocolCache[addr]
	pr.protocolCacheMu.RUnlock()
	if found {
		transport, err := pr.getOrCreateTransport(protocol, addr, req, getTransportFunc)
		if err == nil {
			resp, tripErr := pr.roundTrip(transport, req, addr)
			if tripErr == nil {
				return resp, nil
			}
			// This sentinel is emitted by the TCP dialer before any HTTP write.
			// All other errors may follow a successful server-side operation.
			if !errors.Is(tripErr, errProtocolChanged) {
				return resp, tripErr
			}
			pr.clearProtocolCache(addr)
			if req.Body != nil && req.Body != http.NoBody {
				// A failed RoundTrip may already have closed the body even
				// though this dial failure guarantees it wrote no bytes.
				if req.GetBody == nil {
					return resp, tripErr
				}
				body, err := req.GetBody()
				if err != nil {
					return nil, err
				}
				req = req.Clone(req.Context())
				req.Body = body
			}
		} else {
			pr.handleCachedProtocolError(err, addr, req)
			if errors.Is(err, ErrBadPinDetected) {
				closeRequestBody(req)
				return nil, err
			}
		}
	}
	return pr.startRace(req, addr, getTransportFunc)
}

func (pr *protocolRacer) getOrCreateTransport(protocol, addr string, req *http.Request, getTransportFunc func(*http.Request, string) error) (http.RoundTripper, error) {
	transportKey := pr.getTransportKey(protocol, addr)

	pr.cachedTransportsLck.Lock()
	defer pr.cachedTransportsLck.Unlock()

	if transport, exists := pr.cachedTransports[transportKey]; exists {
		return transport, nil
	}

	transport, err := pr.createTransportForProtocol(protocol, addr, req, getTransportFunc)
	if err != nil {
		return nil, err
	}

	pr.cachedTransports[transportKey] = transport
	return transport, nil
}

func (pr *protocolRacer) createTransportForProtocol(protocol, addr string, req *http.Request, getTransportFunc func(*http.Request, string) error) (http.RoundTripper, error) {
	if protocol == "h3" {
		return buildHTTP3Transport(pr.getHTTP3Config())
	}

	// For HTTP/2, use the standard transport creation
	transportKey := pr.getTransportKey(protocol, addr)
	if err := getTransportFunc(req, transportKey); err != nil {
		return nil, err
	}

	return pr.cachedTransports[transportKey], nil
}

func (pr *protocolRacer) startRace(req *http.Request, addr string, getTransportFunc func(*http.Request, string) error) (*http.Response, error) {
	// Share only connection selection. Application requests remain concurrent.
	for {
		pr.protocolCacheMu.Lock()
		if protocol, ok := pr.protocolCache[addr]; ok {
			pr.protocolCacheMu.Unlock()
			transport, err := pr.getOrCreateTransport(protocol, addr, req, getTransportFunc)
			if err != nil {
				closeRequestBody(req)
				return nil, err
			}
			return pr.selectedRoundTrip(transport, req, addr)
		}
		if pending := pr.connectionSelections[addr]; pending != nil {
			pr.protocolCacheMu.Unlock()
			select {
			case <-pending:
			case <-req.Context().Done():
				closeRequestBody(req)
				return nil, req.Context().Err()
			}
			pr.protocolCacheMu.RLock()
			protocol, ok := pr.protocolCache[addr]
			pr.protocolCacheMu.RUnlock()
			if ok {
				transport, err := pr.getOrCreateTransport(protocol, addr, req, getTransportFunc)
				if err != nil {
					closeRequestBody(req)
					return nil, err
				}
				return pr.selectedRoundTrip(transport, req, addr)
			}
			continue
		}
		if pr.connectionSelections == nil {
			pr.connectionSelections = make(map[string]chan struct{})
		}
		done := make(chan struct{})
		pr.connectionSelections[addr] = done
		pr.protocolCacheMu.Unlock()
		h3, _ := buildHTTP3Transport(pr.getHTTP3Config())
		// An unavailable HTTP/3 route (for example a TCP-only proxy) must
		// still allow the TCP candidate to connect.
		transport, err := pr.selectConnection(req, addr, h3, getTransportFunc)
		pr.protocolCacheMu.Lock()
		delete(pr.connectionSelections, addr)
		close(done)
		pr.protocolCacheMu.Unlock()
		if err != nil {
			return nil, err
		}
		resp, err := pr.roundTrip(transport, req, addr)
		if errors.Is(err, errProtocolChanged) {
			pr.clearProtocolCache(addr)
		}
		return resp, err
	}
}

func (pr *protocolRacer) selectedRoundTrip(transport http.RoundTripper, req *http.Request, addr string) (*http.Response, error) {
	resp, err := pr.roundTrip(transport, req, addr)
	if errors.Is(err, errProtocolChanged) {
		pr.clearProtocolCache(addr)
	}
	return resp, err
}

// The HTTP/3 candidate is private until it wins. The TCP candidate belongs to
// the shared pool: canceled dials stop, but completed idle connections stay in
// that pool and can serve other concurrent requests.
func (pr *protocolRacer) raceConnections(req *http.Request, addr string, h3 http.RoundTripper, getTransportFunc func(*http.Request, string) error) (*http.Response, error) {
	transport, err := pr.selectConnection(req, addr, h3, getTransportFunc)
	if err != nil {
		return nil, err
	}
	return pr.selectedRoundTrip(transport, req, addr)
}

func (pr *protocolRacer) selectConnection(req *http.Request, addr string, h3 http.RoundTripper, getTransportFunc func(*http.Request, string) error) (http.RoundTripper, error) {
	ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
	defer cancel()
	results := make(chan racingResult, 2)
	h3ctx, cancelH3 := context.WithCancel(ctx)
	defer cancelH3()
	h3Done := make(chan struct{})
	go func() {
		defer close(h3Done)
		preconnector, ok := h3.(interface {
			Preconnect(context.Context, string) error
		})
		var err error
		if !ok {
			err = errors.New("HTTP/3 transport cannot preconnect")
		} else {
			err = preconnector.Preconnect(h3ctx, addr)
		}
		results <- racingResult{protocol: "h3", transport: h3, err: err}
	}()
	go func() {
		timer := time.NewTimer(300 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			results <- racingResult{protocol: "h2", err: ctx.Err()}
			return
		}
		transport, err := pr.getOrCreateTransport("h2", addr, req.WithContext(ctx), getTransportFunc)
		results <- racingResult{protocol: "h2", transport: transport, err: err}
	}()
	closeH3 := func() {
		cancelH3()
		// Do not race Close against transport initialization.
		<-h3Done
		if closer, ok := h3.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}
	var lastErr error
	for i := 0; i < 2; i++ {
		select {
		case result := <-results:
			if result.err != nil {
				lastErr = result.err
				continue
			}
			if err := req.Context().Err(); err != nil {
				closeH3()
				closeRequestBody(req)
				return nil, err
			}
			if result.protocol != "h3" {
				closeH3()
			}
			pr.cacheWinningProtocol(addr, result.protocol, result.transport)
			// These contexts govern connection establishment only. The application
			// request and its response body retain the original caller's lifetime.
			cancel()
			return result.transport, nil
		case <-ctx.Done():
			closeH3()
			closeRequestBody(req)
			return nil, ctx.Err()
		}
	}
	closeH3()
	closeRequestBody(req)
	return nil, fmt.Errorf("protocol connection race failed: %w", lastErr)
}

func closeRequestBody(req *http.Request) {
	if req.Body != nil {
		_ = req.Body.Close()
	}
}

// roundTrip sends the request over transport and, when the dial underneath
// reports that the server has moved to a protocol this transport cannot speak,
// forgets the transport so the next attempt builds one that fits.
//
// Clearing the protocol cache alone does not recover from that: the race it
// falls back to reaches for the same cached transport and fails on the same
// mismatch, for every request from then on.
//
// addr is the dial address rather than the transport's cache key, because only
// the transport stored under that key dials through the round tripper. The
// HTTP/3 transport brings its own dialer and never reports this.
func (pr *protocolRacer) roundTrip(transport http.RoundTripper, req *http.Request, addr string) (*http.Response, error) {
	resp, err := transport.RoundTrip(req)
	if err != nil && errors.Is(err, errProtocolChanged) && pr.dropTransport != nil {
		pr.dropTransport(addr, transport)
	}

	return resp, err
}

func (pr *protocolRacer) getTransportKey(protocol, addr string) string {
	if protocol == "h3" {
		return addr + ":h3"
	}
	return addr
}

func (pr *protocolRacer) clearProtocolCache(addr string) {
	pr.protocolCacheMu.Lock()
	delete(pr.protocolCache, addr)
	pr.protocolCacheMu.Unlock()
}

func (pr *protocolRacer) cacheWinningProtocol(addr, protocol string, transport http.RoundTripper) {
	// Publish the connected transport before advertising its protocol; otherwise
	// a concurrent request could build and leak a second HTTP/3 transport.
	if protocol == "h3" {
		if transport == nil {
			return
		}
		pr.cachedTransportsLck.Lock()
		pr.cachedTransports[addr+":h3"] = transport
		pr.cachedTransportsLck.Unlock()
	}
	pr.protocolCacheMu.Lock()
	pr.protocolCache[addr] = protocol
	pr.protocolCacheMu.Unlock()
}

func (pr *protocolRacer) handleCachedProtocolError(err error, addr string, req *http.Request) {
	if errors.Is(err, ErrBadPinDetected) && pr.badPinHandlerFunc != nil {
		pr.badPinHandlerFunc(req)
	}
	pr.clearProtocolCache(addr)
}

func (pr *protocolRacer) getHTTP3Config() *http3Config {
	return &http3Config{
		clientSessionCache:     pr.clientSessionCache,
		insecureSkipVerify:     pr.insecureSkipVerify,
		serverNameOverwrite:    pr.serverNameOverwrite,
		transportOptions:       pr.transportOptions,
		http3Settings:          pr.http3Settings,
		http3SettingsOrder:     pr.http3SettingsOrder,
		http3PriorityParam:     pr.http3PriorityParam,
		http3PseudoHeaderOrder: pr.http3PseudoHeaderOrder,
		http3SendGreaseFrames:  pr.http3SendGreaseFrames,
		proxyURL:               pr.proxyURL,
	}
}

type racingResult struct {
	protocol  string
	transport http.RoundTripper
	err       error
}
