package network

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// SessionState is the browser-context scoped source of truth for network
// policy and the HTTP cache. CDP mutates this object; every loader observes it.
type SessionState struct {
	mu            sync.RWMutex
	cacheDisabled bool
	offline       bool
	extraHeaders  http.Header
	cache         map[string][]cacheEntry
	clientHints   map[string]map[string]bool
	blobs         map[string]blobEntry
	connections   map[string]ConnectionRecord
}
type blobEntry struct {
	body        []byte
	contentType string
}
type cacheEntry struct {
	response       Response
	expires        time.Time
	vary           []string
	requestHeaders http.Header
}
type SessionSnapshot struct {
	CacheDisabled bool
	Offline       bool
	ExtraHeaders  http.Header
}

type ConnectionStatus string

const (
	ConnectionCold        ConnectionStatus = "cold"
	ConnectionConnecting  ConnectionStatus = "connecting"
	ConnectionEstablished ConnectionStatus = "established"
	ConnectionReusable    ConnectionStatus = "reusable"
	ConnectionClosed      ConnectionStatus = "closed"
)

type ConnectionRecord struct {
	Key            string
	Origin         string
	Status         ConnectionStatus
	ConnectionID   string
	Protocol       string
	Reused         bool
	LastTransition time.Time
}

type ConnectionAttempt struct {
	Key, Origin string
	Cold        bool
}

func NewSessionState() *SessionState {
	return &SessionState{extraHeaders: make(http.Header), cache: map[string][]cacheEntry{}, clientHints: map[string]map[string]bool{}, blobs: map[string]blobEntry{}, connections: map[string]ConnectionRecord{}}
}

func connectionKey(u *url.URL) (string, string) {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ""
	}
	origin := u.Scheme + "://" + u.Host
	return origin, origin
}

// BeginConnection is the single browser-session connection lookup used by all
// resource consumers. The key can later grow to include proxy, credentials and
// network-isolation keys without changing those consumers.
func (s *SessionState) BeginConnection(u *url.URL) ConnectionAttempt {
	key, origin := connectionKey(u)
	if key == "" {
		return ConnectionAttempt{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, exists := s.connections[key]
	cold := !exists || record.Status == ConnectionCold || record.Status == ConnectionClosed
	s.connections[key] = ConnectionRecord{Key: key, Origin: origin, Status: ConnectionConnecting, ConnectionID: record.ConnectionID, Protocol: record.Protocol, Reused: !cold, LastTransition: time.Now()}
	return ConnectionAttempt{Key: key, Origin: origin, Cold: cold}
}

func (s *SessionState) CompleteConnection(attempt ConnectionAttempt, timing TransportTimingSnapshot, protocol string, closed bool) ConnectionRecord {
	if attempt.Key == "" {
		return ConnectionRecord{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	status := ConnectionReusable
	if closed {
		status = ConnectionClosed
	} else if !timing.ReuseKnown {
		status = ConnectionEstablished
	}
	record := ConnectionRecord{Key: attempt.Key, Origin: attempt.Origin, Status: status, ConnectionID: timing.ConnectionID, Protocol: protocol, Reused: timing.Reused, LastTransition: time.Now()}
	s.connections[attempt.Key] = record
	return record
}

func (s *SessionState) Connections() []ConnectionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ConnectionRecord, 0, len(s.connections))
	for _, record := range s.connections {
		out = append(out, record)
	}
	return out
}

func (s *SessionState) PutBlob(raw string, body []byte, contentType string) {
	s.mu.Lock()
	s.blobs[raw] = blobEntry{body: append([]byte(nil), body...), contentType: contentType}
	s.mu.Unlock()
}

func (s *SessionState) RevokeBlob(raw string) {
	s.mu.Lock()
	delete(s.blobs, raw)
	s.mu.Unlock()
}

func (s *SessionState) Blob(raw string) ([]byte, string, bool) {
	s.mu.RLock()
	entry, ok := s.blobs[raw]
	s.mu.RUnlock()
	return append([]byte(nil), entry.body...), entry.contentType, ok
}
func (s *SessionState) Snapshot() SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return SessionSnapshot{s.cacheDisabled, s.offline, s.extraHeaders.Clone()}
}
func (s *SessionState) SetCacheDisabled(v bool) { s.mu.Lock(); s.cacheDisabled = v; s.mu.Unlock() }
func (s *SessionState) SetOffline(v bool)       { s.mu.Lock(); s.offline = v; s.mu.Unlock() }
func (s *SessionState) SetExtraHeaders(h http.Header) {
	s.mu.Lock()
	s.extraHeaders = h.Clone()
	s.mu.Unlock()
}
func (s *SessionState) ClearCache() { s.mu.Lock(); s.cache = map[string][]cacheEntry{}; s.mu.Unlock() }
func (s *SessionState) AcceptClientHints(u *url.URL, header string) {
	if u == nil || header == "" {
		return
	}
	origin := u.Scheme + "://" + u.Host
	s.mu.Lock()
	hints := map[string]bool{}
	for _, name := range strings.Split(header, ",") {
		hints[strings.ToLower(strings.TrimSpace(name))] = true
	}
	s.clientHints[origin] = hints
	s.mu.Unlock()
}
func (s *SessionState) ClientHints(u *url.URL) map[string]bool {
	if u == nil {
		return nil
	}
	origin := u.Scheme + "://" + u.Host
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]bool{}
	for name, enabled := range s.clientHints[origin] {
		out[name] = enabled
	}
	return out
}
func (s *SessionState) GetCached(req Request, now time.Time) (Response, bool) {
	if req.Method != http.MethodGet {
		return Response{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cacheDisabled {
		return Response{}, false
	}
	for _, e := range s.cache[req.URL.String()] {
		if now.After(e.expires) {
			continue
		}
		match := true
		for _, k := range e.vary {
			if req.Headers.Get(k) != e.requestHeaders.Get(k) {
				match = false
				break
			}
		}
		if match {
			r := e.response
			r.Body = append([]byte(nil), r.Body...)
			r.Headers = r.Headers.Clone()
			return r, true
		}
	}
	return Response{}, false
}
func (s *SessionState) PutCached(req Request, res Response, now time.Time) {
	if req.Method != http.MethodGet || res.Status != http.StatusOK {
		return
	}
	cc := strings.ToLower(res.Headers.Get("Cache-Control"))
	if strings.Contains(cc, "no-store") {
		return
	}
	maxAge := time.Duration(0)
	for _, part := range strings.Split(cc, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "max-age=") {
			if d, err := time.ParseDuration(strings.TrimPrefix(part, "max-age=") + "s"); err == nil {
				maxAge = d
			}
		}
	}
	if maxAge <= 0 {
		if expires, err := http.ParseTime(res.Headers.Get("Expires")); err == nil {
			maxAge = expires.Sub(now)
		}
	}
	if maxAge <= 0 {
		return
	}
	vary := []string{}
	for _, v := range strings.Split(res.Headers.Get("Vary"), ",") {
		v = strings.TrimSpace(v)
		if v == "*" {
			return
		}
		if v != "" {
			vary = append(vary, http.CanonicalHeaderKey(v))
		}
	}
	copy := res
	copy.Body = append([]byte(nil), res.Body...)
	copy.Headers = res.Headers.Clone()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cacheDisabled {
		return
	}
	s.cache[req.URL.String()] = append(s.cache[req.URL.String()], cacheEntry{copy, now.Add(maxAge), vary, req.Headers.Clone()})
}
