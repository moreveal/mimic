package browser

import (
	"context"
	"fmt"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func BenchmarkFrameReferenceHandleLookup(b *testing.B) {
	for name, factory := range map[string]engine.Factory{"goja": gojaengine.Factory{}, "v8": v8engine.Factory{}} {
		for _, size := range []int{10, 100, 500} {
			b.Run(fmt.Sprintf("%s/%d", name, size), func(b *testing.B) {
				browser, err := New(factory, chrome152.New())
				if err != nil {
					b.Fatal(err)
				}
				page, err := browser.NewContext().NewPage()
				if err != nil {
					b.Fatal(err)
				}
				defer page.Close()
				r := page.Top.Realm
				var value engine.Value
				for i := 0; i < size; i++ {
					value, err = r.runtime.Eval(context.Background(), "({})", "benchmark")
					if err != nil {
						b.Fatal(err)
					}
					if _, err = r.crossRealmValue(value); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err = r.crossRealmValue(value); err != nil {
						b.Fatal(err)
					}
				}
				b.StopTimer()
			})
		}
	}
}

func TestFrameHandleIdentityUsesCapturedWeakMap(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		if _, err := page.Evaluate(context.Background(), `WeakMap.prototype.get=WeakMap.prototype.set=function(){throw Error('replaced WeakMap')};globalThis.a={};globalThis.b={};`); err != nil {
			t.Fatal(err)
		}
		r := page.Top.Realm
		a, err := r.crossRealmValue(r.runtime.Get("a"))
		if err != nil {
			t.Fatal(err)
		}
		again, err := r.crossRealmValue(r.runtime.Get("a"))
		if err != nil {
			t.Fatal(err)
		}
		other, err := r.crossRealmValue(r.runtime.Get("b"))
		if err != nil {
			t.Fatal(err)
		}
		if a["handle"] != again["handle"] || a["handle"] == other["handle"] {
			t.Fatalf("identity changed: %v %v %v", a, again, other)
		}
	})
}
