package cdp

import (
	"fmt"

	"github.com/moreveal/mimic/internal/browser"
)

// Chrome accepts the override on both browser and page sessions. Browser
// sessions cover existing/future Pages; page sessions only cover their target.
// Multiple attached controllers are combined without mutating shared TLS pools.
func (s *session) setCertificateOverride(ignore bool) error {
	s.server.certificateMu.Lock()
	defer s.server.certificateMu.Unlock()
	pages := []*browser.Page{s.page}
	if s.browserSession {
		pages = s.server.pages()
	}
	if ignore {
		for _, page := range pages {
			if !page.Loader().SupportsCertificateOverride() {
				return fmt.Errorf("certificate override is unsupported by this transport")
			}
		}
	}
	s.stateMu.Lock()
	s.ignoreCertificateErrors = ignore
	s.stateMu.Unlock()
	for _, page := range pages {
		s.server.applyCertificatePolicyLocked(page)
	}
	return nil
}

func (s *Server) applyCertificatePolicy(page *browser.Page) {
	s.certificateMu.Lock()
	defer s.certificateMu.Unlock()
	s.applyCertificatePolicyLocked(page)
}
func (s *Server) applyCertificatePolicyLocked(page *browser.Page) {
	ignore := false
	for _, c := range s.clientSnapshot() {
		for _, session := range c.snapshot() {
			session.stateMu.RLock()
			applies := session.ctx.Err() == nil && (session.browserSession || session.page == page && session.targetType == "page")
			ignore = ignore || applies && session.ignoreCertificateErrors
			session.stateMu.RUnlock()
		}
	}
	_ = page.Loader().SetIgnoreCertificateErrors(ignore)
}

func (s *Server) refreshCertificatePolicies() {
	s.certificateMu.Lock()
	defer s.certificateMu.Unlock()
	for _, page := range s.pages() {
		s.applyCertificatePolicyLocked(page)
	}
}
