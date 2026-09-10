package profiles

import (
	quic "github.com/bogdanfinn/quic-go-utls"
	tls "github.com/bogdanfinn/utls"
	"time"
)

// chrome152QUICSpec is the TLS 1.3-only QUIC ClientHello observed on the pinned
// Windows Chrome. QUIC does not reuse the TCP signature list or GREASE slots.
// The live QUIC connection replaces the transport parameter extension payload.
func chrome152QUICSpec() (tls.ClientHelloSpec, error) {
	return tls.ClientHelloSpec{
		TLSVersMin: tls.VersionTLS13, TLSVersMax: tls.VersionTLS13,
		CipherSuites:       []uint16{tls.TLS_AES_128_GCM_SHA256, tls.TLS_AES_256_GCM_SHA384, tls.TLS_CHACHA20_POLY1305_SHA256},
		CompressionMethods: []byte{0},
		Extensions: tls.ShuffleChromeTLSExtensions([]tls.TLSExtension{
			&tls.SNIExtension{},
			&tls.SupportedCurvesExtension{Curves: []tls.CurveID{tls.X25519MLKEM768, tls.X25519, tls.CurveP256, tls.CurveP384}},
			&tls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []tls.SignatureScheme{tls.ECDSAWithP256AndSHA256, tls.PSSWithSHA256, tls.PKCS1WithSHA256, tls.ECDSAWithP384AndSHA384, tls.PSSWithSHA384, tls.PKCS1WithSHA384, tls.PSSWithSHA512, tls.PKCS1WithSHA512, tls.PKCS1WithSHA1}},
			&tls.ALPNExtension{AlpnProtocols: []string{"h3"}},
			&tls.UtlsCompressCertExtension{Algorithms: []tls.CertCompressionAlgo{tls.CertCompressionBrotli}},
			&tls.SupportedVersionsExtension{Versions: []uint16{tls.VersionTLS13}},
			&tls.PSKKeyExchangeModesExtension{Modes: []uint8{tls.PskModeDHE}},
			&tls.KeyShareExtension{KeyShares: []tls.KeyShare{{Group: tls.X25519MLKEM768}, {Group: tls.X25519}}},
			&tls.QUICTransportParametersExtension{},
			&tls.ApplicationSettingsExtensionNew{SupportedProtocols: []string{"h3"}},
			&tls.GenericExtension{Id: 0x12e0, Data: []byte{0, 0}},
			&tls.GenericExtension{Id: 0xca34, Data: chrome152WindowsTrustAnchors},
			tls.BoringGREASEECH(),
			&tls.UtlsPreSharedKeyExtension{},
		}),
	}, nil
}

func chrome152QUICConfig() *quic.Config {
	return &quic.Config{
		ClientHelloSpec: chrome152QUICSpec, EnableDatagrams: true,
		Versions: []quic.Version{quic.Version1}, InitialPacketSize: 1250,
		MaxIdleTimeout:             300 * time.Second,
		InitialStreamReceiveWindow: 6291456, MaxStreamReceiveWindow: 6291456,
		InitialConnectionReceiveWindow: 15728640, MaxConnectionReceiveWindow: 15728640,
		MaxIncomingStreams: 100, MaxIncomingUniStreams: 103,
		ClientTransportParameters: &quic.ClientTransportParameters{
			InitialRTTCache:       quic.NewClientRTTCache(100),
			InitialPacketSizeIPv6: 1230,
			MaxUDPPayloadSize:     1472, MaxDatagramFrameSize: 65536, MaxAckDelay: 25 * time.Millisecond,
			ActiveConnectionIDLimit: 2, AdvertiseVersionInformation: true, RandomizeOrder: true,
			Additional: map[uint64][]byte{0x3128: []byte("ORIGNOIP")},
		},
	}
}
