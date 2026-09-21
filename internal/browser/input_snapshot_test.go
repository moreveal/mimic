//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"os"
	"testing"
)

// A snapshot can be seeded by either world kind. Its closures must acquire the
// destination world's ownership when rebound, before any author code runs.
func TestProtocolInputRestoredWorldOwnership(t *testing.T) {
	serialBrowserTest(t)
	p := bootstrapSnapshotPage(t)
	navigateCapabilityFixture(t, p)
	bootstrapSnapshotWarm(t, p)
	ctx := context.Background()
	bootstrapSnapshotEvaluate(t, p, `document.body.innerHTML='<input id="name"><select id="choice"><option>a</option><option>b</option></select><button id="button" onclick="window.clicked=(window.clicked||0)+1">Go</button>'`)
	world, err := p.isolatedWorld(ctx, p.Top.Realm, "automation")
	if err != nil {
		t.Fatal(err)
	}
	d := NewDebugger(p)
	defer d.Close()
	evaluate := func(source string) any {
		t.Helper()
		result, err := d.Evaluate(ctx, "", world.ID, source, DebuggerOptions{ReturnByValue: true})
		if err != nil || result["exceptionDetails"] != nil {
			t.Fatalf("isolated evaluation: %#v %v", result, err)
		}
		return result["result"].(map[string]any)["value"]
	}
	evaluate(`1`)
	if os.Getenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT") != "1" && !world.bootstrapRestored {
		t.Fatal("expected the observed isolated world restored from the warmed main-world snapshot")
	}
	evaluate(`document.querySelector('#name').focus()`)
	for _, letter := range []string{"A", "l", "p", "h", "a", " ", "4", "2"} {
		if err := p.DispatchProtocolInput(ctx, "Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": letter, "text": letter}); err != nil {
			t.Fatal(err)
		}
	}
	if result := evaluate(`(()=>{const s=document.querySelector('#choice');s.value=undefined;for(const o of s.options){o.selected=o.value==='b';if(o.selected&&!s.multiple)break}document.querySelector('#button').click();return document.querySelector('#name').value+'|'+s.value})()`); result != "Alpha 42|b" {
		t.Fatalf("isolated state after trusted typing/select/click: %#v", result)
	}
	if result := bootstrapSnapshotEvaluate(t, p, `document.querySelector('#name').value+'|'+document.querySelector('#choice').value+'|'+window.clicked`); result != "Alpha 42|b|1" {
		t.Fatalf("main world state after trusted typing/select/click: %#v", result)
	}
}
