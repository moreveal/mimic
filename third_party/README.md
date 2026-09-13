# Local dependencies

`tls-client/` is the retained source of `github.com/bogdanfinn/tls-client v1.16.0`,
selected by the root go.mod replace directive. Preserve its LICENSE and upstream
attribution. It is a nested Go module; root `go test ./...` does not test it.

The established local racer change keeps the winning HTTP/3 transport in the session
pool, preserving its actual QUIC connection instead of manufacturing a new one
for the next same-origin resource. `racer_reuse_test.go` tests that identity.
Do not delete it as an optimization experiment. Upstream tests/examples include
external services and are not part of Mimic's offline baseline.

```powershell
cd third_party/tls-client
go test . -run '^TestWinningHTTP3TransportIsTheCachedTransport$' -count=1
go test -race . -run '^TestWinningHTTP3TransportIsTheCachedTransport$' -count=1
```

No upstream repository was contacted during stabilization. The local source was
compared against the already-installed module-cache copy where available.

The other retained delta is `profiles/internal_browser_profiles.go`: Chrome 152
PSK/non-PSK profiles use extension shuffling, observed server_padding and HTTP/3
settings/order/priority/grease. Root `internal/network` tests cover these profiles
and real TLS resumption. The offline comparison found exactly these two modified
upstream files plus the added racer regression; no upstream files were removed.

## V8 binding

`gov8/` is Mimic's local fork of `github.com/maclof/gov8 v0.1.1`, selected by
`go.mod`. It includes native HTMLDDA behavior and packaged V8 libraries for
Windows amd64 and Linux amd64. Keep both compressed assets and their checksum
metadata in source distributions; only the host asset is embedded in a build.
Linux calls use the pinned `github.com/ebitengine/purego v0.10.0` module.

See [native build provenance and requirements](gov8/README.mimic.md). The nested
module's tests must also run explicitly: `go test ./...` from `third_party/gov8`.
