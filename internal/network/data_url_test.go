package network

import (
	"context"
	"github.com/moreveal/mimic/internal/trace"
	"net/url"
	"testing"
)

func TestDataResourcesNeverUseTransport(t *testing.T) {
	loader := NewLoaderWithSession(testEnvironment, NewCookieStore(), NewSessionState(), trace.New())
	transport := &countingTransport{}
	loader.SetTransport(transport)
	for _, raw := range []string{"data:,hello+world", "data:;base64,aGVsbG8rd29ybGQ="} {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		response, err := loader.Load(context.Background(), Request{URL: u, Initiator: Fetch})
		if err != nil || string(response.Body) != "hello+world" || response.Status != 200 || response.Headers.Get("Content-Type") != "text/plain;charset=US-ASCII" {
			t.Fatalf("%s: %+v %v", raw, response, err)
		}
	}
	if transport.calls != 0 {
		t.Fatalf("transport called %d times", transport.calls)
	}
}
