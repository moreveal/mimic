// Copyright 2026 uTLS. Licensed under the BSD 3-Clause License.

//go:build !nomldsa

package tls

import (
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"

	circlpki "github.com/cloudflare/circl/pki"
	circlsign "github.com/cloudflare/circl/sign"
	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
)

// This is the default build (no `nomldsa` tag): ML-DSA is backed by
// cloudflare/circl. Build with -tags nomldsa to drop the dependency; see
// mldsa_off.go.

// mldsaSchemeForPublicKey maps a CIRCL ML-DSA public key to its TLS
// SignatureScheme. The second return value is false for any non-ML-DSA key.
func mldsaSchemeForPublicKey(pub crypto.PublicKey) (SignatureScheme, bool) {
	cp, ok := pub.(circlsign.PublicKey)
	if !ok {
		return 0, false
	}
	// CIRCL scheme names are not compile-time constants, so use a tagless
	// switch rather than case labels.
	switch name := cp.Scheme().Name(); {
	case name == mldsa44.Scheme().Name():
		return MLDSA44, true
	case name == mldsa65.Scheme().Name():
		return MLDSA65, true
	case name == mldsa87.Scheme().Name():
		return MLDSA87, true
	default:
		return 0, false
	}
}

// verifyMLDSAHandshakeSignature verifies a TLS 1.3 CertificateVerify signature
// produced with an ML-DSA key. ML-DSA signs the message directly (no pre-hash),
// so `signed` is the full signedMessage and opts is nil.
func verifyMLDSAHandshakeSignature(pubkey crypto.PublicKey, signed, sig []byte) error {
	pub, ok := pubkey.(circlsign.PublicKey)
	if !ok {
		return fmt.Errorf("tls: expected an ML-DSA public key, got %T", pubkey)
	}
	if _, ok := mldsaSchemeForPublicKey(pub); !ok {
		return fmt.Errorf("tls: unsupported ML-DSA public key scheme %q", pub.Scheme().Name())
	}
	if !pub.Scheme().Verify(pub, signed, sig, nil) {
		return errors.New("tls: invalid ML-DSA signature by the server certificate")
	}
	return nil
}

// parseCertificate parses a DER certificate, additionally recovering an ML-DSA
// public key that Go's crypto/x509 leaves unparsed (it does not yet know the
// ML-DSA SubjectPublicKeyInfo OIDs, so it returns the certificate with a nil
// PublicKey). We re-parse the raw SPKI through CIRCL and, if it is an ML-DSA
// key, attach it so signature verification can use it.
func parseCertificate(der []byte) (*x509.Certificate, error) {
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	if cert.PublicKey == nil && len(cert.RawSubjectPublicKeyInfo) > 0 {
		if pub, err := circlpki.UnmarshalPKIXPublicKey(cert.RawSubjectPublicKeyInfo); err == nil {
			if _, ok := mldsaSchemeForPublicKey(pub); ok {
				cert.PublicKey = pub
			}
		}
	}
	return cert, nil
}
