package tls_client

import (
	"sync"
	"testing"

	http "github.com/bogdanfinn/fhttp"
)

type identityRoundTripper struct{}

func (*identityRoundTripper) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }

func TestWinningHTTP3TransportIsTheCachedTransport(t *testing.T) {
	cache := make(map[string]http.RoundTripper)
	racer := &protocolRacer{
		protocolCache:       make(map[string]string),
		cachedTransports:    cache,
		cachedTransportsLck: &sync.Mutex{},
	}
	winner := &identityRoundTripper{}
	racer.cacheWinningProtocol("example.test:443", "h3", winner)

	cached, err := racer.getOrCreateTransport("h3", "example.test:443", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cached != winner {
		t.Fatal("the first same-origin request after the race would use a new QUIC connection")
	}
}
