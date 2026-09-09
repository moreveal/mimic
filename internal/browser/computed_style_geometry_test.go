package browser

import (
	"context"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestComputedStyleDoesNotInvokeAuthorGeometryGetters(t *testing.T) {
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
 const element=document.createElement('div');document.body.appendChild(element);
 element.style.cssText='opacity:0.5;width:20px;height:30px';
 for(const name of ['offsetWidth','offsetHeight'])Object.defineProperty(element,name,{get(){throw new Error('author geometry getter')}});
 const style=getComputedStyle(element),checks=[style.opacity==='0.5',style.width==='20px',style.height==='30px',style.length>0,typeof style.item(0)==='string'];
 element.style.opacity='0.75';element.style.width='40px';checks.push(style.opacity==='0.75',style.width==='40px');
 element.style.removeProperty('width');element.style.removeProperty('height');
 checks.push(typeof style.width==='string',typeof style.height==='string',style.getPropertyPriority('width')==='');
 return checks.every(Boolean)?'ok':JSON.stringify(checks);
 })()`)
	if err != nil || value != "ok" {
		t.Fatalf("computed style: %v %v", value, err)
	}
}
