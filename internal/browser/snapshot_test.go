package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/network"
)

func TestSnapshotEmptyAssetEncodesAsBase64String(t *testing.T) {
	base, _ := url.Parse("https://example.test/")
	for _, status := range []int{http.StatusOK, http.StatusNoContent} {
		b := &snapshotBuilder{
			out:  Snapshot{Files: map[string][]byte{}},
			seen: map[string]string{},
			prefetched: map[string]snapshotAssetLoad{
				"https://example.test/empty.png": {response: network.Response{Status: status}},
			},
		}
		name := b.asset("/empty.png", base, false)
		encoded, err := json.Marshal(b.out)
		if err != nil {
			t.Fatal(err)
		}
		var wire struct {
			Files map[string]any `json:"files"`
		}
		if err := json.Unmarshal(encoded, &wire); err != nil {
			t.Fatal(err)
		}
		if value, ok := wire.Files[name]; name == "" || !ok || value != "" {
			t.Fatalf("status %d: empty asset must be a base64 string, got %s", status, encoded)
		}
	}
}

func TestSnapshotPreservesCaseInsensitiveDataURL(t *testing.T) {
	b := &snapshotBuilder{}
	base, _ := url.Parse("https://example.test/")
	data := "DATA:image/png;base64,iVBORw0KGgo="
	if got := b.asset(data, base, false); got != data {
		t.Fatalf("data URL rewritten: %q", got)
	}
}

func TestSnapshotPrefetchesAssetsConcurrently(t *testing.T) {
	var active atomic.Int32
	var peak atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		current := active.Add(1)
		defer active.Add(-1)
		for observed := peak.Load(); current > observed && !peak.CompareAndSwap(observed, current); observed = peak.Load() {
		}
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte{0, 1, 2, 3})
	}))
	defer ts.Close()

	p := testPage(t)
	defer p.Close()
	base, _ := url.Parse(ts.URL + "/")
	b := &snapshotBuilder{
		page:       p,
		ctx:        context.Background(),
		prefetched: map[string]snapshotAssetLoad{},
	}
	specs := make([]snapshotAssetSpec, 8)
	for index := range specs {
		resource, _ := url.Parse(fmt.Sprintf("%s/asset-%d.png", ts.URL, index))
		specs[index] = snapshotAssetSpec{key: resource.String(), url: resource, base: base}
	}
	b.loadAssetBatch(context.Background(), specs)
	if peak.Load() < 2 {
		t.Fatalf("asset prefetch was serial: peak concurrency %d", peak.Load())
	}
	if len(b.prefetched) != len(specs) {
		t.Fatalf("prefetched %d assets, want %d", len(b.prefetched), len(specs))
	}
}

func TestSnapshotPreservesSelfContainedCSSURLQuoting(t *testing.T) {
	base, _ := url.Parse("https://example.test/styles/main.css")
	b := &snapshotBuilder{}
	for _, source := range []string{
		`.icon { mask-image: url('data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0h20v20z"/></svg>'); }`,
		`.icon { background: url("data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg'/>"); }`,
		`.icon { mask: url('#local-mask'); }`,
	} {
		for _, external := range []bool{false, true} {
			if got := b.rewriteCSS(source, base, external, ""); got != source {
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

func TestSnapshotCapturesNestedFramesAsPortableDocuments(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/child":
			fmt.Fprint(w, `<html><body><p id="child">child state</p><img src="data:image/png;base64,AA=="></body></html>`)
		case "/grandchild":
			fmt.Fprint(w, `<html><body><p id="grandchild">grandchild state</p></body></html>`)
		default:
			fmt.Fprint(w, `<html><body><h1>top</h1><iframe src="/child"></iframe></body></html>`)
		}
	}))
	defer ts.Close()
	p := testPage(t)
	defer p.Close()
	if err := p.Navigate(context.Background(), ts.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Evaluate(context.Background(), `new Promise(resolve=>{const childDocument=document.querySelector('iframe').contentDocument,frame=childDocument.createElement('iframe');frame.onload=()=>{frame.contentDocument.body.innerHTML='<p id="grandchild">grandchild state</p>';resolve()};frame.src='/grandchild';childDocument.body.append(frame)})`); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		children := p.Top.Children()
		if len(children) == 1 && len(children[0].Children()) == 1 && children[0].Children()[0].ReadyState() == "complete" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	snapshot, err := p.CaptureSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	index := string(snapshot.Files["index.html"])
	child := string(snapshot.Files["frames/frame-1.html"])
	grandchild := string(snapshot.Files["frames/frame-2.html"])
	if !strings.Contains(index, `src="frames/frame-1.html"`) || !strings.Contains(child, `id="child">child state`) || !strings.Contains(child, `src="data:image/png;base64,AA=="`) || strings.Contains(child, `../data:`) || !strings.Contains(child, `src="frame-2.html"`) || !strings.Contains(grandchild, `id="grandchild">grandchild state`) {
		t.Fatalf("nested frame snapshot was not portable: files=%v index=%s child=%s grandchild=%s", len(snapshot.Files), index, child, grandchild)
	}
	if len(snapshot.Warnings) != 0 {
		t.Fatal(snapshot.Warnings)
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
	if len(snapshot.Files) != 3 || len(snapshot.Warnings) != 1 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if !strings.Contains(string(snapshot.Files["index.html"]), `src="frames/frame-1.html"`) || snapshot.Files["frames/frame-1.html"] == nil {
		t.Fatal("embedded frame was not exported")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.CaptureSnapshot(ctx); err == nil {
		t.Fatal("cancelled export succeeded")
	}
}
