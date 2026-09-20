package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNavigationCommitsRedirectTarget(t *testing.T) {
	parallelBrowserTest(t)
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			scriptReferrers := make(chan string, 1)
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/destination/page":
					fmt.Fprint(w, `<title>Final document</title><script src="relative.js"></script>`)
				case "/destination/relative.js":
					scriptReferrers <- r.Referer()
					fmt.Fprint(w, `globalThis.scriptURL=document.currentScript.src;globalThis.committedURL=location.href;globalThis.committedOrigin=location.origin;history.replaceState(null,'','next')`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer target.Close()
			finalURL := target.URL + "/destination/page?arrived=1"
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, finalURL, status)
			}))
			defer source.Close()
			p := testPage(t)
			if err := p.Navigate(context.Background(), source.URL+"/start"); err != nil {
				t.Fatal(err)
			}
			value, err := p.Evaluate(context.Background(), `({committedURL:globalThis.committedURL,committedOrigin:globalThis.committedOrigin,scriptURL:globalThis.scriptURL,url:location.href,documentURL:document.URL})`)
			if err != nil {
				t.Fatal(err)
			}
			got := value.(map[string]any)
			want := map[string]string{"committedURL": finalURL, "committedOrigin": target.URL, "scriptURL": target.URL + "/destination/relative.js", "url": target.URL + "/destination/next", "documentURL": target.URL + "/destination/next"}
			for key, expected := range want {
				if got[key] != expected {
					t.Errorf("%s = %v, want %s", key, got[key], expected)
				}
			}
			var scriptReferrer string
			select {
			case scriptReferrer = <-scriptReferrers:
			default:
			}
			if scriptReferrer != finalURL {
				t.Errorf("script Referer = %q, want %q", scriptReferrer, finalURL)
			}
			if p.URL() != want["url"] || p.Top.Realm.origin != target.URL || p.history[len(p.history)-1].String() != want["url"] {
				t.Errorf("redirect target was not committed to canonical URL, origin and history")
			}
		})
	}
}
