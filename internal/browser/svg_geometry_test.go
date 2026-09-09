package browser

import (
	"context"
	"testing"
)

// Unsupported geometry must retain a useful failure rather than silently
// contribute a fabricated empty rectangle to an otherwise supported group.
func TestSVGBoundingBoxUnsupportedGeometry(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		value, err := p.Evaluate(context.Background(), `(()=>{
 const ns='http://www.w3.org/2000/svg',svg=document.createElementNS(ns,'svg');document.body.appendChild(svg);
 for(const tag of ['text','use','switch']){
  const g=document.createElementNS(ns,'g'),n=document.createElementNS(ns,tag);n.textContent='hello';g.appendChild(n);svg.appendChild(g);
  try{g.getBBox();return 'silently accepted '+tag}catch(e){if(e.name!=='NotSupportedError')return 'wrong error '+e.name}g.remove();
 }
 svg.remove();return 'ok';
})()`)
		if err != nil || value != "ok" {
			t.Fatalf("SVG unsupported boundary: %v, %v", value, err)
		}
	})
}
