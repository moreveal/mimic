package browser

import (
	"context"
	"testing"
	"time"
)

func TestFontCollectionChangesInvalidateGeometryWithinSameJob(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `(async()=>{
		const face=new FontFace('GeometryEpochFace','local("Courier New")');await face.load();
		const target=document.createElement('span');
		target.style.cssText='display:inline-block;font-family:GeometryEpochFace,Arial;font-size:20px';
		target.textContent='iiiiWWAV';document.body.append(target);
		const computed=getComputedStyle(target);
		const measure=()=>[target.offsetWidth,target.clientWidth,computed.width].join(',');
		// All following observations and mutations occur in one synchronous job.
		const fallback=measure();document.fonts.add(face);const loaded=measure();
		if(loaded.split(',').some((value,index)=>value===fallback.split(',')[index]))return 'add kept stale fallback';
		document.fonts.delete(face);if(measure()!==fallback)return 'delete kept stale face';
		document.fonts.add(face);if(measure()!==loaded)return 're-add failed';
		face.family='OtherEpochFace';if(measure()!==fallback)return 'descriptor kept stale face';
		face.family='GeometryEpochFace';if(measure()!==loaded)return 'descriptor restore failed';
		document.fonts.clear();if(measure()!==fallback)return 'clear kept stale face';
		return 'ok'})()`)
		if err != nil || value != "ok" {
			t.Fatalf("same-job font geometry: %v %v", value, err)
		}
	})
}
