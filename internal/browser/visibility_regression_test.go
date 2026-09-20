package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSSVisibilityFeatureDetectionAndMultilingualFocus(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		historyEval(t, p, `(()=>{
 document.body.innerHTML='<main style="content-visibility:hidden"><input id="entry"><select style="font:12px Arial"><option>العربية</option><option>עברית</option><option>فارسی</option></select></main>';
 const main=document.querySelector('main');
 for(const value of ['visible','auto','hidden','initial','inherit'])if(CSS.supports('content-visibility',value)!==true||!CSS.supports('(content-visibility: '+value+')')||!CSS.supports('content-visibility: '+value))return 'support:'+value;
 if(CSS.supports('content-visibility','invalid')!==false||CSS.supports('unknown-property','x')!==false)return 'invalid';
 main.style.contentVisibility='invalid';if(main.style.contentVisibility!=='hidden')return 'invalid replaced value';
 document.getElementById('entry').focus();
 if(CSS.supports('content-visibility','hidden'))main.style.removeProperty('content-visibility');
 return main.style.contentVisibility===''&&document.activeElement.id==='entry';
 })()`, true)
	})
}

func TestStylesheetCSSOMHonorsCORSMode(t *testing.T) {
	parallelBrowserTest(t)
	assets := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		if r.URL.Path != "/denied.css" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		fmt.Fprint(w, "body { color: red }")
	}))
	defer assets.Close()
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<!doctype html><head><link id="opaque" rel="stylesheet" href="%s/allow.css"><link id="allowed" rel="stylesheet" crossorigin="anonymous" href="%s/allow.css"><link id="denied" rel="stylesheet" crossorigin="anonymous" href="%s/denied.css"></head><body>test`, assets.URL, assets.URL, assets.URL)
	}))
	defer page.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		if err := p.Navigate(context.Background(), page.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `(()=>{
 const a=document.getElementById('allowed').sheet;
 if(!a||a.cssRules.length!==1||a!==document.getElementById('allowed').sheet)return 'allowed';
 try{document.getElementById('opaque').sheet.cssRules;return 'opaque exposed'}catch(e){if(e.name!=='SecurityError')return e.name}
 return document.getElementById('denied').sheet===null;
 })()`, true)
	})
}
