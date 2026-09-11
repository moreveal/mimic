package network

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"

	tlsclient "github.com/bogdanfinn/tls-client"
)

type certificatePolicyTransport interface {
	RoundTripIgnoringCertificateErrors(*http.Request) (*http.Response, error)
}

func (l *Loader) SupportsCertificateOverride() bool {
	_, ok := l.transport.(certificatePolicyTransport)
	return ok
}

// The override belongs to the Loader/Page. A shared transport uses separate
// verified and unverified connection pools, so one Page cannot change another
// Page's verification policy or reuse its unverified TLS connection.
func (l *Loader) SetIgnoreCertificateErrors(ignore bool) error {
	if ignore {
		if _, ok := l.transport.(certificatePolicyTransport); !ok {
			return fmt.Errorf("certificate override is unsupported by this transport")
		}
	}
	l.ignoreCertificateErrors.Store(ignore)
	return nil
}

func (l *Loader) roundTrip(request *http.Request) (*http.Response, error) {
	if l.ignoreCertificateErrors.Load() {
		return l.transport.(certificatePolicyTransport).RoundTripIgnoringCertificateErrors(request)
	}
	return l.transport.RoundTrip(request)
}

func (l *Loader) CloseOwnedTransport() {
	if l.ownsTransport {
		if closer, ok := l.transport.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
	}
}

func (t *TLSClientTransport) RoundTripIgnoringCertificateErrors(request *http.Request) (*http.Response, error) {
	t.insecureMu.Lock()
	if t.insecureClient == nil {
		options := append([]tlsclient.HttpClientOption(nil), t.clientOptions...)
		options = append(options, tlsclient.WithInsecureSkipVerify())
		client, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), options...)
		if err != nil {
			t.insecureMu.Unlock()
			return nil, err
		}
		t.insecureClient = client
	}
	client := t.insecureClient
	t.insecureMu.Unlock()
	return t.roundTrip(request, client)
}

// The default portable transport has the same policy separation as the pinned
// TLS transport. Clones preserve redirect/timeouts and are allocated lazily.
type httpCertificateTransport struct {
	HTTPTransport
	mu       sync.Mutex
	insecure *http.Client
}

func (t *httpCertificateTransport) RoundTripIgnoringCertificateErrors(request *http.Request) (*http.Response, error) {
	t.mu.Lock()
	if t.insecure == nil {
		base := t.Client.Transport.(*http.Transport).Clone()
		if base.TLSClientConfig == nil {
			base.TLSClientConfig = &tls.Config{}
		}
		base.TLSClientConfig.InsecureSkipVerify = true // Explicit CDP override.
		client := *t.Client
		client.Transport = base
		t.insecure = &client
	}
	client := t.insecure
	t.mu.Unlock()
	return client.Do(request)
}

func (t *httpCertificateTransport) CloseIdleConnections() {
	t.Client.CloseIdleConnections()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.insecure != nil {
		t.insecure.CloseIdleConnections()
	}
}
