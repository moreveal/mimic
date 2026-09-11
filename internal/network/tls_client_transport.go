package network

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math/bits"
	"net/http"
	"slices"
	"strings"
	"sync"
	"unicode/utf16"

	fhttp "github.com/bogdanfinn/fhttp"
	fhttptrace "github.com/bogdanfinn/fhttp/httptrace"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	utls "github.com/bogdanfinn/utls"
)

// TLSClientTransport adapts a pinned browser wire profile to the neutral
// net/http boundary consumed by ResourceLoader. Cookie, redirect, cache and
// interception semantics deliberately remain above this transport.
type TLSClientTransport struct {
	client         tls_client.HttpClient
	fallback       http.RoundTripper
	insecureMu     sync.Mutex
	insecureClient tls_client.HttpClient
	clientOptions  []tls_client.HttpClientOption
}

func blinkHeaderHash(name string) uint32 {
	units := utf16.Encode([]rune(strings.ToLower(name)))
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		data[i*2] = byte(unit)
		data[i*2+1] = byte(unit >> 8)
	}
	// This is RAPID_SEED from the rapidhash revision bundled by the pinned
	// Chromium snapshot. Blink's AtomicString hash is the low 24 bits.
	value := uint32(chromiumRapidHash(data)) & 0x00ffffff
	if value == 0 {
		return 0x00800000
	}
	return value
}

// chromiumRapidHash is the compact three-secret rapidhash variant shipped by
// Chromium r1669021. Upstream rapidhash v3 has since changed its short-input
// and finalization paths, so using a contemporary package would not reproduce
// Blink's deterministic WTF string hashes.
func chromiumRapidHash(data []byte) uint64 {
	const seed0 = uint64(0xbdd89aa982704029)
	secret := [3]uint64{0x2d358dccaa6c78a5, 0x8bb84b93962eacc9, 0x4b33a62ed433d4a3}
	seed := seed0 ^ rapidMix(seed0^secret[0], secret[1]) ^ uint64(len(data))
	read32 := func(p []byte) uint64 { return uint64(binary.LittleEndian.Uint32(p)) }
	read64 := func(p []byte) uint64 { return binary.LittleEndian.Uint64(p) }
	var a, b uint64
	if len(data) <= 16 {
		if len(data) >= 4 {
			last := len(data) - 4
			a = read32(data)<<32 | read32(data[last:])
			delta := (len(data) & 24) >> (len(data) >> 3)
			b = read32(data[delta:])<<32 | read32(data[last-delta:])
		} else if len(data) > 0 {
			a = uint64(data[0])<<56 | uint64(data[len(data)/2])<<32 | uint64(data[len(data)-1])
		}
	} else {
		p, remaining := 0, len(data)
		if remaining > 48 {
			see1, see2 := seed, seed
			for {
				seed = rapidMix(read64(data[p:])^secret[0], read64(data[p+8:])^seed)
				see1 = rapidMix(read64(data[p+16:])^secret[1], read64(data[p+24:])^see1)
				see2 = rapidMix(read64(data[p+32:])^secret[2], read64(data[p+40:])^see2)
				p += 48
				remaining -= 48
				if remaining < 48 {
					break
				}
			}
			seed ^= see1 ^ see2
		}
		if remaining > 16 {
			seed = rapidMix(read64(data[p:])^secret[2], read64(data[p+8:])^seed^secret[1])
			if remaining > 32 {
				seed = rapidMix(read64(data[p+16:])^secret[2], read64(data[p+24:])^seed)
			}
		}
		a = read64(data[p+remaining-16:])
		b = read64(data[p+remaining-8:])
	}
	a ^= secret[1]
	b ^= seed
	hi, lo := bits.Mul64(a, b)
	return rapidMix(lo^secret[0]^uint64(len(data)), hi^secret[1])
}

func rapidMix(a, b uint64) uint64 {
	hi, lo := bits.Mul64(a, b)
	return hi ^ lo
}

func NewTLSClientTransport(profile profiles.ClientProfile, options ...tls_client.HttpClientOption) (*TLSClientTransport, error) {
	clientOptions := []tls_client.HttpClientOption{tls_client.WithClientProfile(profile)}
	clientOptions = append(clientOptions, options...)
	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), clientOptions...)
	if err != nil {
		return nil, err
	}
	return &TLSClientTransport{client: client, fallback: http.DefaultTransport.(*http.Transport).Clone(), clientOptions: clientOptions}, nil
}

func (t *TLSClientTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return t.roundTrip(request, t.client)
}
func (t *TLSClientTransport) roundTrip(request *http.Request, client tls_client.HttpClient) (*http.Response, error) {
	// Browser TLS behavior is irrelevant for clear-text local development and
	// retaining net/http here makes ordinary httptest servers deterministic.
	if request.URL.Scheme != "https" {
		return t.fallback.RoundTrip(request)
	}
	body, err := requestBytes(request)
	if err != nil {
		return nil, err
	}
	nativeRequest, err := fhttp.NewRequestWithContext(request.Context(), request.Method, request.URL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	nativeRequest.Host = request.Host
	nativeRequest.Header = make(fhttp.Header, len(request.Header)+1)
	for name, values := range request.Header {
		nativeRequest.Header[name] = append([]string(nil), values...)
	}
	initiator, authorOrder := browserHeaderLayout(request.Context())
	orderHeaders := request.Header.Clone()
	if len(body) > 0 && orderHeaders.Get("Content-Length") == "" {
		orderHeaders.Set("Content-Length", "0") // order marker; fhttp writes the actual length
	}
	nativeRequest.Header[fhttp.HeaderOrderKey] = chromeHeaderOrder(orderHeaders, authorOrder, initiator)
	if timing := transportTimingFromContext(request.Context()); timing != nil {
		trace := &fhttptrace.ClientTrace{
			GetConn:           func(string) { timing.mark("connectionLookupStart") },
			GotConn:           func(info fhttptrace.GotConnInfo) { timing.gotConn(info.Reused, info.Conn) },
			DNSStart:          func(fhttptrace.DNSStartInfo) { timing.mark("dnsStart") },
			DNSDone:           func(fhttptrace.DNSDoneInfo) { timing.mark("dnsEnd") },
			ConnectStart:      func(string, string) { timing.mark("tcpConnectStart") },
			ConnectDone:       func(string, string, error) { timing.mark("tcpConnectEnd") },
			TLSHandshakeStart: func() { timing.mark("tlsHandshakeStart") },
			TLSHandshakeDone: func(state utls.ConnectionState, _ error) {
				timing.mark("tlsHandshakeEnd")
				timing.setALPN(state.NegotiatedProtocol)
			},
			WroteHeaders:         func() { timing.mark("requestHeadersSent") },
			WroteRequest:         func(fhttptrace.WroteRequestInfo) { timing.mark("requestComplete") },
			GotFirstResponseByte: func() { timing.mark("firstResponseByte") },
		}
		nativeRequest = nativeRequest.WithContext(fhttptrace.WithClientTrace(nativeRequest.Context(), trace))
	}

	nativeResponse, err := client.Do(nativeRequest)
	if err != nil {
		return nil, err
	}
	if timing := transportTimingFromContext(request.Context()); timing != nil {
		// This is the protocol actually used by the winning transport. Keep it
		// separate from TLS ALPN, which only exists when the handshake callback
		// was observed for this request.
		timing.setResponseProtocol(nativeResponse.Proto)
	}
	responseHeader := make(http.Header, len(nativeResponse.Header))
	for name, values := range nativeResponse.Header {
		if name == fhttp.HeaderOrderKey {
			continue
		}
		responseHeader[name] = append([]string(nil), values...)
	}
	return &http.Response{
		Status:           nativeResponse.Status,
		StatusCode:       nativeResponse.StatusCode,
		Proto:            nativeResponse.Proto,
		ProtoMajor:       nativeResponse.ProtoMajor,
		ProtoMinor:       nativeResponse.ProtoMinor,
		Header:           responseHeader,
		Body:             nativeResponse.Body,
		ContentLength:    nativeResponse.ContentLength,
		TransferEncoding: append([]string(nil), nativeResponse.TransferEncoding...),
		Close:            nativeResponse.Close,
		Uncompressed:     nativeResponse.Uncompressed,
		Request:          request,
	}, nil
}

func requestBytes(request *http.Request) ([]byte, error) {
	if request.Body == nil || request.Body == http.NoBody {
		return nil, nil
	}
	if request.GetBody != nil {
		body, err := request.GetBody()
		if err != nil {
			return nil, err
		}
		defer body.Close()
		return io.ReadAll(body)
	}
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

type browserHeaderLayoutKey struct{}
type browserHeaderLayoutValue struct {
	initiator   Initiator
	authorOrder []string
}

func withBrowserHeaderLayout(ctx context.Context, initiator Initiator, order []string) context.Context {
	return context.WithValue(ctx, browserHeaderLayoutKey{}, browserHeaderLayoutValue{initiator: initiator, authorOrder: append([]string(nil), order...)})
}

func browserHeaderLayout(ctx context.Context) (Initiator, []string) {
	value, _ := ctx.Value(browserHeaderLayoutKey{}).(browserHeaderLayoutValue)
	return value.initiator, value.authorOrder
}

func chromeHeaderOrder(headers http.Header, authorOrder []string, initiator Initiator) []string {
	// Renderer-owned fields are copied by iterating Blink's HTTPHeaderMap. That
	// is a WTF HashMap, so author headers are neither alphabetic nor insertion
	// ordered. Fields appended by the browser/network service retain their own
	// stable phase order around that renderer map.
	if initiator == Navigation || initiator == Iframe {
		preferred := []string{"host", "connection", "content-length", "cache-control"}
		if headers.Get("Sec-CH-UA-Full-Version-List") != "" {
			preferred = append(preferred, "sec-ch-ua-full-version-list", "sec-ch-ua-platform", "accept-language", "sec-ch-ua", "sec-ch-ua-bitness", "sec-ch-ua-mobile", "sec-ch-ua-model", "sec-ch-ua-arch", "sec-ch-ua-full-version")
		} else {
			preferred = append(preferred, "sec-ch-ua", "sec-ch-ua-mobile", "sec-ch-ua-platform", "accept-language")
		}
		preferred = append(preferred, "upgrade-insecure-requests", "user-agent", "content-type", "sec-ch-ua-platform-version", "accept", "origin", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-user", "sec-fetch-dest", "referer", "accept-encoding", "priority", "cookie")
		return presentHeaders(headers, preferred)
	}
	late := []string{"accept", "origin", "sec-fetch-site", "sec-fetch-mode", "sec-fetch-user", "sec-fetch-dest", "referer", "accept-encoding", "priority", "cookie"}
	lateSet := make(map[string]bool, len(late)+3)
	for _, name := range late {
		lateSet[name] = true
	}
	lateSet["content-length"], lateSet["host"], lateSet["connection"] = true, true, true

	seen := make(map[string]bool, len(headers))
	insertion := make([]string, 0, len(headers))
	appendEarly := func(name string) {
		name = strings.ToLower(name)
		if seen[name] || lateSet[name] || name == strings.ToLower(fhttp.HeaderOrderKey) {
			return
		}
		if _, ok := headers[http.CanonicalHeaderKey(name)]; !ok {
			return
		}
		seen[name] = true
		insertion = append(insertion, name)
	}
	for _, name := range authorOrder {
		appendEarly(name)
	}
	// This sequence describes generic renderer/network construction phases;
	// the externally visible order is produced by the pinned hash table below.
	for _, name := range []string{
		"sec-ch-ua-full-version-list", "sec-ch-ua-platform", "accept-language",
		"sec-ch-ua", "sec-ch-ua-bitness", "sec-ch-ua-mobile", "sec-ch-ua-model",
		"sec-ch-ua-arch", "sec-ch-ua-full-version", "upgrade-insecure-requests",
		"user-agent", "content-type", "sec-ch-ua-platform-version", "cache-control",
	} {
		appendEarly(name)
	}
	remaining := make([]string, 0, len(headers))
	for name := range headers {
		lower := strings.ToLower(name)
		if !seen[lower] && !lateSet[lower] && lower != strings.ToLower(fhttp.HeaderOrderKey) {
			remaining = append(remaining, lower)
		}
	}
	slices.Sort(remaining)
	for _, name := range remaining {
		appendEarly(name)
	}

	order := make([]string, 0, len(headers))
	if _, ok := headers["Content-Length"]; ok {
		order = append(order, "content-length")
	}
	order = append(order, blinkHashTableOrder(insertion)...)
	for _, name := range late {
		if _, ok := headers[http.CanonicalHeaderKey(name)]; ok {
			order = append(order, name)
		}
	}
	return order
}

func presentHeaders(headers http.Header, preferred []string) []string {
	order := make([]string, 0, len(headers))
	seen := make(map[string]bool, len(headers))
	for _, name := range preferred {
		if _, ok := headers[http.CanonicalHeaderKey(name)]; ok {
			order = append(order, name)
			seen[name] = true
		}
	}
	remaining := make([]string, 0, len(headers))
	for name := range headers {
		lower := strings.ToLower(name)
		if !seen[lower] && lower != strings.ToLower(fhttp.HeaderOrderKey) {
			remaining = append(remaining, lower)
		}
	}
	slices.Sort(remaining)
	return append(order, remaining...)
}

func blinkHashTableOrder(insertion []string) []string {
	var table []string
	insert := func(name string) {
		mask := len(table) - 1
		i, probe := int(blinkHeaderHash(name))&mask, 0
		for table[i] != "" {
			probe++
			i = (i + probe) & mask
		}
		table[i] = name
	}
	rehash := func(size int) {
		old := table
		table = make([]string, size)
		for _, name := range old {
			if name != "" {
				insert(name)
			}
		}
	}
	for _, name := range insertion {
		if len(table) == 0 {
			rehash(8)
		}
		insert(name)
		count := 0
		for _, entry := range table {
			if entry != "" {
				count++
			}
		}
		if count*2 >= len(table) {
			rehash(len(table) * 2)
		}
	}
	result := make([]string, 0, len(insertion))
	for _, name := range table {
		if name != "" {
			result = append(result, name)
		}
	}
	return result
}

func (t *TLSClientTransport) CloseIdleConnections() {
	t.client.CloseIdleConnections()
	t.insecureMu.Lock()
	if t.insecureClient != nil {
		t.insecureClient.CloseIdleConnections()
	}
	t.insecureMu.Unlock()
	if closer, ok := t.fallback.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}
