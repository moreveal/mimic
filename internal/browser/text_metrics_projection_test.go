package browser

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"
)

func exposeTextProjectionHosts(t *testing.T, p *Page) {
	t.Helper()
	host := map[string]any{}
	p.Top.Realm.installTextMetrics(host)
	for _, name := range []string{"shapeText", "shapeTextMetrics"} {
		if err := p.Top.Realm.runtime.Set(name, host[name]); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCompactTextMetricsMatchFullGlyphProjection(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		exposeTextProjectionHosts(t, p)
		value, err := p.Evaluate(context.Background(), `(()=>{
		const texts=['','AV ffi ffl','iiiiWWAV','A\u0301 عربى 中文 😀','A '.repeat(120)];
		for(const text of texts)for(const families of ['Arial','Courier New'])for(let variant=0;variant<8;variant++){
		 const args=[text,families,variant===0?19.37:20,variant===1?700:400,...[2,3,4,5].map(v=>Number(v===variant))];
		 const full=JSON.parse(shapeText(...args)), compact=JSON.parse(shapeTextMetrics(...args));
		 if(full.error||compact.error){if(full.error!==compact.error)throw new Error('shaping error changed');continue}
		 const expected={advance:full.glyphs.reduce((sum,g)=>sum+g.advance,0),ascent:full.ascent,descent:full.descent,lineGap:full.lineGap};
		 if(JSON.stringify(compact)!==JSON.stringify(expected))throw new Error(JSON.stringify({args,expected,compact}));
		 compact.advance=-1;
		 if(JSON.parse(shapeTextMetrics(...args)).advance!==expected.advance)throw new Error('mutable result');
		}
		for(let i=0;i<2;i++)if(!JSON.parse(shapeTextMetrics('x','Arial',NaN,400,0,0,0,0)).error)throw new Error('error lost');
		return 'ok'})()`)
		if err != nil || value != "ok" {
			t.Fatalf("compact metrics: %v %v", value, err)
		}
		for key, value := range p.Top.Realm.textShapeCache.values {
			if key.flags&(1<<4) != 0 {
				var fields map[string]any
				if err := json.Unmarshal([]byte(value), &fields); err != nil || len(fields) != 4 {
					t.Fatalf("compact cache retained non-metric data: %s %v", value, err)
				}
			}
		}
	})
}

func TestCompactTextMetricsObserveFontCollectionChanges(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		exposeTextProjectionHosts(t, p)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `(async()=>{
		const measure=()=>JSON.parse(shapeTextMetrics('iiiiWWAV','CacheFace, Arial',20,400,0,0,0,0)).advance;
		const fallback=measure(),face=new FontFace('CacheFace','local("Courier New")');document.fonts.add(face);
		if(measure()!==fallback)return 'unloaded';await face.load();const loaded=measure();
		if(loaded===fallback||measure()!==loaded)return 'load';
		document.fonts.delete(face);if(measure()!==fallback)return 'delete';
		document.fonts.add(face);if(measure()!==loaded)return 'add';
		face.family='OtherFace';if(measure()!==fallback)return 'descriptor';
		face.family='CacheFace';if(measure()!==loaded)return 'restore';
		document.fonts.clear();if(measure()!==fallback)return 'clear';return 'ok'})()`)
		if err != nil || value != "ok" {
			t.Fatalf("compact font selection: %v %v", value, err)
		}
		old := p.Top.Realm
		if old.textShapeCache == nil {
			t.Fatal("compact cache not populated")
		}
		navigateCapabilityFixture(t, p)
		if old.textShapeCache != nil {
			t.Fatal("retired document retained compact metrics")
		}
	})
}

func TestTextShapeHostsDoNotRetainArguments(t *testing.T) {
	parallelBrowserTest(t)
	p := newAsyncModulePage(t)
	navigateCapabilityFixture(t, p)
	exposeTextProjectionHosts(t, p)
	diagnostic, ok := p.Top.Realm.runtime.(interface{ Diagnostics() (any, error) })
	if !ok {
		t.Fatal("native runtime diagnostics unavailable")
	}
	count := func() int {
		data, err := diagnostic.Diagnostics()
		if err != nil {
			t.Fatal(err)
		}
		return data.(map[string]any)["persistent_handles"].(int)
	}
	for _, host := range []string{"shapeText", "shapeTextMetrics"} {
		before := count()
		value, err := p.Evaluate(context.Background(), `(()=>{for(let i=0;i<500;i++){const result=JSON.parse(`+host+`('AV ffi '+i,'Arial',16,400,0,0,0,0));if(result.error)return result.error}return 'ok'})()`)
		if err != nil || value != "ok" {
			t.Fatalf("%s: %v %v", host, value, err)
		}
		// Evaluation's own result/housekeeping may retain a few values. Callback
		// arguments must not add one permanent handle per argument per call.
		if growth := count() - before; growth > 16 {
			t.Fatalf("%s retained %d handles for 500 observations", host, growth)
		}
	}
}

// An explicitly requested local measurement, excluded from correctness runs.
// Every round observes 128 distinct text runs twice; the first round is warmup.
// Full and compact calls are alternated, use identical inputs, and each parse
// and observe exactly the CSS-required metrics. No profiler is active.
func TestTextMetricsProjectionMeasurements(t *testing.T) {
	serialBrowserTest(t)
	if os.Getenv("MIMIC_MEASURE_TEXT_PROJECTION") != "1" {
		t.Skip("explicit local performance diagnostic")
	}
	p := newAsyncModulePage(t)
	navigateCapabilityFixture(t, p)
	exposeTextProjectionHosts(t, p)
	_, err := p.Evaluate(context.Background(), `globalThis.projectTextBatch=compact=>{
	 let checksum=0,bytes=0;
	 for(let round=0;round<2;round++)for(let i=0;i<128;i++){
	  const text='A distinct text run '+i+': '+('AV ffi geometry observations ').repeat(4);
	  const encoded=(compact?shapeTextMetrics:shapeText)(text,'Arial',16,400,0,0,0,0),m=JSON.parse(encoded);
	  checksum+=(compact?m.advance:m.glyphs.reduce((sum,g)=>sum+g.advance,0))+m.ascent+m.descent+m.lineGap;
	  bytes+=encoded.length;
	 }
	 return {checksum,bytes};
	}`)
	if err != nil {
		t.Fatal(err)
	}
	type measurement struct {
		Mode          string  `json:"mode"`
		MedianMS      float64 `json:"medianMs"`
		P95MS         float64 `json:"p95Ms"`
		ResponseBytes int     `json:"responseBytes"`
		CacheEntries  int     `json:"cacheEntries"`
		CacheBytes    int     `json:"cacheBytes"`
	}
	var samples [2][]float64
	var outputs [2]measurement
	var expectedChecksum float64
	for round := 0; round <= 30; round++ {
		for step := 0; step < 2; step++ {
			mode := (round + step) % 2
			// Isolate the two projected working sets without retaining both.
			p.Top.Realm.textShapeCache = nil
			expression := "projectTextBatch(false)"
			if mode == 1 {
				expression = "projectTextBatch(true)"
			}
			start := time.Now()
			result, err := p.Evaluate(context.Background(), expression)
			elapsed := float64(time.Since(start)) / float64(time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(result)
			var observed struct {
				Checksum float64
				Bytes    int
			}
			if err := json.Unmarshal(data, &observed); err != nil {
				t.Fatal(err)
			}
			if expectedChecksum == 0 {
				expectedChecksum = observed.Checksum
			}
			if observed.Checksum != expectedChecksum {
				t.Fatal("projection checksum differs")
			}
			if round > 0 {
				samples[mode] = append(samples[mode], elapsed)
			}
			cache := p.Top.Realm.textShapeCache
			outputs[mode] = measurement{Mode: []string{"full", "compact"}[mode], ResponseBytes: observed.Bytes, CacheEntries: len(cache.values), CacheBytes: cache.bytes}
		}
	}
	for mode := range outputs {
		sort.Float64s(samples[mode])
		outputs[mode].MedianMS = (samples[mode][14] + samples[mode][15]) / 2
		outputs[mode].P95MS = samples[mode][28]
	}
	data, _ := json.Marshal(outputs)
	t.Log(string(data))
}
