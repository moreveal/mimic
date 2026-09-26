package network

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/state"
	"github.com/moreveal/mimic/internal/trace"
)

type sessionFulfillment struct {
	requests []Request
}

func (s *sessionFulfillment) Before(_ context.Context, request Request) (Decision, error) {
	s.requests = append(s.requests, request)
	header := http.Header{"Content-Type": {"text/plain"}}
	status := http.StatusOK
	if request.URL.Path == "/start" {
		time.Sleep(10 * time.Millisecond)
		header["Set-Cookie"] = []string{"visible=1; Path=/; SameSite=Lax", "hidden=2; Path=/; HttpOnly; SameSite=Lax"}
		header["Accept-Ch"] = []string{"Sec-CH-Prefers-Color-Scheme", "Sec-CH-UA-Bitness"}
	} else if request.URL.Path == "/redirect" {
		status = http.StatusFound
		header.Set("Location", "/echo")
		header.Set("Set-Cookie", "redirected=3; Path=/")
	}
	return Decision{Response: &Response{Status: status, Headers: header, Body: []byte("fixture"), URL: request.URL}}, nil
}

func (*sessionFulfillment) After(_ context.Context, _ Request, response Response) (Response, error) {
	return response, nil
}

func TestFulfilledResponseUpdatesCanonicalSession(t *testing.T) {
	for _, credentials := range []string{"include", "omit"} {
		t.Run(credentials, func(t *testing.T) {
			environment := testEnvironment()
			environment.Platform.Architecture = "x86_64"
			loader := NewLoader(func() state.Environment { return environment }, NewCookieStore(), trace.New())
			interceptor := &sessionFulfillment{}
			loader.Use(interceptor)
			load := func(path string) Response {
				t.Helper()
				target, _ := url.Parse("https://fixture.example.test" + path)
				response, err := loader.Load(context.Background(), Request{URL: target, Initiator: Navigation, Credentials: credentials})
				if err != nil {
					t.Fatal(err)
				}
				return response
			}
			response := load("/start")
			phases := response.BrowserVisibleTiming.Phases
			if phases["firstResponseByte"] < 10 || phases["responseComplete"] < phases["firstResponseByte"] {
				t.Errorf("fulfillment wait absent from browser timing: %v", phases)
			}
			if len(response.TransportTiming.Phases) != 0 {
				t.Error("fulfillment fabricated a transport observation")
			}
			load("/echo")
			header := interceptor.requests[1].Headers
			if got := header.Get("Cookie"); credentials == "include" && got != "visible=1; hidden=2" || credentials == "omit" && got != "" {
				t.Errorf("next-request cookies = %q", got)
			}
			if got := header.Get("Sec-CH-UA-Bitness"); got != `"64"` {
				t.Errorf("next-request bitness = %q", got)
			}
			load("/redirect")
			got := interceptor.requests[len(interceptor.requests)-1].Headers.Get("Cookie")
			if strings.Contains(got, "redirected=3") != (credentials == "include") {
				t.Errorf("redirect response cookie = %q", got)
			}
		})
	}
}
