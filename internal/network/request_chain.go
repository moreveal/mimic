package network

import "net/url"

// The URL list is the authoritative redirect history. Cookie access, Fetch
// Metadata and CORS derive their projections from it; returning to the initial
// origin does not erase an intervening cross-site hop. URLs are immutable copies.
type requestChain struct{ urls []*url.URL }

func (r *Request) beginChain() {
	if len(r.chain.urls) == 0 && r.URL != nil {
		u := *r.URL
		r.chain.urls = []*url.URL{&u}
	}
}

func (r *Request) redirectChain(target *url.URL) {
	r.beginChain()
	u := *target
	urls := append([]*url.URL(nil), r.chain.urls...)
	r.chain.urls = append(urls, &u)
}

func (r Request) initiatingURL() *url.URL {
	if r.SourceURL != nil {
		return r.SourceURL
	}
	return r.Referrer
}

func (r Request) chainSite() string {
	source := r.initiatingURL()
	if source == nil {
		return "none"
	}
	if r.OpaqueOrigin {
		return "cross-site"
	}
	site := "same-origin"
	check := func(u *url.URL) {
		if !requestSameSite(source, u) {
			site = "cross-site"
		} else if site == "same-origin" && !sameRequestOrigin(source, u) {
			site = "same-site"
		}
	}
	for _, u := range r.chain.urls {
		check(u)
	}
	check(r.URL)
	return site
}

func (r Request) redirectTaintedOrigin() bool {
	source := r.initiatingURL()
	for i := 1; i < len(r.chain.urls); i++ {
		previous, next := r.chain.urls[i-1], r.chain.urls[i]
		if !sameRequestOrigin(previous, next) && !sameRequestOrigin(source, previous) {
			return true
		}
	}
	return false
}
