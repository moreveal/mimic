package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBlitzExternalStylesheetKeepsOwnURLContext(t *testing.T) {
	parallelBrowserTest(t)
	p := blitzStandardsPage(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/styles/theme.css" {
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, `#target{width:20px;height:10px;background-image:url(icon.png)}`)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><link rel="stylesheet" href="/styles/theme.css"><div id="target"></div>`)
	}))
	defer server.Close()
	if err := p.Navigate(context.Background(), server.URL+"/page/start"); err != nil {
		t.Fatal(err)
	}
	want := `url("` + server.URL + `/styles/icon.png")`
	for _, step := range []string{"", `history.pushState(null,'','/other/route');`, `document.styleSheets[0].insertRule('#target{background-image:url(second.png)}',1);`} {
		if step != "" && step[0] == 'd' {
			want = `url("` + server.URL + `/styles/second.png")`
		}
		value, err := p.Evaluate(context.Background(), `(()=>{`+step+`return getComputedStyle(document.getElementById('target')).backgroundImage;})()`)
		if err != nil || value != want {
			t.Fatalf("sheet context step=%q got=%v want=%q err=%v", step, value, want, err)
		}
		assertBlitzOwnerActive(t, p)
	}
}
