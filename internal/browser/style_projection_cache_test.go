package browser

import (
	"context"
	"testing"
)

func TestForeignStyleProjectionInvalidatesCanonicalInputs(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx := context.Background()
		d := NewDebugger(p)
		defer d.Close()
		debuggerEval(t, d, `
document.head.innerHTML = '<style>input {width:20px} input:focus {width:40px} input:checked {width:60px}</style>';
document.body.innerHTML = '<input id="probe" type="checkbox">';
`, DebuggerOptions{})
		world, err := p.IsolatedWorld(ctx, p.Top.ID, "projection-test")
		if err != nil {
			t.Fatal(err)
		}
		property := "width"
		read := func(want string) {
			t.Helper()
			result, err := d.Evaluate(ctx, p.Top.ID, world, `getComputedStyle(document.getElementById('probe')).`+property, DebuggerOptions{ReturnByValue: true})
			if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != want {
				t.Fatalf("projection: %#v %v; want %s", result, err, want)
			}
		}
		read("20px")
		read("20px")
		if len(p.Top.Realm.styleProjections.values) == 0 {
			t.Fatal("scalar projection not retained")
		}
		debuggerEval(t, d, `document.getElementById('probe').focus()`, DebuggerOptions{})
		read("40px")
		debuggerEval(t, d, `document.getElementById('probe').checked = true`, DebuggerOptions{})
		read("60px")
		debuggerEval(t, d, `document.styleSheets[0].cssRules[2].style.width = '80px'`, DebuggerOptions{})
		read("80px")
		debuggerEval(t, d, `document.getElementById('probe').style.width='10vw'`, DebuggerOptions{})
		if err := p.SetViewport(800, 600); err != nil {
			t.Fatal(err)
		}
		read("80px")
		if err := p.SetViewport(1000, 600); err != nil {
			t.Fatal(err)
		}
		read("100px")
		property = "opacity"
		read("1")
		debuggerEval(t, d, `
globalThis.effect = document.getElementById('probe').animate(
  [{opacity:0.2}, {opacity:0.8}], {duration:1000, fill:'both'}
);
effect.pause();
effect.currentTime = 0;
`, DebuggerOptions{})
		read("0.2")
		debuggerEval(t, d, `effect.currentTime = 500`, DebuggerOptions{})
		read("0.5")
		if !p.Top.Realm.styleProjections.dynamic {
			t.Fatal("animation must disable mutation-only projections")
		}
	})
}

func TestStyleProjectionCacheBounds(t *testing.T) {
	var cache styleProjectionCache
	epoch := styleProjectionEpoch{document: 1}
	for i := 0; i < 40000; i++ {
		cache.put(epoch, styleProjectionKey{node: int64(i), kind: "value", property: "display"}, "block")
	}
	if len(cache.values) != 32768 {
		t.Fatal(len(cache.values))
	}
	if _, ok := cache.get(epoch, styleProjectionKey{node: 0, kind: "value", property: "display"}); !ok {
		t.Fatal("admission discarded hot entries")
	}
	if _, ok := cache.get(styleProjectionEpoch{document: 2}, styleProjectionKey{node: 0, kind: "value", property: "display"}); ok {
		t.Fatal("stale epoch reused")
	}
	cache.disable()
	if len(cache.values) != 0 || cache.bytes != 0 {
		t.Fatal("disabled cache retained scalars")
	}
}
