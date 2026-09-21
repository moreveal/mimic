package browser

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/moreveal/mimic/internal/network"
)

func TestDownloadFilenameUsesAttachmentDispositionAndSafeName(t *testing.T) {
	u, _ := url.Parse("https://example.test/files/fallback.bin")
	for _, test := range []struct {
		disposition, want string
		attachment        bool
	}{
		{`attachment; filename="report.txt"`, "report.txt", true},
		{`attachment; filename="../escape.txt"`, "escape.txt", true},
		{`attachment`, "fallback.bin", true},
		{`inline; filename="inline.txt"`, "", false},
	} {
		got, ok := downloadFilename(network.Response{URL: u, Headers: http.Header{"Content-Disposition": {test.disposition}}})
		if got != test.want || ok != test.attachment {
			t.Fatalf("%q: got %q, %v; want %q, %v", test.disposition, got, ok, test.want, test.attachment)
		}
	}
}
