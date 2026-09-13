package browser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/moreveal/mimic/internal/network"
)

// Resource origin cleanliness depends on the fetch mode and CORS response,
// not merely the final URL. Images and stylesheet CSSOM share this decision.
func resourceResponseOrigin(request network.Request, response network.Response) (bool, error) {
	finalURL := response.URL
	if finalURL == nil {
		finalURL = request.URL
	}
	originURL := finalURL
	if finalURL.Scheme == "blob" {
		if parsed, err := url.Parse(strings.TrimPrefix(finalURL.String(), "blob:")); err == nil {
			originURL = parsed
		}
	}
	clean := finalURL.Scheme == "data" || originURL.Scheme == request.SourceURL.Scheme && originURL.Host == request.SourceURL.Host
	if request.Mode == "cors" && !clean {
		allow := response.Headers.Get("Access-Control-Allow-Origin")
		clean = allow == request.Headers.Get("Origin") || allow == "*" && request.Credentials != "include"
		if request.Credentials == "include" {
			clean = clean && response.Headers.Get("Access-Control-Allow-Credentials") == "true"
		}
		if !clean {
			return false, fmt.Errorf("resource CORS response disallowed")
		}
	}
	return clean, nil
}
