package browser

import (
	"context"
	"testing"
)

func TestBlitzCannotBeDisabledByLegacyEnvironment(t *testing.T) {
	for _, mode := range []string{"", "blitz", "legacy"} {
		t.Run("mode="+mode, func(t *testing.T) {
			t.Setenv("MIMIC_STYLE_ENGINE", mode)
			historyTestPages(t, func(t *testing.T, p *Page) {
				navigateCapabilityFixture(t, p)
				got, err := p.Evaluate(context.Background(), `document.body.innerHTML='<div style="width:42px;height:17px"></div>';String(document.body.firstChild.getBoundingClientRect().width)`)
				if err != nil || got != "42" {
					t.Fatalf("observation %v, error %v", got, err)
				}
				native := p.Top.Realm.blitz != nil && p.Top.Realm.blitz.document.Owner != nil
				if !native {
					t.Fatalf("mode %q native owner=%v", mode, native)
				}
			})
		})
	}
}
