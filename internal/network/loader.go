package network

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	"github.com/moreveal/mimic/internal/monotime"
	"github.com/moreveal/mimic/internal/state"
	"github.com/moreveal/mimic/internal/trace"
)

type Initiator string

const (
	Navigation Initiator = "navigation"
	Fetch      Initiator = "fetch"
	XHR        Initiator = "xhr"
	Script     Initiator = "script"
	Stylesheet Initiator = "stylesheet"
	Iframe     Initiator = "iframe"
	Image      Initiator = "image"
	Worker     Initiator = "worker"
	Other      Initiator = "other"
)

type Request struct {
	// ClientIsWorker identifies the initiating realm, not the resource type:
	// a worker script loaded by a document still has a document client.
	ClientIsWorker       bool
	TopLevelURL          *url.URL
	Credentials          string
	OpaqueOrigin         bool
	HasCrossSiteAncestor bool
	// Worker script and worker Fetch requests do not acquire document hints.
	OmitClientHints bool
	ClientHints     *ClientHintsContext
	ID, ContextID   string
	URL             *url.URL
	Referrer        *url.URL
	// SourceURL is the initiating document/worker URL. It remains available
	// for Fetch Metadata and origin decisions when Referrer-Policy suppresses
	// the wire Referer header.
	SourceURL *url.URL
	// SourceOrigin is the client's security origin when it differs from its
	// document URL, as for inherited about:blank/srcdoc and blob contexts. It
	// governs CORS, credentials and Fetch Metadata without inventing a Referer.
	SourceOrigin *url.URL
	// UserActivation is captured from the navigation initiator, not inferred from
	// its destination. Script and iframe requests need not be user activated.
	UserActivation bool
	ReferrerPolicy string
	Method         string
	Headers        http.Header
	// AuthorHeaderOrder preserves the order in which script supplied distinct
	// header names. Blink stores those fields in HTTPHeaderMap; its pinned WTF
	// HashMap iteration, rather than Go map iteration, feeds the network stack.
	AuthorHeaderOrder []string
	Body              []byte
	Initiator         Initiator
	// Mode and Destination are Fetch-layer properties, not transport guesses.
	// They are populated by browser consumers when an element changes the
	// default request mode (for example, a crossorigin classic script).
	Mode        string
	Destination string
	// Redirect is the Fetch redirect mode; the empty value means follow.
	Redirect string
	// PerformanceInitiatorType is the Resource Timing projection. It is kept
	// separate from Initiator because CDP may classify a browser-owned favicon
	// lookup as Other while PerformanceResourceTiming exposes "img".
	PerformanceInitiatorType string
	// Resource Timing belongs to the initiating document/worker, including
	// across redirects and navigation of its browsing context. Capture these
	// values on the agent's clock; transport diagnostics keep their wall clock.
	// They deliberately live on Request, never on cached Response objects.
	PerformanceOwner string
	PerformanceStart time.Time
	// criticalCHRestarted is loader-owned navigation state. It prevents a
	// malformed or changing response from causing an unbounded internal retry.
	criticalCHRestarted bool
	operationStarted    time.Time
	redirectEnd         float64
	timingAllowFailed   bool
	redirectCount       int
	corsPrepared        bool
	corsPreflight       bool
	corsUnsafeHeaders   []string
	reportingFailure    bool
	chain               requestChain
}
type Response struct {
	// Referrer is the committed navigation referrer after request policy/redirects.
	Referrer        string
	Redirected      bool
	Type            string
	Status          int
	Headers         http.Header
	Body            []byte
	URL             *url.URL
	Synthetic       bool
	FromCache       bool
	Duration        time.Duration
	EncodedBodySize int64
	Protocol        string
	TransportTiming TransportTimingSnapshot
	// BrowserVisibleTiming is a projection over the complete browser operation.
	// For a Critical-CH navigation restart it includes the real elapsed offset
	// before the final transport attempt; TransportTiming remains per-request.
	BrowserVisibleTiming TransportTimingSnapshot
}
type Decision struct {
	Block    error
	Redirect *url.URL
	Request  *Request
	Response *Response
}
type Interceptor interface {
	Before(context.Context, Request) (Decision, error)
	After(context.Context, Request, Response) (Response, error)
}
type Transport interface {
	RoundTrip(*http.Request) (*http.Response, error)
}
type HTTPTransport struct{ Client *http.Client }

func (t HTTPTransport) RoundTrip(r *http.Request) (*http.Response, error) { return t.Client.Do(r) }

type Loader struct {
	ignoreCertificateErrors atomic.Bool
	ownsTransport           bool
	activityMu              sync.Mutex
	activeLoads             int
	documentLoadSequence    uint64
	documentLoads           map[uint64]context.CancelCauseFunc
	idleSince               [2]time.Time
	transport               Transport
	env                     func() state.Environment
	cookies                 *CookieStore
	session                 *SessionState
	policy                  RequestPolicy
	interceptors            []Interceptor
	interceptorsMu          sync.RWMutex
	trace                   *trace.Recorder
	completedMu             sync.RWMutex
	completed               map[string]Response
	completedOrder          []string
}

func NewLoader(env func() state.Environment, cookies *CookieStore, tr *trace.Recorder) *Loader {
	return NewLoaderWithSession(env, cookies, NewSessionState(), tr)
}
func NewLoaderWithSession(env func() state.Environment, cookies *CookieStore, session *SessionState, tr *trace.Recorder) *Loader {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &Loader{transport: &httpCertificateTransport{HTTPTransport: HTTPTransport{client}}, ownsTransport: true, env: env, cookies: cookies, session: session, trace: tr, completed: map[string]Response{}}
}
func (l *Loader) SetTransport(t Transport) {
	l.CloseOwnedTransport()
	l.transport = t
	l.ownsTransport = false
	l.ignoreCertificateErrors.Store(false)
}

func (l *Loader) Policy() *RequestPolicy { return &l.policy }
func (l *Loader) Use(i Interceptor) func() {
	l.interceptorsMu.Lock()
	l.interceptors = append(l.interceptors, i)
	l.interceptorsMu.Unlock()
	return func() {
		l.interceptorsMu.Lock()
		defer l.interceptorsMu.Unlock()
		for n, x := range l.interceptors {
			if x == i {
				l.interceptors = append(l.interceptors[:n], l.interceptors[n+1:]...)
				break
			}
		}
	}
}
func (l *Loader) Load(ctx context.Context, r Request) (response Response, loadErr error) {
	l.beginActivity()
	defer l.endActivity()
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	if !r.reportingFailure && !r.ClientIsWorker {
		var release func()
		ctx, release = l.trackDocumentLoad(ctx)
		defer release()
		defer func() {
			if loadErr != nil && context.Cause(ctx) == ErrDocumentLoadingStopped {
				loadErr = ErrDocumentLoadingStopped
			}
		}()
	}
	if r.operationStarted.IsZero() {
		r.operationStarted = monotime.Now()
	}
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if !r.reportingFailure {
		r.reportingFailure = true
		defer func() {
			if loadErr != nil {
				l.trace.Add(trace.Network, "failed", map[string]any{"id": r.ID, "url": r.URL.String(), "error": loadErr.Error(), "initiator": r.Initiator, "context": r.ContextID, "canceled": errors.Is(loadErr, context.Canceled) || ctx.Err() == context.Canceled})
				l.trace.Add(trace.Resource, "loadEnd", map[string]any{"id": r.ID, "url": r.URL.String(), "error": loadErr.Error()})
			}
		}()
	}
	if r.Method == "" {
		r.Method = http.MethodGet
	}
	if r.Headers == nil {
		r.Headers = make(http.Header)
	}
	r.beginChain()
	if err := l.prepareFetchCORS(ctx, &r); err != nil {
		return Response{}, err
	}
	snapshot := l.policy.Snapshot()
	if r.URL.Scheme == "data" {
		body, contentType, err := decodeDataURL(r.URL)
		if err != nil {
			return Response{}, err
		}
		headers := make(http.Header)
		headers.Set("Content-Type", contentType)
		return l.after(ctx, r, Response{Status: http.StatusOK, Headers: headers, Body: body, URL: r.URL, Synthetic: true})
	}
	if r.URL.Scheme == "blob" {
		if r.Headers.Get("Accept") == "" {
			r.Headers.Set("Accept", "*/*")
		}
	} else {
		headers := l.env().RequestHeaders()
		if r.ClientIsWorker {
			headers = l.env().WorkerRequestHeaders()
		}
		for k, v := range headers {
			if r.OmitClientHints && strings.HasPrefix(strings.ToLower(k), "sec-ch-") {
				continue
			}
			if r.Headers.Get(k) == "" {
				r.Headers.Set(k, v)
			}
		}
		for k, v := range l.env().ClientHintHeaders(l.acceptedClientHints(r)) {
			if r.Headers.Get(k) == "" {
				r.Headers.Set(k, v)
			}
		}
		applyBrowserRequestHeaders(&r)
		applyStorageAccessHeader(&r, l.env().Network.CookiesEnabled)
		if r.Headers.Get("Accept-Encoding") == "" {
			r.Headers.Set("Accept-Encoding", "gzip, deflate, br, zstd")
		}
	}
	if r.Headers.Get("Referer") == "" {
		if referrer := r.ReferrerValue(); referrer != "" {
			r.Headers.Set("Referer", referrer)
		}
	}
	for k, values := range snapshot.ExtraHeaders {
		r.Headers.Del(k)
		for _, v := range values {
			r.Headers.Add(k, v)
		}
	}
	if !requestIncludesCredentials(r) {
		r.Headers.Del("Cookie")
	}
	var cookiePairs []string
	if l.env().Network.CookiesEnabled && requestIncludesCredentials(r) {
		for _, c := range l.cookies.ForURL(r.URL, r.cookieContext()) {
			cookiePairs = append(cookiePairs, c.Name+"="+c.Value)
		}
	}
	if len(cookiePairs) > 0 && r.Headers.Get("Cookie") == "" {
		r.Headers.Set("Cookie", strings.Join(cookiePairs, "; "))
	}
	visibleHeaders := headerStrings(r.Headers)
	requestStarted := monotime.Now()
	l.trace.Add(trace.Network, "request", map[string]any{"id": r.ID, "url": r.URL.String(), "method": r.Method, "headers": visibleHeaders, "postData": string(r.Body), "initiator": r.Initiator, "context": r.ContextID, "performanceOwner": r.PerformanceOwner, "performanceStart": r.PerformanceStart})
	l.trace.Add(trace.Resource, "loadStart", map[string]any{"id": r.ID, "url": r.URL.String(), "type": r.Initiator, "context": r.ContextID})
	if snapshot.Offline && r.URL.Scheme != "blob" {
		return Response{}, fmt.Errorf("net::ERR_INTERNET_DISCONNECTED")
	}
	interceptors := l.interceptorSnapshot()
	for _, i := range interceptors {
		d, err := i.Before(ctx, r)
		if err != nil {
			return Response{}, err
		}
		if d.Block != nil {
			return Response{}, d.Block
		}
		if d.Redirect != nil {
			r.URL = d.Redirect
		}
		if d.Request != nil {
			r = *d.Request
		}
		if d.Response != nil {
			res := *d.Response
			res.Synthetic = true
			return l.after(ctx, r, res)
		}
	}
	if r.URL.Scheme == "blob" {
		body, contentType, ok := l.session.Blob(r.URL.String())
		if !ok {
			return Response{}, fmt.Errorf("blob URL has been revoked or is unknown")
		}
		headers := make(http.Header)
		if contentType != "" {
			headers.Set("Content-Type", contentType)
		}
		return l.after(ctx, r, Response{Status: http.StatusOK, Headers: headers, Body: body, URL: r.URL, Synthetic: true})
	}
	if cached, ok := l.cachedResponse(r, snapshot); ok {
		l.trace.Add(trace.Network, "cacheHit", map[string]any{"id": r.ID, "url": r.URL.String()})
		cached.FromCache = true
		// The bytes/headers describe the stored representation; elapsed time and
		// transport phases belong to this retrieval, not its original download.
		cached.Duration = monotime.Since(requestStarted)
		cached.TransportTiming.Phases = nil
		cached.BrowserVisibleTiming = TransportTimingSnapshot{Phases: map[string]float64{
			"firstResponseByte": float64(cached.Duration) / float64(time.Millisecond),
			"responseComplete":  float64(cached.Duration) / float64(time.Millisecond),
		}}
		return l.after(ctx, r, cached)
	}
	attempt := l.session.BeginConnection(r.URL)
	timing := newTransportTiming(attempt.Origin, attempt.Key)
	start := timing.started
	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL.String(), bytes.NewReader(r.Body))
	if err != nil {
		return Response{}, err
	}
	req.Header = r.Headers.Clone()
	req = req.WithContext(withBrowserHeaderLayout(req.Context(), r.Initiator, r.AuthorHeaderOrder))
	req = req.WithContext(withTransportTiming(httptrace.WithClientTrace(req.Context(), timing.standardTrace()), timing))
	raw, err := l.roundTrip(req)
	if err != nil {
		timing.mark("responseComplete")
		timingSnapshot := timing.snapshot()
		record := l.session.CompleteConnection(attempt, timingSnapshot, "", true)
		l.trace.Add(trace.Network, "transport", map[string]any{"id": r.ID, "url": r.URL.String(), "timing": timingSnapshot, "sessionCold": attempt.Cold, "connectionState": record.Status, "error": err.Error()})
		return Response{}, err
	}
	stopBodyCancellation := context.AfterFunc(ctx, func() { _ = raw.Body.Close() })
	encodedBody, err := io.ReadAll(io.LimitReader(raw.Body, 32<<20))
	stopBodyCancellation()
	closeErr := raw.Body.Close()
	if contextErr := ctx.Err(); contextErr != nil {
		return Response{}, contextErr
	}
	if err != nil {
		return Response{}, err
	}
	if closeErr != nil {
		return Response{}, closeErr
	}
	encodedBodySize := raw.ContentLength
	if encodedBodySize < 0 {
		encodedBodySize = int64(len(encodedBody))
	}
	body, err := decodeContent(encodedBody, raw.Header.Get("Content-Encoding"))
	if err != nil {
		return Response{}, fmt.Errorf("decode %s response: %w", raw.Header.Get("Content-Encoding"), err)
	}
	timing.mark("responseComplete")
	timing.setResponseProtocol(raw.Proto)
	timingSnapshot := timing.snapshot()
	record := l.session.CompleteConnection(attempt, timingSnapshot, raw.Proto, raw.Close)
	l.trace.Add(trace.Network, "transport", map[string]any{"id": r.ID, "url": r.URL.String(), "timing": timingSnapshot, "sessionCold": attempt.Cold, "connectionState": record.Status})
	browserTiming := timingSnapshot.shifted(float64(start.Sub(r.operationStarted)) / float64(time.Millisecond))
	res := Response{Status: raw.StatusCode, Headers: raw.Header.Clone(), Body: body, URL: r.URL, Duration: monotime.Since(start), EncodedBodySize: encodedBodySize, Protocol: raw.Proto, TransportTiming: timingSnapshot, BrowserVisibleTiming: browserTiming}
	acceptedBefore := l.session.ClientHints(r.URL)
	l.session.AcceptClientHints(r.URL, res.Headers.Get("Accept-CH"))
	if l.env().Network.CookiesEnabled && requestIncludesCredentials(r) {
		l.cookies.SetFromResponse(r.URL, res.Headers, r.cookieContext())
	}
	if missing := criticalClientHintsForRestart(l.env(), r, res, acceptedBefore); len(missing) != 0 {
		l.trace.Add(trace.Network, "criticalClientHintsRestart", map[string]any{"id": r.ID, "url": r.URL.String(), "missing": missing, "connectionId": timingSnapshot.ConnectionID})
		r.criticalCHRestarted = true
		return l.Load(ctx, r)
	}
	l.session.PutCached(r, res, time.Now())
	return l.after(ctx, r, res)
}

func (l *Loader) cachedResponse(request Request, policy PolicySnapshot) (Response, bool) {
	// Chrome's cacheDisabled bypasses reads, while the successful network
	// response still refreshes the shared HTTP cache for other Pages.
	if policy.CacheDisabled {
		return Response{}, false
	}
	return l.session.GetCached(request, time.Now())
}

func criticalClientHintsForRestart(environment state.Environment, request Request, response Response, acceptedBefore map[string]bool) []string {
	if request.criticalCHRestarted || request.Initiator != Navigation || request.URL == nil || request.URL.Scheme != "https" {
		return nil
	}
	accept := headerTokens(response.Headers.Values("Accept-CH"))
	critical := headerTokens(response.Headers.Values("Critical-CH"))
	if len(accept) == 0 || len(critical) == 0 {
		return nil
	}
	available := environment.ClientHintHeaders(accept)
	missing := make([]string, 0)
	for name := range available {
		lower := strings.ToLower(name)
		if critical[lower] && !acceptedBefore[lower] {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	return missing
}

func headerTokens(values []string) map[string]bool {
	tokens := map[string]bool{}
	for _, value := range values {
		for _, token := range strings.Split(value, ",") {
			if token = strings.ToLower(strings.TrimSpace(token)); token != "" {
				tokens[token] = true
			}
		}
	}
	return tokens
}

func decodeContent(encoded []byte, contentEncoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(contentEncoding)) {
	case "", "identity":
		return encoded, nil
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(io.LimitReader(reader, 32<<20))
	case "deflate":
		reader, err := zlib.NewReader(bytes.NewReader(encoded))
		if err == nil {
			defer reader.Close()
			return io.ReadAll(io.LimitReader(reader, 32<<20))
		}
		raw := flate.NewReader(bytes.NewReader(encoded))
		defer raw.Close()
		return io.ReadAll(io.LimitReader(raw, 32<<20))
	case "br":
		return io.ReadAll(io.LimitReader(brotli.NewReader(bytes.NewReader(encoded)), 32<<20))
	case "zstd":
		decoder, err := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(64<<20))
		if err != nil {
			return nil, err
		}
		defer decoder.Close()
		return decoder.DecodeAll(encoded, nil)
	default:
		return nil, fmt.Errorf("unsupported content encoding %q", contentEncoding)
	}
}

func applyBrowserRequestHeaders(r *Request) {
	setDefault := func(name, value string) {
		if r.Headers.Get(name) == "" {
			r.Headers.Set(name, value)
		}
	}
	site := r.chainSite()
	source := r.initiatingURL()
	mode, destination := "no-cors", "empty"
	switch r.Initiator {
	case Navigation, Iframe:
		mode, destination = "navigate", "document"
		if source != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
			origin := source.Scheme + "://" + source.Host
			policy := strings.ToLower(r.ReferrerPolicy)
			downgrade := source.Scheme == "https" && r.URL.Scheme != "https"
			if r.OpaqueOrigin || r.redirectTaintedOrigin() || source.Scheme != "http" && source.Scheme != "https" || policy == "no-referrer" || policy == "same-origin" && !sameRequestOrigin(source, r.URL) || downgrade && (policy == "strict-origin" || policy == "strict-origin-when-cross-origin" || policy == "no-referrer-when-downgrade" || policy == "") {
				origin = "null"
			}
			setDefault("Origin", origin)
		}
		if r.Initiator == Iframe {
			destination = "iframe"
		}
		setDefault("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
		setDefault("Upgrade-Insecure-Requests", "1")
		if r.UserActivation {
			setDefault("Sec-Fetch-User", "?1")
		}
	case Worker:
		mode, destination = "same-origin", "worker"
		setDefault("Accept", "*/*")
	case Script:
		destination = "script"
		setDefault("Accept", "*/*")
	case Stylesheet:
		destination = "style"
		setDefault("Accept", "text/css,*/*;q=0.1")
	case Image:
		destination = "image"
		setDefault("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	case Other:
		// The current browser-owned Other consumer is the fallback favicon.
		// CDP classifies that lookup as Other, while Fetch still treats it as
		// an image request on the wire.
		destination = "image"
		setDefault("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	case Fetch, XHR:
		mode = "cors"
		setDefault("Accept", "*/*")
		if source != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
			origin := source.Scheme + "://" + source.Host
			if r.OpaqueOrigin || r.redirectTaintedOrigin() || source.Scheme == "file" || source.Scheme == "data" {
				origin = "null"
			}
			setDefault("Origin", origin)
		}
	}
	if r.Mode != "" {
		mode = r.Mode
	}
	if r.Destination != "" {
		destination = r.Destination
	}
	setDefault("Sec-Fetch-Dest", destination)
	setDefault("Sec-Fetch-Mode", mode)
	setDefault("Sec-Fetch-Site", site)
	switch r.Initiator {
	case Navigation, Iframe:
		setDefault("Priority", "u=0, i")
	case Stylesheet:
		setDefault("Priority", "u=0")
	case Fetch, XHR, Other:
		setDefault("Priority", "u=1, i")
	case Image:
		setDefault("Priority", "i")
	}
}
func (l *Loader) after(ctx context.Context, r Request, res Response) (Response, error) {
	res.Referrer = r.Headers.Get("Referer")
	interceptors := l.interceptorSnapshot()
	for n := len(interceptors) - 1; n >= 0; n-- {
		var err error
		res, err = interceptors[n].After(ctx, r, res)
		if err != nil {
			return Response{}, err
		}
	}
	encodedBodySize := res.EncodedBodySize
	if encodedBodySize <= 0 {
		encodedBodySize = int64(len(res.Body))
	}
	transferSize := encodedBodySize + 300
	if res.FromCache || res.Synthetic {
		transferSize = 0
	}
	performanceInitiatorType := r.PerformanceInitiatorType
	if performanceInitiatorType == "" {
		performanceInitiatorType = string(r.Initiator)
		switch r.Initiator {
		case Image:
			performanceInitiatorType = "img"
		case XHR:
			performanceInitiatorType = "xmlhttprequest"
		case Stylesheet:
			performanceInitiatorType = "link"
		}
	}
	l.trace.Add(trace.Network, "response", map[string]any{"id": r.ID, "url": r.URL.String(), "status": res.Status, "headers": headerStrings(res.Headers), "mimeType": strings.Split(res.Headers.Get("Content-Type"), ";")[0], "encodedDataLength": len(res.Body), "encodedBodySize": encodedBodySize, "decodedBodySize": len(res.Body), "transferSize": transferSize, "durationMs": float64(res.Duration) / float64(time.Millisecond), "protocol": res.Protocol, "transportTiming": res.TransportTiming, "browserVisibleTiming": res.BrowserVisibleTiming, "connectionReused": res.TransportTiming.Reused, "connectionId": res.TransportTiming.ConnectionID, "fromCache": res.FromCache, "initiator": r.Initiator, "performanceInitiatorType": performanceInitiatorType,
		"performanceURL": r.performanceURL(), "performanceRedirectEnd": r.redirectEnd,
		"performanceRedirectCount": r.redirectCount, "performanceTimingAllowFailed": r.performanceTimingAllowFailed(res.Headers),
		"performanceCORSAccessible": r.Initiator == Fetch && r.Mode != "no-cors" && corsResponseAllowed(r, res.Headers), "synthetic": res.Synthetic, "context": r.ContextID, "performanceOwner": r.PerformanceOwner, "performanceStart": r.PerformanceStart})
	l.remember(r.ID, res)
	l.trace.Add(trace.Resource, "loadEnd", map[string]any{"id": r.ID, "url": r.URL.String(), "status": res.Status, "type": r.Initiator})
	if fetchCrossOrigin(r) && !r.corsPreflight && r.Mode != "no-cors" && !corsResponseAllowed(r, res.Headers) {
		return Response{}, fmt.Errorf("CORS response denied")
	}
	if isRedirectStatus(res.Status) {
		if r.Redirect == "manual" {
			return Response{Status: 0, Type: "opaqueredirect", URL: r.URL, Headers: make(http.Header)}, nil
		}
		if loc := res.Headers.Get("Location"); loc != "" {
			if r.Redirect == "error" {
				return Response{}, fmt.Errorf("redirect forbidden by request redirect mode")
			}
			if r.redirectCount >= 20 {
				return Response{}, fmt.Errorf("too many redirects")
			}
			u, err := r.URL.Parse(loc)
			if err != nil {
				return Response{}, err
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				return Response{}, fmt.Errorf("redirect to unsupported scheme %q", u.Scheme)
			}
			if (r.Initiator == Fetch || r.Initiator == XHR) && u.User != nil {
				return Response{}, fmt.Errorf("redirect URL contains credentials")
			}
			if !strings.Contains(loc, "#") {
				u.Fragment, u.RawFragment = r.URL.Fragment, r.URL.RawFragment
			}
			r.Headers = r.Headers.Clone()
			for _, name := range []string{"Sec-CH-UA-Arch", "Sec-CH-UA-Bitness", "Sec-CH-UA-Full-Version", "Sec-CH-UA-Full-Version-List", "Sec-CH-UA-Model", "Sec-CH-UA-Platform-Version"} {
				r.Headers.Del(name)
			}
			if !sameRedirectOrigin(r.URL, u) {
				r.Headers.Del("Authorization")
				r.Headers.Del("Proxy-Authorization")
			}
			if (res.Status == 301 || res.Status == 302) && r.Method == http.MethodPost || res.Status == 303 && r.Method != http.MethodGet && r.Method != http.MethodHead {
				r.Method, r.Body = http.MethodGet, nil
				for _, name := range []string{"Content-Encoding", "Content-Language", "Content-Location", "Content-Type", "Content-Length"} {
					r.Headers.Del(name)
				}
			}
			// These fields belong to the new request, not to the previous hop.
			// In particular, do not forward the source host's cookie jar entry.
			for _, name := range []string{"Cookie", "Referer", "Origin", "Sec-Fetch-Site", "Sec-Fetch-Storage-Access"} {
				r.Headers.Del(name)
			}
			r.redirectChain(u)
			r.redirectEnd = float64(monotime.Since(r.operationStarted)) / float64(time.Millisecond)
			r.timingAllowFailed = r.timingAllowFailed || !requestTimingAllowed(r, res.Headers)
			r.URL = u
			r.redirectCount++
			return l.Load(ctx, r)
		}
	}
	res.Redirected = r.redirectCount > 0
	return filterFetchResponse(r, res), nil
}

func isRedirectStatus(status int) bool {
	return status == 301 || status == 302 || status == 303 || status == 307 || status == 308
}

func sameRedirectOrigin(a, b *url.URL) bool {
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		if strings.EqualFold(u.Scheme, "https") {
			return "443"
		}
		return "80"
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
}
func headerStrings(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, values := range h {
		out[k] = strings.Join(values, ", ")
	}
	return out
}
func (l *Loader) interceptorSnapshot() []Interceptor {
	l.interceptorsMu.RLock()
	defer l.interceptorsMu.RUnlock()
	return append([]Interceptor(nil), l.interceptors...)
}
func (l *Loader) remember(id string, res Response) {
	l.completedMu.Lock()
	defer l.completedMu.Unlock()
	copy := res
	copy.Body = append([]byte(nil), res.Body...)
	copy.Headers = res.Headers.Clone()
	l.completed[id] = copy
	l.completedOrder = append(l.completedOrder, id)
	if len(l.completedOrder) > 128 {
		old := l.completedOrder[0]
		l.completedOrder = l.completedOrder[1:]
		delete(l.completed, old)
	}
}
func (l *Loader) Completed(id string) (Response, bool) {
	l.completedMu.RLock()
	defer l.completedMu.RUnlock()
	r, ok := l.completed[id]
	if ok {
		r.Body = append([]byte(nil), r.Body...)
		r.Headers = r.Headers.Clone()
	}
	return r, ok
}

// CompletedURL returns the latest retained response for an absolute URL.
func (l *Loader) CompletedURL(rawURL string) (Response, bool) {
	l.completedMu.RLock()
	defer l.completedMu.RUnlock()
	for i := len(l.completedOrder) - 1; i >= 0; i-- {
		r, ok := l.completed[l.completedOrder[i]]
		if ok && r.URL != nil && r.URL.String() == rawURL {
			r.Body = append([]byte(nil), r.Body...)
			r.Headers = r.Headers.Clone()
			return r, true
		}
	}
	return Response{}, false
}

// ReferrerValue is shared by the wire request and inline document navigation.
func (r Request) ReferrerValue() string {
	if r.Referrer == nil || r.URL == nil {
		return ""
	}
	referrer := *r.Referrer
	referrer.User = nil
	referrer.Fragment = ""
	referrer.RawFragment = ""
	policy := "strict-origin-when-cross-origin"
	for _, token := range strings.Split(r.ReferrerPolicy, ",") {
		switch strings.TrimSpace(strings.ToLower(token)) {
		case "no-referrer", "no-referrer-when-downgrade", "same-origin", "origin", "strict-origin", "origin-when-cross-origin", "strict-origin-when-cross-origin", "unsafe-url":
			policy = strings.TrimSpace(strings.ToLower(token))
		}
	}
	sameOrigin := referrer.Scheme == r.URL.Scheme && referrer.Host == r.URL.Host
	downgrade := referrer.Scheme == "https" && r.URL.Scheme != "https" && r.URL.Scheme != "about"
	suppress := policy == "no-referrer" || policy == "same-origin" && !sameOrigin || downgrade && (policy == "no-referrer-when-downgrade" || policy == "strict-origin" || policy == "strict-origin-when-cross-origin")
	if (referrer.Scheme == "http" || referrer.Scheme == "https") && !suppress {
		if policy == "origin" || policy == "strict-origin" || !sameOrigin && (policy == "origin-when-cross-origin" || policy == "strict-origin-when-cross-origin") {
			referrer.Path = "/"
			referrer.RawPath = ""
			referrer.RawQuery = ""
			referrer.ForceQuery = false
		}
		return referrer.String()
	}
	return ""
}
