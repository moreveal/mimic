package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestConsumedWindowStateAndFrameViewport(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `(()=>{
 const f=document.createElement('iframe');f.setAttribute('name','child');document.body.append(f);const w=f.contentWindow;
 const out=[closed,w.closed,w.name,w.status,w.opener===null,w.innerWidth,w.innerHeight];
 w.name='changed';w.status=42;out.push(w.name,w.status,w.eval('name'),name);
 f.style.cssText='width:240px;height:110px';out.push(w.innerWidth,w.innerHeight);
 f.style.display='none';out.push(w.innerWidth,w.innerHeight);
 f.style.display='block';out.push(w.innerWidth,w.innerHeight);
 const d=Object.getOwnPropertyDescriptor(w,'innerWidth');w.innerWidth=77;out.push(w.innerWidth);Object.defineProperty(w,'innerWidth',d);
 out.push(screenX===screenLeft,screenY===screenTop);f.remove();out.push(w.closed,w.innerWidth,w.innerHeight,w.outerWidth,w.outerHeight,w.document.hidden,w.document.visibilityState);
 return JSON.stringify(out);
 })()`)
		if err != nil {
			t.Fatal(err)
		}
		want := `[false,false,"child","",true,300,150,"changed","42","changed","",240,110,240,110,240,110,77,true,true,true,0,0,0,0,true,"hidden"]`
		if value != want {
			t.Fatalf("got %v; want %s", value, want)
		}
	})
}

func TestConsumedWindowEventChromeOracle(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		if strings.HasSuffix(t.Name(), "/goja") {
			t.Skip("Goja cannot checkpoint its job queue inside the private native dispatch stack")
		}
		source, err := os.ReadFile("testdata/consumed_window_event_oracle.js")
		if err != nil {
			t.Fatal(err)
		}
		expected, err := os.ReadFile("testdata/consumed_window_event_chrome152.json")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		actual, err := p.Evaluate(ctx, string(source))
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(actual)
		if err != nil {
			t.Fatal(err)
		}
		var got, want any
		json.Unmarshal(encoded, &got)
		json.Unmarshal(expected, &want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("event oracle: got %s; want %s", encoded, expected)
		}
	})
}

func TestConsumedWindowObjectsAndPrivateViewportQuery(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `(()=>{
 const f=document.createElement('iframe');f.style='display:none';document.body.append(f);const w=f.contentWindow;
 const original=document.querySelectorAll;document.querySelectorAll=()=>{throw Error('public query invoked')};
 let out;try{out=[w.innerWidth,w.innerHeight,locationbar.visible,menubar!==locationbar,external.AddSearchProvider('x')===undefined,external.IsSearchProviderInstalled('x')===undefined,styleMedia.type,styleMedia.matchMedium('screen'),styleMedia.matchMedium('print'),fence===null,visualViewport===visualViewport,visualViewport.width===innerWidth,viewport.segments.length,Object.isFrozen(viewport.segments),w.viewport.segments===null];f.style.display='block';out.push(w.innerWidth,w.innerHeight)}finally{document.querySelectorAll=original;f.remove()}
 return JSON.stringify(out);
 })()`)
		if err != nil {
			t.Fatal(err)
		}
		want := `[0,0,true,true,true,true,"screen",true,false,true,true,true,1,true,true,300,150]`
		if value != want {
			t.Fatalf("got %v; want %s", value, want)
		}
	})
}

func TestConsumedWindowStateNavigationAndViewportResize(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `(async()=>{const f=document.createElement('iframe');f.srcdoc='one';await new Promise(r=>{f.onload=r;document.body.append(f)});const w=f.contentWindow;w.name='kept-name';w.status='kept-status';f.srcdoc='two';await new Promise(r=>f.onload=r);const out=[f.contentWindow===w,w.name,w.status];f.remove();globalThis.resizeLog=[];visualViewport.onresize=()=>resizeLog.push(visualViewport.width);return JSON.stringify(out)})()`)
		if err != nil {
			t.Fatal(err)
		}
		if value != `[true,"kept-name",""]` {
			t.Fatalf("navigation state: %v", value)
		}
		if err := p.SetViewport(800, 500); err != nil {
			t.Fatal(err)
		}
		if err := p.AdvanceTime(ctx, time.Millisecond); err != nil {
			t.Fatal(err)
		}
		value, err = p.Evaluate(ctx, `JSON.stringify(resizeLog)`)
		if err != nil || value != `[800]` {
			t.Fatalf("resize: %v %v", value, err)
		}
	})
}

func TestConsumedCPUPerformanceUsesEnvironment(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		p.mu.Lock()
		p.env.Hardware.CPUPerformance = 3
		p.env.Hardware.CPUPerformanceKnown = true
		p.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `navigator.cpuPerformance`)
		if err != nil || numberValue(value) != 3 {
			detail, _ := p.Evaluate(ctx, `JSON.stringify({descriptor:Object.getOwnPropertyDescriptor(Navigator.prototype,"cpuPerformance"),has:"cpuPerformance" in navigator})`)
			t.Fatalf("CPU performance profile: %v %v %v", value, err, detail)
		}
	})
}

func TestConsumedNativeListenerCheckpoint(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		if strings.HasSuffix(t.Name(), "/goja") {
			t.Skip("Goja has no nested native callback checkpoint")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `(async()=>{
 const log=[];
 const one=e=>{log.push('first');queueMicrotask(()=>log.push('micro:'+String(event)))};
 const two=e=>log.push('second');
 addEventListener('probe',one);addEventListener('probe',two);dispatchEvent(new Event('probe'));log.push('script');await Promise.resolve();
 removeEventListener('probe',one);removeEventListener('probe',two);
 await new Promise(resolve=>{const first=e=>{if(e.data!=='checkpoint')return;removeEventListener('message',first);log.push('native-first');queueMicrotask(()=>log.push('native-micro:'+event.type))};const second=e=>{if(e.data!=='checkpoint')return;removeEventListener('message',second);log.push('native-second');resolve()};addEventListener('message',first);addEventListener('message',second);postMessage('checkpoint','*')});
 return JSON.stringify(log);
 })()`)
		if err != nil {
			t.Fatal(err)
		}
		if value != `["first","second","script","micro:undefined","native-first","native-micro:message","native-second"]` {
			t.Fatalf("checkpoint order: %v", value)
		}
	})
}
