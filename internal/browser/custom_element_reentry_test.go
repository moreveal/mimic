package browser

import (
	"context"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// Measured against Chrome 152: an attribute change during one connected
// callback must not recursively invoke the connected callbacks of siblings.
func TestCustomElementNestedReactionsDoNotDrainSiblingQueue(t *testing.T) {
	parallelBrowserTest(t)
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(context.Background(), `(()=>{
 const log=[];customElements.define('reaction-probe',class extends HTMLElement{
 static observedAttributes=['data-value'];
 connectedCallback(){log.push('start'+this.id);this.setAttribute('data-value','x');log.push('end'+this.id)}
 attributeChangedCallback(){log.push('attr'+this.id)}
 });
 const f=document.createDocumentFragment();for(let i=0;i<3;i++){const e=document.createElement('reaction-probe');e.id=String(i);f.append(e)}
 document.body.append(f);return log.join(',');
})()`)
	const want = "start0,attr0,end0,start1,attr1,end1,start2,attr2,end2"
	if err != nil || value != want {
		t.Fatalf("nested reaction order: %v %v", value, err)
	}
}
