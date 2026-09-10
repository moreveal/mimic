// Copyright 2026 uTLS. Licensed under the BSD 3-Clause License.

package tls

// ML-DSA (FIPS 204) support for TLS 1.3, as advertised by Chrome 150+.
//
// This file holds the build-tag-independent glue. The actual signature
// verification and certificate public-key recovery live in mldsa_circl.go
// (default) and are stubbed out in mldsa_off.go behind the `nomldsa` build
// tag, which drops the cloudflare/circl dependency.
//
// Note: ML-DSA is not added to the default advertised signature algorithms.
// Profiles that want to offer it list the codepoints in their own
// ClientHelloSpec. The verify path (handshake_client_tls13.go) accepts ML-DSA
// via isMLDSASignatureScheme so such a handshake can complete.

// isMLDSASignatureScheme reports whether sigAlg is one of the ML-DSA schemes.
func isMLDSASignatureScheme(sigAlg SignatureScheme) bool {
	switch sigAlg {
	case MLDSA44, MLDSA65, MLDSA87:
		return true
	default:
		return false
	}
}
