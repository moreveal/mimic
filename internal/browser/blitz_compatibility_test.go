package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func blitzStandardsPage(t *testing.T) *Page {
	t.Helper()
	t.Setenv("MIMIC_STYLE_ENGINE", "blitz")
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	t.Cleanup(func() { c.Close() })
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><html><head></head><body></body></html>"))
	}))
	t.Cleanup(s.Close)
	if err := p.Navigate(context.Background(), s.URL); err != nil {
		t.Fatal(err)
	}
	return p
}

func assertBlitzOwnerActive(t *testing.T, p *Page) {
	t.Helper()
	if p.Top.Realm.blitz == nil || p.Top.Realm.blitz.document.Owner == nil || p.Top.Realm.blitz.fallback != "" {
		t.Fatal("native compatibility test used fallback")
	}
}

func TestBlitzCompatibilityIsolatedMutationAndInheritance(t *testing.T) {
	p := blitzStandardsPage(t)
	d := NewDebugger(p)
	defer d.Close()
	debuggerEval(t, d, `document.head.innerHTML='<style>.parent{--size:73px;color:rgb(1,2,3)}.parent > .child{width:var(--size);height:29px}</style>';document.body.innerHTML='<div class="parent"><div class="child" id="probe"></div></div>'`, DebuggerOptions{})
	w, err := p.IsolatedWorld(context.Background(), p.Top.ID, "native-compatibility")
	if err != nil {
		t.Fatal(err)
	}
	read := func(want string) {
		t.Helper()
		r, e := d.Evaluate(context.Background(), p.Top.ID, w, `(()=>{const e=document.getElementById('probe');return JSON.stringify([getComputedStyle(e).width,getComputedStyle(e).color,e.getBoundingClientRect().width,e.checkVisibility()])})()`, DebuggerOptions{ReturnByValue: true})
		if e != nil || r["exceptionDetails"] != nil || r["result"].(map[string]any)["value"] != want {
			t.Fatalf("observation=%#v error=%v want=%s", r, e, want)
		}
		assertBlitzOwnerActive(t, p)
	}
	read(`["73px","rgb(1, 2, 3)",73,true]`)
	debuggerEval(t, d, `document.querySelector('.parent').style.setProperty('--size','91px')`, DebuggerOptions{})
	read(`["91px","rgb(1, 2, 3)",91,true]`)
	debuggerEval(t, d, `document.querySelector('.parent').style.display='none'`, DebuggerOptions{})
	read(`["91px","rgb(1, 2, 3)",0,false]`)
}

func TestBlitzCompatibilityFlexGridAndScrollGeometry(t *testing.T) {
	p := blitzStandardsPage(t)
	v, err := p.Evaluate(context.Background(), `(()=>{
document.body.style.margin='0';document.body.innerHTML='<div style="display:flex;width:300px;height:40px"><div id="a" style="flex:1"></div><div id="b" style="flex:2"></div></div><div style="display:grid;width:300px;grid-template-columns:100px 200px"><div id="c" style="height:20px"></div><div id="d"></div></div><div id="scroller" style="width:100px;height:50px;overflow:auto"><div id="inside" style="height:200px;width:100px"></div></div>';
const rect=id=>document.getElementById(id).getBoundingClientRect();const s=document.getElementById('scroller');const before=rect('inside').top;s.scrollTop=30;
return JSON.stringify([rect('a').width,rect('b').width,rect('c').width,rect('d').width,s.clientHeight,s.scrollHeight,s.scrollTop,before-rect('inside').top]);})()`)
	if err != nil || v != `[100,200,100,200,50,200,30,30]` {
		t.Fatalf("geometry=%v error=%v", v, err)
	}
	assertBlitzOwnerActive(t, p)
}

func TestBlitzCompatibilityIntersectionObserver(t *testing.T) {
	p := blitzStandardsPage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	v, err := p.Evaluate(ctx, intersectionObserverRegressions)
	if err != nil || v != "ok" {
		t.Fatalf("native intersection regression: %v error=%v", v, err)
	}
	assertBlitzOwnerActive(t, p)
}

func TestBlitzCompatibilityIsolatedHitTesting(t *testing.T) {
	p := blitzStandardsPage(t)
	d := NewDebugger(p)
	defer d.Close()
	debuggerEval(t, d, inputStackingSetup+`Document.prototype.elementsFromPoint=()=>{throw Error('author override')}`, DebuggerOptions{})
	w, err := p.IsolatedWorld(context.Background(), p.Top.ID, "native-hit-testing")
	if err != nil {
		t.Fatal(err)
	}
	r, err := d.Evaluate(context.Background(), p.Top.ID, w, `(()=>{
const target=document.getElementById('target'),cover=document.getElementById('cover');
target.getBoundingClientRect();const first=document.elementFromPoint(30,30)===target;
cover.style.zIndex='4';const changed=document.elementsFromPoint(30,30)[0]===cover;
cover.style.pointerEvents='none';const uncovered=document.elementFromPoint(30,30)===target;
return first&&changed&&uncovered&&document.elementFromPoint(NaN,30)===null&&document.elementsFromPoint(-1,30).length===0;
})()`, DebuggerOptions{ReturnByValue: true})
	if err != nil || r["exceptionDetails"] != nil || r["result"].(map[string]any)["value"] != true {
		t.Fatalf("native hit test=%#v error=%v", r, err)
	}
	assertBlitzOwnerActive(t, p)
}

func TestBlitzCompatibilityContainingBlocks(t *testing.T) {
	p := blitzStandardsPage(t)
	v, err := p.Evaluate(context.Background(), `(()=>{
document.body.style.cssText='margin:8px;height:3000px';
document.body.innerHTML='<div id="outer" style="width:200px;height:100px;border:3px solid;padding:5px"><div id="middle" style="margin:7px"><div id="absolute" style="position:absolute;left:13px;top:17px;width:80px;height:20px"></div></div></div><div id="fixed" style="position:fixed;right:12px;bottom:14px;width:20px;height:30px"></div>';
const a=document.getElementById('absolute'),outer=document.getElementById('outer'),f=document.getElementById('fixed');
const xy=e=>{const r=e.getBoundingClientRect();return [r.x,r.y]},out=[];
out.push(xy(a));outer.style.position='relative';out.push(xy(a));outer.style.position='';out.push(xy(a));
const r=f.getBoundingClientRect();out.push(r.right===innerWidth-12&&r.bottom===innerHeight-14);
scrollTo(0,400);const s=f.getBoundingClientRect();out.push(s.x===r.x&&s.y===r.y);
outer.style.position='relative';document.getElementById('middle').append(f);out.push(xy(f)[1]===r.y);
scrollTo(0,0);outer.style.transform='translateX(0px)';out.push(xy(f));
scrollTo(0,40);out.push(xy(f));scrollTo(0,0);
a.style.position='static';out.push(xy(a));
return JSON.stringify(out);
})()`)
	if err != nil || v != `[[13,17],[24,28],[13,17],true,true,true,[189,77],[189,37],[23,23]]` {
		t.Fatalf("containing blocks=%v error=%v", v, err)
	}
	assertBlitzOwnerActive(t, p)
}
