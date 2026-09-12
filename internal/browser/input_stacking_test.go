package browser

import (
	"context"
	"testing"
)

const inputStackingSetup = `document.body.style.margin='0';document.body.innerHTML='<section id="context" style="position:absolute;left:0;top:0;width:100px;height:100px;z-index:3"><button id="target" style="position:absolute;left:0;top:0;width:100px;height:100px">button</button></section><div id="cover" style="position:fixed;left:0;top:0;width:100px;height:100px;z-index:2"></div>';globalThis.clicked='';document.addEventListener('click',e=>clicked=e.target.id);`

func TestInputStackingContextHitTargetsMatchChrome152(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		ctx := context.Background()
		if _, err := p.Evaluate(ctx, inputStackingSetup); err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct{ change, want string }{
			{`void 0`, "target"},
			{`document.getElementById('cover').style.zIndex='4'`, "cover"},
			{`document.getElementById('context').style.zIndex='auto';document.getElementById('target').style.zIndex='5'`, "target"},
			{`document.getElementById('context').style.zIndex='1';document.getElementById('target').style.zIndex='999'`, "cover"},
		} {
			if _, err := p.Evaluate(ctx, test.change); err != nil {
				t.Fatal(err)
			}
			for _, kind := range []string{"mousePressed", "mouseReleased"} {
				if err := p.DispatchProtocolInput(ctx, "Input.dispatchMouseEvent", map[string]any{"type": kind, "x": 30, "y": 30, "button": "left", "clickCount": 1}); err != nil {
					t.Fatal(err)
				}
			}
			got, err := p.Evaluate(ctx, `clicked`)
			if err != nil || got != test.want {
				t.Fatalf("%s: target=%v want=%s err=%v", test.change, got, test.want, err)
			}
		}
	})
}
