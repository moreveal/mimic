package network

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/moreveal/mimic/internal/trace"
)

func TestCertificateOverrideDoesNotChangeOtherLoadersOrReuseUnverifiedConnections(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "tls-body") }))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()
	u, _ := url.Parse(server.URL)
	for _, pinned := range []bool{false, true} {
		t.Run(map[bool]string{false: "portable", true: "pinned"}[pinned], func(t *testing.T) {
			a := NewLoader(testEnvironment, NewCookieStore(), trace.New())
			b := NewLoader(testEnvironment, NewCookieStore(), trace.New())
			defer a.CloseOwnedTransport()
			defer b.CloseOwnedTransport()
			if pinned {
				transport, err := NewTLSClientTransport(profiles.Chrome_152_PSK, tlsclient.WithDisableHttp3(), tlsclient.WithTimeoutSeconds(5))
				if err != nil {
					t.Fatal(err)
				}
				defer transport.CloseIdleConnections()
				a.SetTransport(transport)
				b.SetTransport(transport)
			}
			if _, err := a.Load(context.Background(), Request{URL: u, Initiator: Navigation}); err == nil {
				t.Fatal("self-signed certificate unexpectedly verified")
			}
			if err := a.SetIgnoreCertificateErrors(true); err != nil {
				t.Fatal(err)
			}
			response, err := a.Load(context.Background(), Request{URL: u, Initiator: Navigation})
			if err != nil || string(response.Body) != "tls-body" {
				t.Fatalf("override: %s, %v", response.Body, err)
			}
			if _, err := b.Load(context.Background(), Request{URL: u, Initiator: Navigation}); err == nil {
				t.Fatal("override leaked into other loader")
			}
			if err := a.SetIgnoreCertificateErrors(false); err != nil {
				t.Fatal(err)
			}
			if _, err := a.Load(context.Background(), Request{URL: u, Initiator: Navigation}); err == nil {
				t.Fatal("disabled override reused unverified connection")
			}
		})
	}
}
