package network

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/monotime"
	"github.com/moreveal/mimic/internal/trace"
)

type elapsedTimingTransport struct{ delay time.Duration }

func (t elapsedTimingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// A short measured operation exposes coarse elapsed-time sources without
	// relying on the platform timer resolution or a network server's latency.
	start := monotime.Now()
	for monotime.Since(start) < t.delay {
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Cache-Control": {"no-store"}}, Body: io.NopCloser(strings.NewReader("body")), Request: r}, nil
}

func TestTransportTimingPreservesShortMeasuredIntervals(t *testing.T) {
	const delay = 150 * time.Microsecond
	l := NewLoader(testEnvironment, NewCookieStore(), trace.New())
	l.SetTransport(elapsedTimingTransport{delay: delay})
	u, _ := url.Parse("https://example.test/short")
	for i := 0; i < 32; i++ {
		res, err := l.Load(context.Background(), Request{URL: u, Initiator: Navigation})
		if err != nil {
			t.Fatal(err)
		}
		end := res.TransportTiming.Phases["responseComplete"]
		visible := res.BrowserVisibleTiming.Phases["responseComplete"]
		if res.Duration < delay || end < float64(delay)/float64(time.Millisecond) || visible < end {
			t.Fatalf("elapsed interval lost: duration=%v transport=%g visible=%g", res.Duration, end, visible)
		}
	}
}
