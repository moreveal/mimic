package browser

import (
	"context"
	"testing"
)

func TestBlitzLiveSearchValuePreservesDefaultAttribute(t *testing.T) {
	serialBrowserTest(t)
	t.Setenv("MIMIC_STYLE_ENGINE", "blitz")
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		scripts := []string{
			`document.body.innerHTML='<style>input[value="default"]{width:123px;box-sizing:border-box}</style><form><input id="search" type="search" value="default"></form>';`,
			`document.getElementById('search').value='Wikipedia JavaScript';`,
			`document.getElementById('search').setAttribute('value','default');document.body.setAttribute('data-refresh','1');`,
			`document.getElementById('search').value='';`,
			`document.querySelector('form').reset();`,
		}
		for i, script := range scripts {
			result, err := p.Evaluate(context.Background(), `(()=>{`+script+`const e=document.getElementById('search');return JSON.stringify([e.getAttribute('value'),e.getBoundingClientRect().width,e.value]);})()`)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{`["default",123,"default"]`, `["default",123,"Wikipedia JavaScript"]`, `["default",123,"Wikipedia JavaScript"]`, `["default",123,""]`, `["default",123,"default"]`}[i]
			if result != want {
				t.Fatalf("step%d result%v want%s", i, result, want)
			}
			state := p.Top.Realm.blitz
			if state == nil || state.fallback != "" || state.document.Owner == nil {
				t.Fatalf("step%d not native: %+v", i, state)
			}
		}
		result, err := p.Evaluate(context.Background(), `(()=>{const e=document.getElementById('search');e.type='text';e.removeAttribute('value');e.style.cssText='box-sizing:border-box;width:60px;font:16px "Times New Roman";padding:2px;border:1px solid';e.value='';const empty=e.scrollWidth;e.value='MMMMMMMMMMMMMMMMMMMM';const full=e.scrollWidth;e.value='';return JSON.stringify([empty,full,e.scrollWidth,e.clientWidth]);})()`)
		if err != nil {
			t.Fatal(err)
		}
		// Exact frozen Chrome 152 receipt for this source at 1272x653.
		if result != `[58,289,58,58]` {
			t.Fatalf("live text extents %v", result)
		}
		info, _ := p.Evaluate(context.Background(), `(()=>{const e=document.getElementById('search'),s=getComputedStyle(e);return [e.style.cssText,s.fontFamily,s.fontSize,s.boxSizing,e.outerHTML]})()`)
		t.Logf("live input style %v", info)
	})
}
