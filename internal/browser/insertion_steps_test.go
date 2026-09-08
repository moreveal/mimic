package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestFrameInsertionAfterInnerHTMLAndFragmentMove(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body></body>")) }))
	defer server.Close()
	for name, factory := range map[string]engine.Factory{"goja": gojaengine.Factory{}, "v8": v8engine.Factory{}} {
		t.Run(name, func(t *testing.T) {
			b, err := New(factory, chrome152.New())
			if err != nil {
				t.Fatal(err)
			}
			c := b.NewContext()
			defer c.Close()
			p, err := c.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := p.Navigate(ctx, server.URL); err != nil {
				t.Fatal(err)
			}
			v, err := p.Evaluate(ctx, `new Promise(resolve=>{
 const parent=document.createElement('div');parent.innerHTML='<section><iframe src="/child"></iframe></section>';
 const iframe=parent.querySelector('iframe'),fragment=document.createDocumentFragment();
 fragment.appendChild(parent);
 if(iframe.isConnected)throw Error('detached fragment connected');
 iframe.addEventListener('load',()=>resolve(iframe.isConnected&&iframe.contentWindow!==null&&parent.parentNode===document.body&&fragment.childNodes.length===0));
 document.body.appendChild(fragment);
})`)
			if err != nil || v != true {
				t.Fatalf("frame insertion: %v %v", v, err)
			}
		})
	}
}
