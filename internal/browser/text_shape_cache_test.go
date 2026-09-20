package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTextShapeCachePreservesAllShapeInputs(t *testing.T) {
	parallelBrowserTest(t)
	p := newAsyncModulePage(t)
	navigateCapabilityFixture(t, p)
	r := p.Top.Realm
	host := map[string]any{}
	r.installTextMetrics(host)
	if err := r.runtime.Set("shapeForTest", host["shapeText"]); err != nil {
		t.Fatal(err)
	}
	cases := [][]any{
		{"AV ffi", "Arial", 20, 400, 0, 0, 0, 0},
		{"AV ffl", "Arial", 20, 400, 0, 0, 0, 0},
		{"AV ffi", "Courier New", 20, 400, 0, 0, 0, 0},
		{"AV ffi", "Arial", 21, 400, 0, 0, 0, 0},
		{"AV ffi", "Arial", 20, 700, 0, 0, 0, 0},
		{"AV ffi", "Arial", 20, 400, 1, 0, 0, 0},
		{"AV ffi", "Arial", 20, 400, 0, 1, 0, 0},
		{"AV ffi", "Arial", 20, 400, 0, 0, 1, 0},
		{"AV ffi", "Arial", 20, 400, 0, 0, 0, 1},
	}
	for round := 0; round < 2; round++ {
		for i, args := range cases {
			shape := p.textMetricsEngine().ShapeWithFonts
			if args[7].(int) != 0 {
				shape = p.textMetricsEngine().ShapeCanvasWithFonts
			}
			result, err := shape(args[0].(string), args[1].(string), float64(args[2].(int)), float64(args[3].(int)), args[4].(int) != 0, args[5].(int) != 0, args[6].(int) != 0, r.fontChoices)
			if err != nil {
				t.Fatal(err)
			}
			expected, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(args)
			actual, err := p.Evaluate(context.Background(), "shapeForTest(..."+string(encoded)+")")
			if err != nil || actual != string(expected) {
				t.Fatalf("round%d case%d: %v %v; want %s", round, i, actual, err, expected)
			}
		}
	}
	if r.textShapeCache == nil || len(r.textShapeCache.values) != len(cases) {
		t.Fatal("successful shape reuse missing")
	}
	count := len(r.textShapeCache.values)
	for i := 0; i < 2; i++ {
		value, err := p.Evaluate(context.Background(), `shapeForTest('x','Arial',NaN,400,0,0,0,0)`)
		if err != nil || !strings.Contains(fmt.Sprint(value), "error") {
			t.Fatalf("font failure: %v %v", value, err)
		}
	}
	if len(r.textShapeCache.values) != count {
		t.Fatal("failed shape was cached")
	}
}

func TestTextShapeCacheInvalidatesForFontLoadAndCollectionChanges(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `(async()=>{
		const c=new OffscreenCanvas(100,100),x=c.getContext('2d');x.font='20px CacheFace, Arial';
		const measure=()=>x.measureText('iiiiWWAV').width;
		const fallback=measure();if(measure()!==fallback)return 'unstable';
		const face=new FontFace('CacheFace','local("Courier New")');document.fonts.add(face);
		if(measure()!==fallback)return 'unloaded selected';await face.load();
		const loaded=measure();if(loaded===fallback||measure()!==loaded)return 'load not reflected';
		document.fonts.delete(face);if(measure()!==fallback)return 'delete not reflected';
		document.fonts.add(face);if(measure()!==loaded)return 'add not reflected';
		face.family='OtherFace';if(measure()!==fallback)return 'descriptor not reflected';
		face.family='CacheFace';if(measure()!==loaded)return 'descriptor restore';
		document.fonts.clear();if(measure()!==fallback)return 'clear not reflected';
		return 'ok'})()`)
		if err != nil || value != "ok" {
			t.Fatalf("font selection cache: %v %v", value, err)
		}
		old := p.Top.Realm
		if old.textShapeCache == nil {
			t.Fatal("fixture did not populate shaping cache")
		}
		navigateCapabilityFixture(t, p)
		if old.textShapeCache != nil {
			t.Fatal("retired document retained shaping cache")
		}
	})
}

func TestTextShapeCacheBoundsRetainedInputsAndResults(t *testing.T) {
	parallelBrowserTest(t)
	cache := &textShapeCache{}
	for i := 0; i < textShapeCacheEntries+128; i++ {
		key := textShapeKey{text: fmt.Sprint(i), families: "Arial"}
		cache.put(key, strings.Repeat("x", 4096))
		if cache.bytes > textShapeCacheBytes || len(cache.values) > textShapeCacheEntries || len(cache.order) > textShapeCacheEntries {
			t.Fatal("shaping cache exceeded retention bound")
		}
		if _, ok := cache.get(key); !ok {
			t.Fatal("new successful result not available")
		}
	}
	if _, ok := cache.get(textShapeKey{text: "0", families: "Arial"}); ok {
		t.Fatal("old entry was not evicted")
	}
	key := textShapeKey{text: strings.Repeat("large", textShapeCacheBytes)}
	cache.put(key, "ok")
	if _, ok := cache.get(key); ok {
		t.Fatal("oversized input retained")
	}
}

func TestTextShapeCacheRetainsCompactWorkingSetWithinByteBudget(t *testing.T) {
	parallelBrowserTest(t)
	cache := &textShapeCache{}
	const entries = 2048
	const result = `{"advance":53.375,"ascent":15,"descent":4,"lineGap":0}`
	keyFor := func(i int) textShapeKey {
		return textShapeKey{text: fmt.Sprintf("distinct text observation %d", i), families: "Arial", flags: 1 << 4}
	}
	for i := 0; i < entries; i++ {
		cache.put(keyFor(i), result)
	}
	if cache.bytes > textShapeCacheBytes || len(cache.values) != entries {
		t.Fatalf("compact working set was evicted below byte budget: entries=%d bytes=%d", len(cache.values), cache.bytes)
	}
	beforeBytes, beforeOrder, beforeOldest := cache.bytes, len(cache.order), cache.oldest
	for round := 0; round < 2; round++ {
		for i := 0; i < entries; i++ {
			if value, hit := cache.get(keyFor(i)); !hit || value != result {
				t.Fatalf("second-pass reshaping required: round=%d key=%d", round, i)
			}
		}
	}
	if cache.bytes != beforeBytes || len(cache.order) != beforeOrder || cache.oldest != beforeOldest {
		t.Fatal("cache reads altered FIFO ownership or retained memory")
	}
}
