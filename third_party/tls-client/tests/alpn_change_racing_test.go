package tests

import (
	"crypto/tls"
	"fmt"
	"sync/atomic"
	"testing"

	http "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/fhttp/http2"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/stretchr/testify/assert"
)

// TestClient_ProtocolChangeWithRacingRecovers is the protocol change of
// TestClient_ProtocolChangeBetweenConnections with protocol racing turned on.
//
// Racing takes its own route through RoundTrip and returns before the branch
// that drops a transport the server has outgrown, so the stale transport used
// to survive every retry. The racer clears its own protocol cache when a
// request fails, which looks like recovery but is not: the next request races
// again, reaches for the very same cached transport, and fails on the same
// mismatch. The address stayed broken for the life of the client.
//
// Racing gives the client a free retry that the plain path does not have: the
// failing leg makes the racer fall back to a full race, which rebuilds the
// transport and answers the request. So here even the request that runs into
// the change succeeds, over the protocol the server has moved to.
func TestClient_ProtocolChangeWithRacingRecovers(t *testing.T) {
	var handshakes int64

	cert := selfSignedCert(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
			config := &tls.Config{Certificates: []tls.Certificate{cert}}
			if atomic.AddInt64(&handshakes, 1) == 1 {
				config.NextProtos = []string{"http/1.1"}
			} else {
				config.NextProtos = []string{http2.NextProtoTLS}
			}
			return config, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	go serveChangingProtocol(listener)

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(),
		tls_client.WithClientProfile(profiles.Chrome_133),
		tls_client.WithInsecureSkipVerify(),
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithProtocolRacing(),
	)
	if err != nil {
		t.Fatal(err)
	}

	endpoint := fmt.Sprintf("https://%s/", listener.Addr().String())

	// The HTTP/3 leg has nothing to talk to here, so the HTTP/2 leg wins every
	// race. The first one negotiates http/1.1 and caches an HTTP/1 transport.
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "HTTP/1.1", resp.Proto)

	// The server closed that connection and negotiates h2 from now on. Both
	// requests below have to come back over HTTP/2: the first one because the
	// racer falls back to a race that rebuilds the transport, the second one
	// because the rebuilt transport is what the cache now holds.
	for i := 2; i <= 3; i++ {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			t.Fatal(err)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request %d: %v, want a rebuilt transport speaking the protocol the server moved to", i, err)
		}

		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "HTTP/2.0", resp.Proto)
		resp.Body.Close()
	}
}
