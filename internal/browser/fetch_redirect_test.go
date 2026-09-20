package browser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestFetchRedirectResponseSemantics(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			w.Header().Set("Location", "/target")
			w.WriteHeader(301)
		case "/target":
			body, _ := io.ReadAll(r.Body)
			fmt.Fprint(w, r.Method+"|"+string(body)+"|"+r.Header.Get("Content-Type"))
		default:
			fmt.Fprint(w, "<!doctype html><title>redirect fixture</title>")
		}
	}))
	defer server.Close()
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	bc := b.NewContext()
	defer bc.Close()
	p, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	got, err := p.Evaluate(ctx, `(async()=>{
const followed=await fetch('/redirect',{method:'POST',body:'payload'});
if(!followed.redirected||new URL(followed.url).pathname!=='/target'||await followed.text()!=='GET||')return 'follow';
const manual=await fetch('/redirect',{redirect:'manual'});
if(manual.type!=='opaqueredirect'||manual.status!==0||manual.ok||manual.redirected||manual.body!==null||[...manual.headers].length||new URL(manual.url).pathname!=='/redirect'||await manual.text()!==''||manual.bodyUsed)return 'manual';
try{await fetch('/redirect',{redirect:'error'});return 'error resolved'}catch(e){if(!(e instanceof TypeError))return 'error type'}
return true;
})()`)
	if err != nil || got != true {
		t.Fatalf("redirect response = %#v, %v", got, err)
	}
}
