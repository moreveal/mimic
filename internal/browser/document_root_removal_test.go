package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoveDocumentRootComment(t *testing.T) {
	parallelBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><body>ready</body></html><!--Blazor-Server-Component-State:abc-->`))
	}))
	defer server.Close()
	p := testPage(t)
	if err := p.Navigate(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	got, err := p.Evaluate(context.Background(), `(() => {
		const comment = Array.from(document.childNodes).find(node => node.nodeType === Node.COMMENT_NODE);
		return !!comment && document.removeChild(comment) === comment && comment.parentNode === null &&
			!Array.from(document.childNodes).includes(comment) && !!document.documentElement;
	})()`)
	if err != nil {
		t.Fatal(err)
	}
	if got != true {
		t.Fatalf("document root comment removal = %v, want true", got)
	}
}
