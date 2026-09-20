package browser

import (
	"context"
	"testing"
	"time"
)

func TestGeometryTextMetricsTrackFontCollection(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `(async()=>{
		 document.body.innerHTML='<span style="display:inline-block;font:20px CacheFace,Arial">iiiiWWAV</span>';
		 const element=document.body.firstChild,measure=()=>element.getBoundingClientRect().width;
		 const fallback=measure(),face=new FontFace('CacheFace','local("Courier New")');
		 // Unrelated DOM mutations rebuild geometry, not identical shaping data.
		 for(let i=0;i<5;i++){element.dataset.revision=String(i);if(measure()!==fallback)return 'mutation'}
		 document.fonts.add(face);if(measure()!==fallback)return 'unloaded';
		 await face.load();const loaded=measure();if(loaded===fallback||measure()!==loaded)return 'load';
		 document.fonts.delete(face);if(measure()!==fallback)return 'delete';
		 document.fonts.add(face);if(measure()!==loaded)return 'add';
		 face.family='OtherFace';if(measure()!==fallback)return 'descriptor';
		 face.family='CacheFace';if(measure()!==loaded)return 'restore';
		 document.fonts.clear();if(measure()!==fallback)return 'clear';
		 element.style.fontSize='40px';if(measure()<=fallback)return 'size';
		 return 'ok';
		})()`)
		if err != nil || value != "ok" {
			t.Fatalf("geometry font cache: %v %v", value, err)
		}
	})
}
