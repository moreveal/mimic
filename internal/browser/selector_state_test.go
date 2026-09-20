package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSelectorStateUsesNavigationAndElementState(t *testing.T) {
	parallelBrowserTest(t)
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><div id="a"></div><div id="b"></div><a name="legacy"></a><input id="field"><x-state id="custom"></x-state>`))
	}))
	defer fixture.Close()
	p := testPage(t)
	ctx := context.Background()
	if err := p.Navigate(ctx, fixture.URL+"/#a"); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `(()=>{
 const original=document.getElementById('a');if(document.querySelector(':target')!==original)return 'initial';
 original.id='changed';history.replaceState({},'','#b');if(document.querySelector(':target')!==original)return 'identity/history';
 location.hash='#legacy';if(document.querySelector(':target').getAttribute('name')!=='legacy')return 'legacy anchor';
 const input=document.getElementById('field');if(input.matches(':focus'))return 'initial focus';input.focus();
 if(!input.matches(':focus')||!input.matches(':focus-visible')||!document.body.matches(':focus-within'))return 'focus';input.blur();if(document.querySelector(':focus'))return 'blur';
 const custom=document.getElementById('custom');if(custom.matches(':defined'))return 'undefined';
 customElements.define('x-state',class extends HTMLElement{});return custom.matches(':defined')&&input.matches(':defined');
})()`)
	if err != nil || value != true {
		t.Fatalf("selector state: %v %v", value, err)
	}
}
