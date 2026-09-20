package browser

import (
	"context"
	"testing"
)

const inputStackingSetup = `document.body.style.margin='0';document.body.innerHTML='<section id="context" style="position:absolute;left:0;top:0;width:100px;height:100px;z-index:3"><button id="target" style="position:absolute;left:0;top:0;width:100px;height:100px">button</button></section><div id="cover" style="position:fixed;left:0;top:0;width:100px;height:100px;z-index:2"></div>';globalThis.clicked='';document.addEventListener('click',e=>clicked=e.target.id);`

func TestIsolatedHitTestUsesOwnerAndLocalWrappers(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		d := NewDebugger(p)
		defer d.Close()
		debuggerEval(t, d, inputStackingSetup+`Document.prototype.elementsFromPoint=()=>{throw Error('author override')}`, DebuggerOptions{})
		world, err := p.IsolatedWorld(context.Background(), p.Top.ID, "hit-test")
		if err != nil {
			t.Fatal(err)
		}
		result, err := d.Evaluate(context.Background(), p.Top.ID, world, `(()=>{
const target=document.getElementById('target'),cover=document.getElementById('cover');
target.getBoundingClientRect();
const first=document.elementFromPoint(30,30)===target;
cover.style.zIndex='4';const changed=document.elementsFromPoint(30,30)[0]===cover;
return first&&changed&&document.elementFromPoint(NaN,30)===null&&document.elementsFromPoint(-1,30).length===0;
})()`, DebuggerOptions{ReturnByValue: true})
		if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != true {
			t.Fatalf("isolated hit test: %#v %v", result, err)
		}
	})
}

func TestInputStackingContextHitTargetsMatchChrome152(t *testing.T) {
	parallelBrowserTest(t)
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
