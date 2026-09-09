package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSnapshotPreservesSelfContainedCSSURLQuoting(t *testing.T) {
	base, _ := url.Parse("https://example.test/styles/main.css")
	b := &snapshotBuilder{}
	for _, source := range []string{
		`.icon { mask-image: url('data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0h20v20z"/></svg>'); }`,
		`.icon { background: url("data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg'/>"); }`,
		`.icon { mask: url('#local-mask'); }`,
	} {
		for _, external := range []bool{false, true} {
			if got := b.rewriteCSS(source, base, external); got != source {
				t.Fatalf("self-contained CSS changed: %s", got)
			}
		}
	}
}

func TestSnapshotUsesResponseMediaTypeForDynamicImages(t *testing.T) {
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20"><path d="M0 0h20v20H0z"/></svg>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/image.php" {
			w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
			fmt.Fprint(w, svg)
			return
		}
		fmt.Fprint(w, `<style>.icon { mask-image: url('/image.php?name=menu'); }</style><span class="icon"></span>`)
	}))
	defer ts.Close()
	p := testPage(t)
	defer p.Close()
	if err := p.Navigate(context.Background(), ts.URL); err != nil {
		t.Fatal(err)
	}
	snapshot, err := p.CaptureSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range snapshot.Files {
		if strings.HasSuffix(name, ".svg") && string(body) == svg && strings.Contains(string(snapshot.Files["index.html"]), name) {
			return
		}
	}
	t.Fatalf("dynamic SVG did not retain its media type: %v", snapshot.Files)
}

func TestSnapshotPortableAssetsAndCurrentDOM(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/css/main.css":
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, `@import "nested.css"; body { background: url('../pixel.png'); }`)
		case "/css/nested.css":
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, `p { color: red }`)
		case "/pixel.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte{0, 1, 2, 255})
		default:
			fmt.Fprint(w, `<!doctype html><html><head><link rel="stylesheet" href="/css/main.css"></head><body><p id="state">before</p><img src="/pixel.png"><script>document.getElementById('state').textContent='after'</script></body></html>`)
		}
	}))
	defer ts.Close()
	p := testPage(t)
	defer p.Close()
	if err := p.Navigate(context.Background(), ts.URL); err != nil {
		t.Fatal(err)
	}
	snapshot, err := p.CaptureSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	source := string(snapshot.Files["index.html"])
	if !strings.Contains(source, ">after</p>") || strings.Contains(source, "<script") || !strings.Contains(source, `src="assets/`) {
		t.Fatalf("bad snapshot: %s", source)
	}
	if len(snapshot.Warnings) != 0 {
		t.Fatal(snapshot.Warnings)
	}
	if len(snapshot.Files) != 4 {
		t.Fatalf("files: %v", snapshot.Files)
	}
	for name, body := range snapshot.Files {
		if strings.HasSuffix(name, ".css") && (strings.Contains(string(body), "../pixel") || strings.Contains(string(body), "assets/")) {
			t.Fatalf("nonportable CSS: %s", body)
		}
	}
	d, _ := p.Document()
	original, _ := d.InnerHTML(d.Root().ID)
	if !strings.Contains(original, "<script") {
		t.Fatal("snapshot mutated live document")
	}
}

func TestSnapshotMissingResourceAndCSSCycle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/loop.css":
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, `@import "loop.css";`)
		case "/missing.png":
			http.NotFound(w, r)
		default:
			fmt.Fprint(w, `<link rel="stylesheet" href="/loop.css"><img src="/missing.png"><iframe src="about:blank"></iframe>`)
		}
	}))
	defer ts.Close()
	p := testPage(t)
	defer p.Close()
	if err := p.Navigate(context.Background(), ts.URL); err != nil {
		t.Fatal(err)
	}
	snapshot, err := p.CaptureSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Files) != 2 || len(snapshot.Warnings) != 2 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if strings.Contains(string(snapshot.Files["index.html"]), "<iframe") {
		t.Fatal("live frame retained")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.CaptureSnapshot(ctx); err == nil {
		t.Fatal("cancelled export succeeded")
	}
}
