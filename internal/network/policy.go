package network

import (
	"net/http"
	"sync"
)

// RequestPolicy belongs to one Page's Loader. Its document, frames and workers
// share these overrides without changing another Page in the browser context.
// Cache entries, cookies, blobs and connection state have their own shared owners.
type RequestPolicy struct {
	mu            sync.RWMutex
	cacheDisabled bool
	offline       bool
	extraHeaders  http.Header
}

type PolicySnapshot struct {
	CacheDisabled bool
	Offline       bool
	ExtraHeaders  http.Header
}

func (p *RequestPolicy) Snapshot() PolicySnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return PolicySnapshot{p.cacheDisabled, p.offline, p.extraHeaders.Clone()}
}

func (p *RequestPolicy) Offline() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.offline
}

func (p *RequestPolicy) SetCacheDisabled(disabled bool) {
	p.mu.Lock()
	p.cacheDisabled = disabled
	p.mu.Unlock()
}

func (p *RequestPolicy) SetOffline(offline bool) {
	p.mu.Lock()
	p.offline = offline
	p.mu.Unlock()
}

func (p *RequestPolicy) SetExtraHeaders(headers http.Header) {
	p.mu.Lock()
	p.extraHeaders = headers.Clone()
	p.mu.Unlock()
}
