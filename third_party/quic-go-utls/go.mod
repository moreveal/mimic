module github.com/bogdanfinn/quic-go-utls

go 1.24.1

require (
	github.com/bogdanfinn/fhttp v0.6.9
	github.com/bogdanfinn/utls v1.7.8-barnius
	github.com/quic-go/qpack v0.6.0
	github.com/stretchr/testify v1.11.1
	go.uber.org/mock v0.5.2
	golang.org/x/crypto v0.46.0
	golang.org/x/net v0.48.0
	golang.org/x/sync v0.19.0
	golang.org/x/sys v0.39.0
)

require (
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/cloudflare/circl v1.6.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/jordanlewis/gcassert v0.0.0-20250430164644-389ef753e22e // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	golang.org/x/mod v0.30.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	golang.org/x/tools v0.39.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

tool (
	github.com/jordanlewis/gcassert/cmd/gcassert
	go.uber.org/mock/mockgen
)

//replace github.com/bogdanfinn/fhttp => ../fhttp

//replace github.com/bogdanfinn/utls => ../utls
