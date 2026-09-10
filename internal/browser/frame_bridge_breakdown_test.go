//go:build windows && amd64

package browser

import (
	"context"
	"encoding/json"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"
	"unsafe"
)

func TestFrameBridgeBreakdown(t *testing.T) {
	clockDLL := windows.NewLazySystemDLL("kernel32.dll")
	counter := clockDLL.NewProc("QueryPerformanceCounter")
	frequency := clockDLL.NewProc("QueryPerformanceFrequency")
	var frequencyValue int64
	frequency.Call(uintptr(unsafe.Pointer(&frequencyValue)))
	tick := func() float64 {
		var value int64
		counter.Call(uintptr(unsafe.Pointer(&value)))
		return float64(value) * 1000 / float64(frequencyValue)
	}
	dir := os.Getenv("MIMIC_BRIDGE_BREAKDOWN")
	if dir == "" {
		t.Skip("diagnostic")
	}
	os.MkdirAll(dir, 0700)
	b, e := New(v8engine.Factory{}, chrome152.New())
	if e != nil {
		t.Fatal(e)
	}
	p, e := b.NewContext().NewPage()
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	ctx := context.Background()
	eval := func(s string) any {
		v, e := p.Evaluate(ctx, s)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	eval("true")
	setup := `globalThis.f=document.createElement('iframe');document.body.appendChild(f);globalThis.w=f.contentWindow;w.eval("globalThis.chain=Object.create(Object.create(Object.create(null)));globalThis.fn=x=>x+1;globalThis.obj={x:7}");true`
	rows := []map[string]any{}
	for round := 0; round < 6; round++ {
		start := tick()
		eval(setup)
		rows = append(rows, map[string]any{"phase": "setup", "round": round, "ms": tick() - start})
		for _, phase := range []struct{ name, source string }{
			{"noop", `6050`},
			{"global-chain-100", `(()=>{let p;for(let i=0;i<100;i++)p=w.chain;return p!==null})()`},
			{"prototype-300", `(()=>{let n=0;const root=w.chain;for(let i=0;i<100;i++){let p=root;while(p!==null){n++;p=Object.getPrototypeOf(p)}}return n})()`},
			{"global-obj-and-read-100", `(()=>{let n=0;for(let i=0;i<100;i++)n+=w.obj.x;return n})()`},
			{"global-fn-and-call-100", `(()=>{let n=0;for(let i=0;i<100;i++)n+=w.fn(i);return n})()`},
			{"mixed", `(()=>{let checks=0;for(let i=0;i<100;i++){let p=w.chain;while(p!==null){checks++;p=Object.getPrototypeOf(p)}checks+=w.obj.x;checks+=w.fn(i)}return checks})()`},
			{"local-equivalent", `(()=>{const v={chain:Object.create(Object.create(Object.create(null))),obj:{x:7},fn:x=>x+1};let checks=0;for(let i=0;i<100;i++){let p=v.chain;while(p!==null){checks++;p=Object.getPrototypeOf(p)}checks+=v.obj.x;checks+=v.fn(i)}return checks})()`},
		} {
			start := tick()
			v := eval(phase.source)
			rows = append(rows, map[string]any{"phase": phase.name, "round": round, "ms": tick() - start, "result": v})
		}
		start = tick()
		eval("f.remove();true")
		rows = append(rows, map[string]any{"phase": "remove", "round": round, "ms": tick() - start})
	}
	eval(setup)
	if os.Getenv("MIMIC_PROFILE_HOSTS") == "1" {
		snap := func(name string) {
			for _, frame := range p.frames {
				if frame.Realm == nil {
					continue
				}
				if d, ok := frame.Realm.runtime.(interface{ Diagnostics() (any, error) }); ok {
					v, e := d.Diagnostics()
					if e != nil {
						t.Fatal(e)
					}
					data, _ := json.Marshal(v)
					os.WriteFile(filepath.Join(dir, name+"-"+frame.Realm.ID+".json"), data, 0600)
				}
			}
		}
		snap("before")
		cpu, e := os.Create(filepath.Join(dir, "cpu.pprof"))
		if e != nil {
			t.Fatal(e)
		}
		pprof.StartCPUProfile(cpu)
		for i := 0; i < 20; i++ {
			eval(`(()=>{let checks=0;for(let i=0;i<100;i++){let p=w.chain;while(p!==null){checks++;p=Object.getPrototypeOf(p)}checks+=w.obj.x;checks+=w.fn(i)}return checks})()`)
		}
		pprof.StopCPUProfile()
		cpu.Close()
		snap("after")
	}
	data, _ := json.MarshalIndent(rows, "", "  ")
	os.WriteFile(filepath.Join(dir, "phases.json"), data, 0600)
}
