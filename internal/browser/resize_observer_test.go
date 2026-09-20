package browser

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestResizeObserverInitialChangeAndBoxSizes(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `(async()=>{
 const target=document.createElement('div');
 target.style.cssText='box-sizing:content-box;width:40px;height:20px;padding:3px;border:2px solid';
 document.body.append(target);
 const samples=[];
 await new Promise(resolve=>{
   const observer=new ResizeObserver(entries=>{
     const entry=entries[0];
     samples.push([entry.contentRect.width,entry.contentRect.height,entry.contentBoxSize[0].inlineSize,entry.borderBoxSize[0].inlineSize]);
     if(samples.length===1) target.style.width='75px';
     else { observer.disconnect(); resolve(); }
   });
   observer.observe(target);
 });
 return JSON.stringify(samples);
})()`)
		if err != nil {
			t.Fatal(err)
		}
		var samples [][]float64
		if err := json.Unmarshal([]byte(value.(string)), &samples); err != nil {
			t.Fatal(err)
		}
		want := [][]float64{{40, 20, 40, 50}, {75, 20, 75, 85}}
		if len(samples) != len(want) {
			t.Fatalf("samples: got %v want %v", samples, want)
		}
		for i := range want {
			for j := range want[i] {
				if samples[i][j] != want[i][j] {
					t.Fatalf("samples: got %v want %v", samples, want)
				}
			}
		}
	})
}

func TestResizeObserverMultipleTargetsUnobserveAndDisconnect(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `(async()=>{
 const first=document.createElement('div'),second=document.createElement('div');
 first.style.width='10px'; second.style.width='20px';
 document.body.append(first,second);
 let calls=0,seen=[];
 await new Promise(resolve=>{
   const observer=new ResizeObserver(entries=>{
     calls++; seen.push(entries.map(entry=>entry.target===first?'first':'second').sort().join(','));
     if(calls===1){ observer.unobserve(first); second.style.width='30px'; }
     else { observer.disconnect(); second.style.width='40px'; setTimeout(resolve,40); }
   });
   observer.observe(first); observer.observe(second);
 });
 return JSON.stringify([calls,seen]);
})()`)
		if err != nil {
			t.Fatal(err)
		}
		if value != `[2,["first,second","second"]]` {
			t.Fatalf("got %v", value)
		}
	})
}
