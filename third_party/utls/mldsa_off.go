// Copyright 2026 uTLS. Licensed under the BSD 3-Clause License.

//go:build nomldsa

package tls

import (
	"crypto"
	"crypto/x509"
	"errors"
)

// This file is compiled with the `nomldsa` build tag. It provides ML-DSA-free
// stubs so the module has no cloudflare/circl dependency. The ML-DSA
// SignatureScheme constants still exist (common.go), so a parrot can still
// advertise the codepoints on the wire; only the receive/verify path is
// disabled.

// verifyMLDSAHandshakeSignature always fails when ML-DSA support is disabled.
func verifyMLDSAHandshakeSignature(pubkey crypto.PublicKey, signed, sig []byte) error {
	return errors.New("tls: ML-DSA support is disabled by the nomldsa build tag")
}

// parseCertificate is plain x509.ParseCertificate when ML-DSA is disabled.
func parseCertificate(der []byte) (*x509.Certificate, error) {
	return x509.ParseCertificate(der)
}
