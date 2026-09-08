package network

import "testing"

func TestTransportTraceSeparatesHandshakeALPNFromResponseProtocol(t *testing.T) {
	timing := newTransportTiming("https://example.test", "https://example.test")
	timing.setALPN("h3")
	timing.setResponseProtocol("HTTP/3.0")

	snapshot := timing.snapshot()
	if snapshot.NegotiatedALPN != "h3" {
		t.Fatalf("negotiated ALPN = %q, want h3", snapshot.NegotiatedALPN)
	}
	if snapshot.ResponseProtocol != "HTTP/3.0" {
		t.Fatalf("response protocol = %q, want HTTP/3.0", snapshot.ResponseProtocol)
	}
}

func TestReusedRequestDoesNotInventHandshakeALPN(t *testing.T) {
	timing := newTransportTiming("https://example.test", "https://example.test")
	timing.setResponseProtocol("HTTP/3.0")

	snapshot := timing.snapshot()
	if snapshot.NegotiatedALPN != "" {
		t.Fatalf("reused request invented ALPN %q without a handshake observation", snapshot.NegotiatedALPN)
	}
	if snapshot.ResponseProtocol != "HTTP/3.0" {
		t.Fatalf("response protocol = %q, want HTTP/3.0", snapshot.ResponseProtocol)
	}
}
