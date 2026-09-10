# Local uTLS integration

Module baseline: `github.com/bogdanfinn/utls v1.7.8-barnius` (see root `go.mod`).
Keep the upstream license. This directory already contained the QUIC session
event integration when the network-profile work began; the root module now
selects this local copy explicitly.

Local QUIC changes expose session events and StoreSession, drain handshake
events before releasing the handshake goroutine, and put the connection's live
transport parameters into a custom ClientHello regardless of preset order.
ClientHello construction failures use the common QUIC handshake cleanup path.

Actual TLS and QUIC cold/resumption regressions live in `internal/network` in the
root module. Profile data belongs to `third_party/tls-client/profiles`, not this
TLS implementation.
