package browser

import (
	"context"
	"testing"
)

func TestBlitzCanonicalColorSchemeInvalidatesNativeStyles(t *testing.T) {
	p := blitzStandardsPage(t)
	_, err := p.Evaluate(context.Background(), `document.head.innerHTML='<style>div{width:10px;height:5px}@media(prefers-color-scheme:dark){div{width:30px}}</style>';document.body.innerHTML='<div></div>'`)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		scheme string
		width  string
	}{{"light", "10"}, {"dark", "30"}, {"light", "10"}} {
		p.env.Preferences.ColorScheme = step.scheme
		value, err := p.Evaluate(context.Background(), `String(document.querySelector('div').getBoundingClientRect().width)`)
		if err != nil || value != step.width {
			t.Fatalf("scheme %s: width=%v error=%v", step.scheme, value, err)
		}
		assertBlitzOwnerActive(t, p)
	}
}
