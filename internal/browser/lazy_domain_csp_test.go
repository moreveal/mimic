package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestLazyWebAPIDomainsIgnoreAuthorEvalCSP(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Security-Policy", "script-src 'self' 'unsafe-inline'; require-trusted-types-for 'script'; trusted-types default")
		_, _ = w.Write([]byte(`<body>lazy domain CSP</body>`))
	}))
	defer server.Close()
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
	ctx := context.Background()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `(()=>{
  trustedTypes.createPolicy('default',{createScript:source=>source==='this'?source:''});
  const result={};
  try{eval('1+1');result.authorEval='allowed'}catch(error){result.authorEval=error.name}
  try{eval(trustedTypes.defaultPolicy.createScript('this'));result.trustedAuthorEval='allowed'}
  catch(error){result.trustedAuthorEval=error.name}
  const originalEval=globalThis.eval;
  let leaked=false;
  globalThis.eval=()=>{leaked=true};
  try{navigator.gpu;result.overrideError='allowed'}catch(error){result.overrideError=error.name}
  globalThis.eval=originalEval;
  result.overrideLeak=leaked;
  for(const [name,probe] of [
    ['webgpu',()=>navigator.gpu],
    ['webgl',()=>new OffscreenCanvas(2,2).getContext('webgl')],
    ['audio',()=>new OfflineAudioContext(1,128,44100)]
  ]){
    try{result[name]=probe()==null?'null':'loaded'}
    catch(error){result[name]=error.name+':'+error.message}
  }
  try{eval('2+2');result.authorEvalAfter='allowed'}
  catch(error){result.authorEvalAfter=error.name}
  return result;
})()`)
	if err != nil {
		t.Fatal(err)
	}
	got := value.(map[string]any)
	if got["authorEval"] != "EvalError" || got["authorEvalAfter"] != "EvalError" || got["trustedAuthorEval"] != "EvalError" || got["overrideError"] != "TypeError" || got["overrideLeak"] != false || got["webgpu"] != "loaded" || got["webgl"] != "loaded" || got["audio"] != "loaded" {
		t.Fatalf("lazy domains under author CSP = %#v", got)
	}
}
