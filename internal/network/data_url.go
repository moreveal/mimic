package network

import (
	"encoding/base64"
	"fmt"
	"mime"
	"net/url"
	"strings"
)

// decodeDataURL implements local resource decoding without involving transport.
func decodeDataURL(u *url.URL) ([]byte, string, error) {
	if len(u.String()) > 44<<20 {
		return nil, "", fmt.Errorf("data URL byte limit")
	}
	copyURL := *u
	copyURL.Fragment, copyURL.RawFragment = "", ""
	metadata, payload, ok := strings.Cut(strings.TrimPrefix(copyURL.String(), "data:"), ",")
	if !ok {
		return nil, "", fmt.Errorf("data URL has no payload separator")
	}
	metadata = strings.TrimSpace(metadata)
	encoded := strings.HasSuffix(strings.ToLower(metadata), ";base64")
	if encoded {
		metadata = strings.TrimSpace(metadata[:len(metadata)-7])
	}
	if strings.HasPrefix(metadata, ";") {
		metadata = "text/plain" + metadata
	}
	contentType := metadata
	if mediaType, _, err := mime.ParseMediaType(metadata); err != nil || !strings.Contains(mediaType, "/") {
		contentType = "text/plain;charset=US-ASCII"
	}
	// URL percent decoding leaves malformed escapes unchanged.
	var decodedBytes []byte
	for i := 0; i < len(payload); i++ {
		if payload[i] == '%' && i+2 < len(payload) {
			if value, e := url.PathUnescape(payload[i : i+3]); e == nil {
				decodedBytes = append(decodedBytes, value...)
				i += 2
				continue
			}
		}
		decodedBytes = append(decodedBytes, payload[i])
	}
	decoded := string(decodedBytes)
	var err error
	if !encoded {
		return []byte(decoded), contentType, nil
	}
	decoded = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
			return -1
		}
		return r
	}, decoded)
	var body []byte
	if len(decoded)%4 == 0 {
		body, err = base64.StdEncoding.DecodeString(decoded)
	} else {
		body, err = base64.RawStdEncoding.DecodeString(decoded)
	}
	return body, contentType, err
}
