package browser

import (
	"context"
	"testing"
)

func TestWidthDimensionProjectionMatchesFrozenChrome152(t *testing.T) {
	testCSSObservation(t, "css_width_projection")
}

// A fixed-width box remains observable correctly with unrelated sibling flow.
func TestWidthObservationKeepsUnrelatedDocumentFlowCorrect(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		value, err := p.Evaluate(context.Background(), `(()=>{
		 document.body.innerHTML='<div id="target" style="width:120px;padding:3px;border:2px solid"></div><div style="width:300px">Unrelated sibling text whose line height belongs to the rectangle observation.</div>';
		 const target=document.getElementById('target');globalThis.widthTarget=target;
		 return [target.offsetWidth,target.clientWidth,getComputedStyle(target).width].join(',');
		})()`)
		if err != nil || value != "130,126,120px" {
			t.Fatalf("width observation: %v %v", value, err)
		}
		value, err = p.Evaluate(context.Background(), `String(widthTarget.getBoundingClientRect().width)`)
		if err != nil || value != "130" {
			t.Fatalf("rectangle: %v %v", value, err)
		}
	})
}
