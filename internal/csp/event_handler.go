package csp

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"strings"
)

// AllowsEventHandler uses the attribute-specific directive, whose fallback does
// not include script-src-elem. Nonces cannot authorize event content attributes;
// hashes require unsafe-hashes and cover the exact decoded attribute source.
func (set PolicySet) AllowsEventHandler(source string) bool {
	for _, policy := range set {
		sources, exists := policy.directives["script-src-attr"]
		if !exists {
			sources, exists = policy.directives["script-src"]
		}
		if !exists {
			sources, exists = policy.directives["default-src"]
		}
		if !exists {
			continue
		}
		if contains(sources, "'unsafe-inline'") && !hasNonceOrHash(sources) {
			continue
		}
		allowed := false
		if contains(sources, "'unsafe-hashes'") {
			for _, expression := range sources {
				var hash []byte
				algorithm, expected, ok := strings.Cut(strings.Trim(expression, "'"), "-")
				if !ok {
					continue
				}
				switch algorithm {
				case "sha256":
					sum := sha256.Sum256([]byte(source))
					hash = sum[:]
				case "sha384":
					sum := sha512.Sum384([]byte(source))
					hash = sum[:]
				case "sha512":
					sum := sha512.Sum512([]byte(source))
					hash = sum[:]
				default:
					continue
				}
				if base64.StdEncoding.EncodeToString(hash) == expected {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			return false
		}
	}
	return true
}
