package browser

import (
	"context"
	"testing"
)

func TestForeignStyleProjectionInvalidatesCanonicalInputs(t *testing.T) {
	parallelBrowserTest(t)
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
	parallelBrowserTest(t)
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

func TestStyleProjectionEpochIgnoresImageOnlyCompletion(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	r := p.Top.Realm
	styleBefore := r.styleProjectionEpoch("value")
	boxBefore := r.styleProjectionEpoch("box")
	r.resourceRevision.Add(1)
	if got := r.styleProjectionEpoch("value"); got != styleBefore {
		t.Fatal("image-only completion invalidated scalar style projections")
	}
	if got := r.styleProjectionEpoch("box"); got == boxBefore {
		t.Fatal("image-only completion did not invalidate intrinsic geometry")
	}
	r.styleResourceRevision.Add(1)
	if got := r.styleProjectionEpoch("value"); got == styleBefore {
		t.Fatal("stylesheet completion did not invalidate scalar style projections")
	}
}

func TestStyleProjectionEpochTracksOnlyConnectedDOMAcrossRealms(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		d := NewDebugger(p)
		defer d.Close()
		r := p.Top.Realm
		before := r.styleProjectionEpoch("value")
		debuggerEval(t, d, `globalThis.detachedProbe=document.createElement('div');detachedProbe.className='before';detachedProbe.textContent='detached'`, DebuggerOptions{})
		if got := r.styleProjectionEpoch("value"); got != before {
			t.Fatalf("detached construction invalidated style projection: %#v -> %#v", before, got)
		}
		debuggerEval(t, d, `document.body.append(detachedProbe)`, DebuggerOptions{})
		connected := r.styleProjectionEpoch("value")
		if connected == before {
			t.Fatal("connected insertion did not invalidate style projection")
		}
		world, err := p.IsolatedWorld(context.Background(), p.Top.ID, "observation-revision")
		if err != nil {
			t.Fatal(err)
		}
		result, err := d.Evaluate(context.Background(), p.Top.ID, world, `document.querySelector('div').setAttribute('data-cross-realm','yes')`, DebuggerOptions{ReturnByValue: true})
		if err != nil || result["exceptionDetails"] != nil {
			t.Fatalf("isolated mutation: %#v %v", result, err)
		}
		if got := r.styleProjectionEpoch("value"); got == connected {
			t.Fatal("cross-realm connected mutation did not invalidate style projection")
		}
	})
}

func TestDetachedStyleReadsRevalidateWithoutConnectedEpoch(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		historyEval(t, p, `(()=>{
const element=document.createElement('div');
element.style.width='10px';
const before=getComputedStyle(element).width;
element.style.width='20px';
const after=getComputedStyle(element).width;
const first=element.getBoundingClientRect();
element.textContent='changed';
const second=element.getBoundingClientRect();
return [before,after,first.width,second.width].join(',');
})()`, ",,0,0")
	})
}
