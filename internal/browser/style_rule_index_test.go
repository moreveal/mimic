package browser

import (
	"context"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// Mutation observations verified against Chrome 152.0.7977.83.
func TestStyleRuleIndexRevalidatesCanonicalState(t *testing.T) {
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
	got, err := p.Evaluate(context.Background(), `(()=>{
const style=document.createElement('style');style.textContent='* { margin-top: 1px } div { margin-top: 2px } .target { margin-top: 3px } section > .target { margin-top: 4px } [id="BOX" i] { margin-top: 5px } :not(.absent).target { margin-top: 6px }';document.head.append(style);
const parent=document.createElement('section'),node=document.createElement('div');node.id='box';node.className='target';parent.append(node);document.body.append(parent);
const seen=[],read=()=>seen.push(getComputedStyle(node).marginTop);read();
style.sheet.insertRule('#box { margin-top: 7px }',style.sheet.cssRules.length);read();
style.sheet.cssRules[6].style.marginTop='8px';read();
style.sheet.cssRules[6].selectorText='#other';read();
node.className='unmatched';read();node.removeAttribute('id');read();
const extra=document.createElement('style');extra.textContent='div { margin-top: 9px }';document.head.append(extra);read();
extra.sheet.disabled=true;read();extra.sheet.disabled=false;read();
extra.setAttribute('media','not all');read();extra.setAttribute('media','all');read();
extra.textContent='div { margin-top: 10px }';read();extra.remove();read();
const sheet=new CSSStyleSheet();sheet.replaceSync('div { margin-top: 11px }');document.adoptedStyleSheets=[sheet];read();sheet.replaceSync('div { margin-top: 12px }');read();document.adoptedStyleSheets=[];read();
const shadowHost=document.createElement('aside');document.body.append(shadowHost);const root=shadowHost.attachShadow({mode:'open'});root.innerHTML='<style>div { margin-top: 13px }</style><div></div>';const child=root.querySelector('div');seen.push(getComputedStyle(child).marginTop);root.adoptedStyleSheets=[sheet];seen.push(getComputedStyle(child).marginTop);sheet.cssRules[0].style.marginTop='14px';seen.push(getComputedStyle(child).marginTop);
return seen.join(',');
})()`)
	const want = "6px,7px,8px,6px,5px,2px,9px,2px,9px,2px,9px,10px,2px,11px,12px,2px,13px,12px,14px"
	if err != nil || got != want {
		t.Fatalf("style mutations: %v, %v; want %s", got, err, want)
	}
}
