# Protocol selection must not duplicate application requests

The transport previously raced HTTP/3 and TCP `RoundTrip` calls. TCP started after 300 ms even if HTTP/3 had already submitted the request and was waiting for its response. A slow response could therefore cause two server-side operations, and both attempts could read the same request body.

On the cold path, protocol selection now races connection establishment and submits the application request once. A small local `http3.Transport.Preconnect` extension uses the existing connection pool without opening a request stream. Concurrent initial requests share connection selection; their application responses are not serialized. The losing private HTTP/3 candidate is closed. A previously cached TCP transport can still reconnect during dispatch; selection does not guarantee that every reused TCP transport has a live socket.

The selected request retains its original context, so canceling the connection race does not cancel its response body. Generic errors after dispatch no longer trigger a protocol fallback that might replay an already-processed request. The existing explicit pre-write protocol-change sentinel remains eligible for retry when the request body can be recreated.

Regression coverage includes a real loopback HTTP/3 server with a trusted certificate, a POST response delayed by 450 ms, and a streaming body consumed after connection selection ends. The server must see exactly one request. Additional tests cover rejected certificates, cancellation, losing-handshake cleanup, concurrent dispatch, and errors after body consumption.

Ordinary application errors preserve the selected transport so a canceled request cannot orphan its connection pool. Closing idle connections also drains completed TCP handshakes that have not yet been consumed by an HTTP transport.

Validation: `go test -race .` in `third_party/tls-client`, and `go test -race ./internal/network/... ./chrome/152/...` from the repository root pass.

This change does not alter TLS fingerprints or QUIC transport parameters and does not establish the cause of any historical server rejection. The 300 ms TCP connection delay remains unchanged.

Recovery limitation: an application error on an already selected HTTP/3 transport is returned without TCP fallback. Subsequent requests can reconnect through that transport, but persistent UDP failure does not automatically restart protocol selection. Recovering across protocols requires distinguishing pre-dispatch connection failures from ambiguous failures after a server may have processed the request.
