package browser

import (
	"context"
	"encoding/json"
	"testing"
)

// Unsupported geometry must retain a useful failure rather than silently
// contribute a fabricated empty rectangle to an otherwise supported group.
func TestSVGBoundingBoxUnsupportedGeometry(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		value, err := p.Evaluate(context.Background(), `(()=>{
 const ns='http://www.w3.org/2000/svg',svg=document.createElementNS(ns,'svg');document.body.appendChild(svg);
 for(const tag of ['textPath','switch']){
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

func TestSVGTextObservations(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		value, err := p.Evaluate(context.Background(), `(()=>{
 const ns='http://www.w3.org/2000/svg',svg=document.createElementNS(ns,'svg'),text=document.createElementNS(ns,'text');
 svg.appendChild(text);document.body.appendChild(svg);
 const box=()=>{const b=text.getBBox();return [b.x,b.y,b.width,b.height]};
 text.textContent='AAAA';const a=box(),length=text.getComputedTextLength();
 if(!a.every(Number.isFinite)||a[2]<=0||a[3]<=0||length<=0)return 'invalid metrics';
 text.textContent='AAAB';const b=box();text.textContent='AAAA';
 if(JSON.stringify(a)!==JSON.stringify(box()))return 'unstable repeated observation';
 if(JSON.stringify(a)===JSON.stringify(b))return 'mutation ignored';
 text.setAttribute('x','10');text.setAttribute('y','20');const moved=box();
 if(moved[0]!==a[0]+10||moved[1]!==a[1]+20||moved[2]!==a[2]||moved[3]!==a[3])return 'position mismatch';
 text.textContent='';if(box().some(v=>v!==0)||text.getComputedTextLength()!==0)return 'empty mismatch';
 svg.remove();return 'ok';
})()`)
		if err != nil || value != "ok" {
			t.Fatalf("SVG text observations: %v, %v", value, err)
		}
	})
}

func TestCSSFontSizeFailureTrace(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		_, err := p.Evaluate(context.Background(), `(()=>{const ns='http://www.w3.org/2000/svg',s=document.createElementNS(ns,'svg'),n=document.createElementNS(ns,'text');s.appendChild(n);document.body.appendChild(s);n.textContent='A';n.setAttribute('font-size','2ex');try{n.getBBox()}catch(e){if(e.name!=='NotSupportedError')throw e}})()`)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range p.Trace().Events() {
			if event.Name != "CSS.fontSizeResolution" {
				continue
			}
			var detail map[string]any
			raw, ok := event.Data["detail"].(string)
			if !ok {
				t.Fatal("missing diagnostic detail")
			}
			if err := json.Unmarshal([]byte(raw), &detail); err != nil {
				t.Fatal(err)
			}
			if detail["value"] != "2ex" || detail["parentSize"] != float64(16) || detail["rootBasis"] != float64(16) || detail["reason"] != "unsupported-expression" {
				t.Fatalf("unexpected context: %v", detail)
			}
			return
		}
		t.Fatal("missing font-size failure context")
	})
}

func TestTextShapingFailureTrace(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		_, err := p.Evaluate(context.Background(), `(()=>{const ns='http://www.w3.org/2000/svg',s=document.createElementNS(ns,'svg'),n=document.createElementNS(ns,'text');document.body.appendChild(s);s.appendChild(n);n.textContent='diagnostic';n.setAttribute('font-size','5000px');try{n.getBBox()}catch(e){if(e.name!=='NotSupportedError')throw e}})()`)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range p.Trace().Events() {
			if e.Name == "Text.shapeFailure" {
				if e.Data["reason"] != "unsupported font size or weight" || e.Data["text"] != "diagnostic" || e.Data["size"] != float64(5000) {
					t.Fatalf("unexpected shaping context: %v", e.Data)
				}
				return
			}
		}
		t.Fatal("missing shaping failure context")
	})
}

func TestSVGCSSTransformBoundaryContext(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		v, err := p.Evaluate(context.Background(), `(()=>{const ns='http://www.w3.org/2000/svg',s=document.createElementNS(ns,'svg'),g=document.createElementNS(ns,'g'),r=document.createElementNS(ns,'rect');document.body.append(s);s.append(g);g.append(r);r.setAttribute('width','10');r.setAttribute('height','20');r.style.transform='rotateX(30deg)';try{g.getBBox();return 'missing error'}catch(e){return e.name}})()`)
		if err != nil || v != "NotSupportedError" {
			t.Fatalf("boundary: %v %v", v, err)
		}
		for _, e := range p.Trace().Events() {
			if e.Name == "SVG.cssTransformResolution" {
				detail, ok := e.Data["context"].(map[string]any)
				if !ok || detail["value"] != "rotateX(30deg)" || detail["reason"] != "unsupported-transform-function:rotatex" {
					t.Fatalf("missing context: %v", e.Data)
				}
				return
			}
		}
		t.Fatal("missing transform diagnostic")
	})
}
