package browser

import (
	"context"
	"testing"
)

func TestDialogCommandActivationMatchesChrome152(t *testing.T) {
	// Captured from Chrome 152.0.7977.83: even synthetic click activation
	// dispatches a trusted, composed, cancelable, non-bubbling command event.
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		got, err := p.Evaluate(context.Background(), `(()=>{
document.body.innerHTML='<button id="b" command="show-modal" commandfor="d"><span id="s">open</span></button><dialog id="d">Hello</dialog>';
const b=document.getElementById('b'),d=document.getElementById('d'),out=[];
d.addEventListener('command',e=>out.push([e.command,e.source===b,e.bubbles,e.cancelable,e.composed,e.isTrusted]));
document.getElementById('s').click();out.push([d.open,d.matches(':modal'),b.command,b.commandForElement===d]);
b.command='close';b.click();out.push(d.open);b.command='INVALID';out.push(b.command);b.command='--custom';b.click();
b.command='show-modal';d.addEventListener('command',e=>e.preventDefault(),{once:true});b.click();out.push(d.open);
const e=new CommandEvent('command',{command:'--x',source:b});out.push([e.command,e.source===b,e.bubbles,e.cancelable]);return JSON.stringify(out)
})()`)
		const want = `[["show-modal",true,false,true,true,true],[true,true,"show-modal",true],["close",true,false,true,true,true],false,"",["--custom",true,false,true,true,true],["show-modal",true,false,true,true,true],false,["--x",true,false,false]]`
		if err != nil || got != want {
			t.Fatalf("command: %v %v", got, err)
		}
	})
}

func TestClosedDialogGeometryDoesNotMeasureDescendants(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		got, err := p.Evaluate(context.Background(), `(()=>{
const d=document.createElement('dialog');d.innerHTML='<button style="font-size:2ex">unsupported text metric</button>';document.body.append(d);
const button=d.firstChild;
const out=[getComputedStyle(d).display,d.getBoundingClientRect().width,button.getBoundingClientRect().width];
button.style.fontSize='16px';d.showModal();out.push(getComputedStyle(d).display,button.getBoundingClientRect().width>0);d.close();out.push(button.getBoundingClientRect().width);
return JSON.stringify(out)
})()`)
		if err != nil || got != `["none",0,0,"block",true,0]` {
			t.Fatalf("dialog geometry: %v %v", got, err)
		}
	})
}

func TestControlFontVariablesUseResolvedMetrics(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		got, err := p.Evaluate(context.Background(), `(()=>{
document.body.innerHTML='<div style="font-size:20px;--size:18px"><button id="a" style="font-size:var(--missing,var(--size))">Example</button><button id="b" style="font-size:18px">Example</button></div>';
const a=document.getElementById('a'),b=document.getElementById('b'),out=[a.getBoundingClientRect().width===b.getBoundingClientRect().width];
a.parentElement.style.setProperty('--size','24px');b.style.fontSize='24px';out.push(a.getBoundingClientRect().width===b.getBoundingClientRect().width);
a.style.fontSize='var(--absent)';b.style.fontSize='20px';out.push(a.getBoundingClientRect().width===b.getBoundingClientRect().width);
a.style.fontSize='0px';b.style.fontSize='0px';out.push(a.getBoundingClientRect().width===b.getBoundingClientRect().width);
return JSON.stringify(out)
})()`)
		if err != nil || got != `[true,true,true,true]` {
			t.Fatalf("font variable: %v %v", got, err)
		}
	})
}

func TestControlGeometryResolvesCalcFontSizeProducts(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		got, err := p.Evaluate(context.Background(), `(()=>{
document.documentElement.style.cssText='--scale:1;font-size:calc(100%*var(--scale, 1))';
document.body.innerHTML='<button id="product" style="font-size:1rem">Example</button><button id="explicit" style="font-size:16px">Example</button>';
const product=document.getElementById('product').getBoundingClientRect(),explicit=document.getElementById('explicit').getBoundingClientRect();
return [getComputedStyle(document.documentElement).fontSize,product.width,product.height,product.width===explicit.width&&product.height===explicit.height]
})()`)
		values, ok := got.([]any)
		if err != nil || !ok || len(values) != 4 || values[0] != "16px" || values[3] != true {
			t.Fatalf("calc product control geometry: %#v %v", got, err)
		}
	})
}
