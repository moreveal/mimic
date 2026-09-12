package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBaseResolutionRevalidatesHistoryDOMAndInheritedBase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<!doctype html><body></body>") }))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		if err := p.Navigate(context.Background(), server.URL+"/one/page"); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `(()=>{
 const seen=[],read=()=>seen.push(new URL(document.baseURI).pathname);
 read();history.replaceState(null,'','/two/page');read();
 const base=document.createElement('base');base.setAttribute('href','/relative/');document.head.append(base);read();
 history.replaceState(null,'','/three/page');read();
 base.setAttribute('href','/absolute/');read();base.remove();read();
 const frame=document.createElement('iframe');document.body.append(frame);
 const child=frame.contentDocument;seen.push(new URL(child.baseURI).pathname);
 
 child.open();child.write('<base href="/written/"><body>x</body>');child.close();seen.push(new URL(child.baseURI).pathname);
 return seen.join(',');
})()`, "/one/page,/two/page,/relative/,/relative/,/absolute/,/three/page,/three/page,/written/")
	})
}
