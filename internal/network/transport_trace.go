package network

import (
	"context"
	"crypto/tls"
	"net"
	"net/http/httptrace"
	"sync"
	"time"
)

type transportTimingKey struct{}

// TransportTiming records real transport phases. Browser-visible timing is a
// separate projection of this data plus the selected Environment profile.
type TransportTiming struct {
	mu               sync.Mutex
	started          time.Time
	origin           string
	connectionKey    string
	phases           map[string]float64
	reused           bool
	reuseKnown       bool
	connectionID     string
	negotiatedALPN   string
	responseProtocol string
}

type TransportTimingSnapshot struct {
	Origin           string             `json:"origin"`
	ConnectionKey    string             `json:"connectionKey"`
	Phases           map[string]float64 `json:"phases"`
	Reused           bool               `json:"reused"`
	ReuseKnown       bool               `json:"reuseKnown"`
	ConnectionID     string             `json:"connectionId,omitempty"`
	NegotiatedALPN   string             `json:"negotiatedALPN,omitempty"`
	ResponseProtocol string             `json:"responseProtocol,omitempty"`
}

func newTransportTiming(origin, connectionKey string) *TransportTiming {
	return &TransportTiming{started: time.Now(), origin: origin, connectionKey: connectionKey, phases: map[string]float64{}}
}

func withTransportTiming(ctx context.Context, timing *TransportTiming) context.Context {
	return context.WithValue(ctx, transportTimingKey{}, timing)
}

func transportTimingFromContext(ctx context.Context) *TransportTiming {
	timing, _ := ctx.Value(transportTimingKey{}).(*TransportTiming)
	return timing
}

func (t *TransportTiming) mark(name string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	if _, exists := t.phases[name]; !exists {
		t.phases[name] = float64(time.Since(t.started)) / float64(time.Millisecond)
	}
	t.mu.Unlock()
}

func (t *TransportTiming) gotConn(reused bool, connection net.Conn) {
	t.mark("connectionLookupEnd")
	t.mu.Lock()
	t.reused, t.reuseKnown = reused, true
	if connection != nil {
		t.connectionID = connection.LocalAddr().String() + "->" + connection.RemoteAddr().String()
	}
	t.mu.Unlock()
}

func (t *TransportTiming) setALPN(protocol string) {
	t.mu.Lock()
	if t.negotiatedALPN == "" {
		t.negotiatedALPN = protocol
	}
	t.mu.Unlock()
}

func (t *TransportTiming) setResponseProtocol(protocol string) {
	t.mu.Lock()
	t.responseProtocol = protocol
	t.mu.Unlock()
}

func (t *TransportTiming) snapshot() TransportTimingSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	phases := make(map[string]float64, len(t.phases))
	for name, value := range t.phases {
		phases[name] = value
	}
	return TransportTimingSnapshot{Origin: t.origin, ConnectionKey: t.connectionKey, Phases: phases, Reused: t.reused, ReuseKnown: t.reuseKnown, ConnectionID: t.connectionID, NegotiatedALPN: t.negotiatedALPN, ResponseProtocol: t.responseProtocol}
}

func (t TransportTimingSnapshot) shifted(offsetMillis float64) TransportTimingSnapshot {
	shifted := t
	shifted.Phases = make(map[string]float64, len(t.Phases))
	for name, value := range t.Phases {
		shifted.Phases[name] = offsetMillis + value
	}
	return shifted
}

func (t *TransportTiming) standardTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GetConn:           func(string) { t.mark("connectionLookupStart") },
		GotConn:           func(info httptrace.GotConnInfo) { t.gotConn(info.Reused, info.Conn) },
		DNSStart:          func(httptrace.DNSStartInfo) { t.mark("dnsStart") },
		DNSDone:           func(httptrace.DNSDoneInfo) { t.mark("dnsEnd") },
		ConnectStart:      func(string, string) { t.mark("tcpConnectStart") },
		ConnectDone:       func(string, string, error) { t.mark("tcpConnectEnd") },
		TLSHandshakeStart: func() { t.mark("tlsHandshakeStart") },
		TLSHandshakeDone: func(state tls.ConnectionState, _ error) {
			t.mark("tlsHandshakeEnd")
			t.setALPN(state.NegotiatedProtocol)
		},
		WroteHeaders:         func() { t.mark("requestHeadersSent") },
		WroteRequest:         func(httptrace.WroteRequestInfo) { t.mark("requestComplete") },
		GotFirstResponseByte: func() { t.mark("firstResponseByte") },
	}
}
