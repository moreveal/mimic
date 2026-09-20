package browser

import (
	"context"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBlitzProducerActuallyServesCanonicalDocument(t *testing.T) {
	serialBrowserTest(t)
	t.Setenv("MIMIC_STYLE_ENGINE", "blitz")
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><html><head></head><body></body></html>"))
	}))
	defer server.Close()
	if err := p.Navigate(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(context.Background(), `(()=>{
      document.head.innerHTML='<style>.box{width:123px;height:45px}</style>';
      document.body.innerHTML='<div class="box"></div>';
      const e=document.querySelector('.box');
      return JSON.stringify([getComputedStyle(e).width,e.getBoundingClientRect().width]);
    })()`)
	if err != nil || value != `["123px",123]` {
		t.Fatalf("native observation=%v error=%v", value, err)
	}
	if p.Top.Realm.blitz == nil || p.Top.Realm.blitz.document.Owner == nil {
		t.Fatal("test used fallback instead of Blitz")
	}
	value, err = p.Evaluate(context.Background(), `(()=>{
      const e=document.querySelector('.box'), values=[];
      document.styleSheets[0].insertRule('.box{width:150px}',1);
      values.push(getComputedStyle(e).width,e.getBoundingClientRect().width);
      e.style.width='210px';values.push(e.getBoundingClientRect().width);
      e.style.removeProperty('width');values.push(e.getBoundingClientRect().width);
      const parent=e.parentNode;e.remove();document.body.getBoundingClientRect();parent.append(e);
      values.push(e.getBoundingClientRect().width);
      return JSON.stringify(values);
    })()`)
	if err != nil || value != `["150px",150,210,150,150]` {
		t.Fatalf("native updates=%v error=%v", value, err)
	}
	value, err = p.Evaluate(context.Background(), `(()=>{
      document.head.innerHTML='<style>input{width:100px;border:0;padding:0}input:checked{width:200px}input:focus{height:80px}div:focus-within{padding-left:17px}</style>';
      document.body.innerHTML='<div><input type="checkbox"></div>';
      const e=document.querySelector('input'), values=[];
      values.push(e.getBoundingClientRect().width);
      e.checked=true;values.push(e.getBoundingClientRect().width);
      e.checked=false;values.push(e.getBoundingClientRect().width);
      e.focus();values.push(getComputedStyle(e).height,getComputedStyle(e.parentNode).paddingLeft);
      e.blur();values.push(getComputedStyle(e.parentNode).paddingLeft);
      return JSON.stringify(values);
    })()`)
	if err != nil || value != `[100,200,100,"80px","17px","0px"]` {
		t.Fatalf("native state=%v error=%v", value, err)
	}
	value, err = p.Evaluate(context.Background(), `(()=>{
      document.head.innerHTML='<style>.hidden{display:none;--shade:red}.hidden div{color:var(--shade);font-size:17px}</style>';
      document.body.innerHTML='<section class="hidden"><div>hidden text</div></section>';
      const e=document.querySelector('div'), s=getComputedStyle(e), values=[];
      values.push(s.display,s.fontSize,s.color,e.getBoundingClientRect().width);
      e.parentNode.style.setProperty('--shade','blue');values.push(s.color);
      e.parentNode.className='';values.push(e.getBoundingClientRect().width>0);
      e.parentNode.className='hidden';values.push(s.color,e.getBoundingClientRect().width);
      return JSON.stringify(values);
    })()`)
	if err != nil || value != `["block","17px","rgb(255, 0, 0)",0,"rgb(0, 0, 255)",true,"rgb(0, 0, 255)",0]` {
		t.Fatalf("native hidden style lifecycle=%v error=%v", value, err)
	}
}
