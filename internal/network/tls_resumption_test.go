package network

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// A Chrome-compatible transport must send a real TLS 1.3 PSK binder after a
// ticket is learned. Merely advertising extension 41 in a fingerprint would
// not make crypto/tls report DidResume on the server side.
func TestChrome152TransportPerformsRealTLSResumption(t *testing.T) {
	resumed := make(chan bool, 2)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		resumed <- request.TLS != nil && request.TLS.DidResume
		w.Header().Set("Connection", "close")
		_, _ = io.WriteString(w, "ok")
	}))
	server.EnableHTTP2 = false
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13}
	server.StartTLS()
	defer server.Close()

	transport, err := NewTLSClientTransport(
		profiles.Chrome_152_PSK,
		tls_client.WithInsecureSkipVerify(),
		tls_client.WithForceHttp1(),
	)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		request, requestErr := http.NewRequest(http.MethodGet, server.URL, nil)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		request.Close = true
		response, roundTripErr := transport.RoundTrip(request)
		if roundTripErr != nil {
			t.Fatal(roundTripErr)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}
	if first, second := <-resumed, <-resumed; first || !second {
		t.Fatalf("TLS resumption sequence = [%t %t], want [false true]", first, second)
	}
}
