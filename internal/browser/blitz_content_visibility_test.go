package browser

import (
	"context"
	"testing"
)

func TestBlitzContentVisibilityAdmission(t *testing.T) {
	serialBrowserTest(t)
	t.Setenv("MIMIC_STYLE_ENGINE", "blitz")
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		for _, tc := range []struct {
			name, script string
			fallback     bool
		}{
			{"initial", `document.head.innerHTML='';document.body.innerHTML='<div id="target" style="width:20px;height:10px"></div>';`, false},
			{"string-is-not-declaration", `document.head.innerHTML='<style>#target::before{content:"content-visibility:hidden"}#target{--unused:"content-visibility:hidden"}</style>';`, false},
			{"inline", `document.getElementById('target').setAttribute('style','width:20px;content-visibility:hidden');`, true},
			{"inline-remove", `document.getElementById('target').setAttribute('style','width:20px');`, false},
			{"escaped-inline", `document.getElementById('target').setAttribute('style','content-\\76 isibility:hidden');`, true},
			{"sheet", `document.getElementById('target').removeAttribute('style');document.head.innerHTML='<style>#target{content-visibility:hidden}</style>';`, true},
			{"sheet-cssom-remove", `document.styleSheets[0].cssRules[0].style.removeProperty('content-visibility');`, false},
			{"escaped-sheet", `document.head.innerHTML='<style>#target{content-\\76 isibility:hidden}</style>';`, true},
			{"nested-sheet", `document.head.innerHTML='<style>@media screen{#target{content-visibility:hidden}}</style>';`, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				_, err := p.Evaluate(context.Background(), `(()=>{`+tc.script+`document.getElementById('target').getBoundingClientRect();return getComputedStyle(document.getElementById('target')).contentVisibility;})()`)
				if err != nil {
					t.Fatal(err)
				}
				state := p.Top.Realm.blitz
				if state == nil {
					t.Fatal("missing native admission result")
				}
				if (state.fallback != "") != tc.fallback {
					t.Fatalf("fallback=%q want=%v", state.fallback, tc.fallback)
				}
				if !tc.fallback && state.document.Owner == nil {
					t.Fatal("default case failed native admission")
				}
			})
		}
	})
}
