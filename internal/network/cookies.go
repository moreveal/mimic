package network

import (
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

type cookieKey struct {
	domain, path, name string
	partition          CookiePartitionKey
}
type storedCookie struct {
	cookie    http.Cookie
	hostOnly  bool
	order     uint64
	partition CookiePartitionKey
	created   time.Time
}

type CookieStore struct {
	mu           sync.RWMutex
	jar          map[cookieKey]storedCookie
	nextOrder    uint64
	observers    map[uint64]func(CookieSnapshot, bool)
	nextObserver uint64
}

func NewCookieStore() *CookieStore { return &CookieStore{jar: make(map[cookieKey]storedCookie)} }

func cookieHost(host string) string {
	host = strings.ToLower(host)
	if net.ParseIP(host) != nil {
		return host
	}
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return ""
	}
	return ascii
}
func cookieDomainMatches(host, domain string) bool {
	return host == domain || (net.ParseIP(host) == nil && strings.HasSuffix(host, "."+domain))
}
func cookiePathMatches(path, scope string) bool {
	return path == scope || (strings.HasPrefix(path, scope) && (strings.HasSuffix(scope, "/") || len(path) > len(scope) && path[len(scope)] == '/'))
}
func defaultCookiePath(u *url.URL) string {
	path := u.EscapedPath()
	if !strings.HasPrefix(path, "/") {
		return "/"
	}
	if i := strings.LastIndex(path, "/"); i > 0 {
		return path[:i]
	}
	return "/"
}
func secureCookieURL(u *url.URL) bool {
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	ip := net.ParseIP(host)
	return u.Scheme == "https" || host == "localhost" || strings.HasSuffix(host, ".localhost") || ip != nil && ip.IsLoopback()
}
func (s *CookieStore) Set(u *url.URL, c *http.Cookie, context ...CookieContext) {
	s.set(u, c, false, context...)
}
func (s *CookieStore) set(u *url.URL, c *http.Cookie, script bool, context ...CookieContext) {
	s.setWithPartition(u, c, script, nil, context...)
}

// SetWithPartition imports an explicit CDP key, whose ancestor bit is supplied
// by the client rather than inferred from the document making a request.
func (s *CookieStore) SetWithPartition(u *url.URL, c *http.Cookie, key *CookiePartitionKey) {
	s.setWithPartition(u, c, false, key)
}

func (s *CookieStore) setWithPartition(u *url.URL, c *http.Cookie, script bool, explicit *CookiePartitionKey, context ...CookieContext) {
	if u == nil || c == nil || (u.Scheme != "http" && u.Scheme != "https") {
		return
	}
	host := cookieHost(u.Hostname())
	if host == "" || c.Secure && !secureCookieURL(u) || script && c.HttpOnly {
		return
	}
	// Chrome rejects SameSite=None without the Secure attribute, even on a
	// potentially trustworthy loopback URL. Trustworthiness only permits a
	// Secure cookie over that URL; it does not supply the missing attribute.
	if c.SameSite == http.SameSiteNoneMode && !c.Secure {
		return
	}
	// Subresource responses and script writes cannot create SameSite cookies
	// in a cross-site context. Main-frame responses may set them for the new
	// document even when its initiating request could not send them.
	if c.SameSite != http.SameSiteNoneMode && cookieAccess(u, context) == CookieAccessCrossSite &&
		!(len(context) != 0 && context[0].MainFrameNavigation && !script) {
		return
	}
	domain, hostOnly := host, c.Domain == ""
	if !hostOnly {
		domain = cookieHost(strings.TrimPrefix(c.Domain, "."))
		if domain == "" || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || !cookieDomainMatches(host, domain) {
			return
		}
		suffix, _ := publicsuffix.PublicSuffix(domain)
		if suffix == domain {
			if host != domain {
				return
			}
			hostOnly = true
		}
		if net.ParseIP(host) != nil {
			hostOnly = true
		}
	}
	copy := *c
	copy.Domain = domain
	if !strings.HasPrefix(copy.Path, "/") {
		copy.Path = defaultCookiePath(u)
	}
	var partition CookiePartitionKey
	if copy.Partitioned {
		partition = cookiePartition(u, context)
		if explicit != nil {
			partition = *explicit
		}
		// Partitioned requires Secure. An unavailable/opaque top-level site
		// must never be collapsed into the destination's first-party partition.
		if !copy.Secure || partition.TopLevelSite == "" {
			return
		}
	}
	key := cookieKey{domain, copy.Path, copy.Name, partition}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	old, exists := s.jar[key]
	if exists && script && old.cookie.HttpOnly && (old.cookie.Expires.IsZero() || old.cookie.Expires.After(now)) {
		return
	}
	if copy.MaxAge < 0 || copy.MaxAge == 0 && !copy.Expires.IsZero() && !copy.Expires.After(now) {
		delete(s.jar, key)
		if exists {
			s.notifyLocked(old, true)
		}
		return
	}
	if copy.MaxAge > 0 {
		copy.Expires = now.Add(time.Duration(min(copy.MaxAge, 400*24*60*60)) * time.Second)
	}
	// Chrome 152 moves a replaced cookie behind cookies created earlier at
	// the same path length (including document.cookie replacements).
	order := s.nextOrder
	s.nextOrder++
	created := now
	if exists && (old.cookie.Expires.IsZero() || old.cookie.Expires.After(now)) {
		created = old.created
	}
	s.jar[key] = storedCookie{copy, hostOnly, order, partition, created}
	s.notifyLocked(s.jar[key], false)
}
func (s *CookieStore) SetFromResponse(u *url.URL, h http.Header, context ...CookieContext) {
	for _, c := range (&http.Response{Header: h}).Cookies() {
		s.Set(u, c, context...)
	}
}

// Document writes share the network jar but cannot create or replace HttpOnly cookies.
func (s *CookieStore) SetFromDocument(u *url.URL, value string, context ...CookieContext) {
	h := http.Header{"Set-Cookie": {value}}
	for _, c := range (&http.Response{Header: h}).Cookies() {
		s.set(u, c, true, context...)
	}
}
func (s *CookieStore) matching(u *url.URL, context ...CookieContext) []storedCookie {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	entries := []storedCookie{}
	var host, path string
	var secure bool
	if u != nil {
		host, path, secure = cookieHost(u.Hostname()), u.EscapedPath(), secureCookieURL(u)
		if path == "" {
			path = "/"
		}
	}
	for _, entry := range s.jar {
		c := entry.cookie
		if !c.Expires.IsZero() && !c.Expires.After(now) {
			continue
		}
		if u != nil {
			access := cookieAccess(u, context)
			switch c.SameSite {
			case http.SameSiteNoneMode:
			case http.SameSiteStrictMode:
				if access != CookieAccessStrict {
					continue
				}
			case http.SameSiteLaxMode:
				if access < CookieAccessLax {
					continue
				}
			default:
				// Chrome's default-Lax compatibility window applies only to
				// recent cookies lacking an explicit SameSite value.
				if access < CookieAccessLax && !(access == CookieAccessLaxUnsafe && now.Sub(entry.created) <= 2*time.Minute) {
					continue
				}
			}
			if c.Partitioned && entry.partition != cookiePartition(u, context) {
				continue
			}
			if (entry.hostOnly && host != c.Domain) || !cookieDomainMatches(host, c.Domain) || !cookiePathMatches(path, c.Path) || c.Secure && !secure {
				continue
			}
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if u != nil && len(entries[i].cookie.Path) != len(entries[j].cookie.Path) {
			return len(entries[i].cookie.Path) > len(entries[j].cookie.Path)
		}
		return entries[i].order < entries[j].order
	})
	return entries
}
func cookieCopies(entries []storedCookie) []*http.Cookie {
	out := make([]*http.Cookie, 0, len(entries))
	for _, entry := range entries {
		copy := entry.cookie
		out = append(out, &copy)
	}
	return out
}
func (s *CookieStore) ForURL(u *url.URL, context ...CookieContext) []*http.Cookie {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil
	}
	return cookieCopies(s.matching(u, context...))
}
func (s *CookieStore) All() []*http.Cookie { return cookieCopies(s.matching(nil)) }

type CookieSnapshot struct {
	Cookie       http.Cookie
	HostOnly     bool
	PartitionKey *CookiePartitionKey
}

// Snapshots expose the canonical jar to diagnostics without discarding CHIPS
// identity or lending mutable references to the store.
func (s *CookieStore) Snapshots() []CookieSnapshot {
	entries := s.matching(nil)
	out := make([]CookieSnapshot, 0, len(entries))
	for _, entry := range entries {
		row := CookieSnapshot{Cookie: entry.cookie, HostOnly: entry.hostOnly}
		if entry.cookie.Partitioned {
			key := entry.partition
			row.PartitionKey = &key
		}
		out = append(out, row)
	}
	return out
}
func (s *CookieStore) Delete(domain, name string) {
	domain = cookieHost(strings.TrimPrefix(domain, "."))
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, entry := range s.jar {
		if key.domain == domain && key.name == name {
			delete(s.jar, key)
			s.notifyLocked(entry, true)
		}
	}
}

// DeleteScoped follows CDP's partition selection: an omitted key selects only
// unpartitioned cookies. Empty path matches all paths in the selected domain.
func (s *CookieStore) DeleteScoped(domain, path, name string, partition *CookiePartitionKey) {
	domain = cookieHost(strings.TrimPrefix(domain, "."))
	var selected CookiePartitionKey
	if partition != nil {
		selected = *partition
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, entry := range s.jar {
		if key.domain == domain && key.name == name && (path == "" || key.path == path) && key.partition == selected {
			delete(s.jar, key)
			s.notifyLocked(entry, true)
		}
	}
}
func (s *CookieStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.jar {
		s.notifyLocked(entry, true)
	}
	s.jar = make(map[cookieKey]storedCookie)
	s.nextOrder = 0
}

// Subscribe reports canonical mutations, including HTTP and CDP writes. The
// callback runs under the jar lock and may only enqueue work, never call the jar.
func (s *CookieStore) Subscribe(callback func(CookieSnapshot, bool)) func() {
	s.mu.Lock()
	s.nextObserver++
	id := s.nextObserver
	if s.observers == nil {
		s.observers = map[uint64]func(CookieSnapshot, bool){}
	}
	s.observers[id] = callback
	s.mu.Unlock()
	return func() { s.mu.Lock(); delete(s.observers, id); s.mu.Unlock() }
}
func snapshotCookie(entry storedCookie) CookieSnapshot {
	row := CookieSnapshot{Cookie: entry.cookie, HostOnly: entry.hostOnly}
	if entry.cookie.Partitioned {
		key := entry.partition
		row.PartitionKey = &key
	}
	return row
}
func (s *CookieStore) notifyLocked(entry storedCookie, deleted bool) {
	if entry.cookie.HttpOnly {
		return
	}
	row := snapshotCookie(entry)
	for _, callback := range s.observers {
		callback(row, deleted)
	}
}
func (s *CookieStore) DocumentSnapshots(u *url.URL, access ...CookieContext) []CookieSnapshot {
	out := []CookieSnapshot{}
	for _, entry := range s.matching(u, access...) {
		if !entry.cookie.HttpOnly {
			out = append(out, snapshotCookie(entry))
		}
	}
	return out
}

// Visible uses the same cookie selection algorithm as HTTP and document.cookie.
// Deleted records are checked without their old expiry, since the deletion is
// itself the observation being delivered.
func (row CookieSnapshot) Visible(u *url.URL, deleted bool, access ...CookieContext) bool {
	entry := storedCookie{cookie: row.Cookie, hostOnly: row.HostOnly, created: time.Now()}
	if deleted {
		entry.cookie.Expires = time.Time{}
	}
	if row.PartitionKey != nil {
		entry.partition = *row.PartitionKey
	}
	temporary := CookieStore{jar: map[cookieKey]storedCookie{{}: entry}}
	return !row.Cookie.HttpOnly && len(temporary.matching(u, access...)) != 0
}
func (s *CookieStore) SetFromCookieStore(u *url.URL, c *http.Cookie, access ...CookieContext) {
	s.set(u, c, true, access...)
}
