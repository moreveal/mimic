# Local dependency provenance

Source: `github.com/bogdanfinn/quic-go-utls v1.0.10-utls`, copied from the Go module cache. The upstream LICENSE is retained. Upstream tests, test data, examples, and GitHub automation are omitted from this source copy.

The sole local runtime extension is `http3.Transport.Preconnect`: establish a pooled connection and wait for its full handshake without sending an application request. This lets the caller race connection establishment without sending the same request twice. TLS ClientHello construction and QUIC transport settings remain upstream behavior.

Local regression tests cover this extension; this directory does not contain the upstream test suite.
