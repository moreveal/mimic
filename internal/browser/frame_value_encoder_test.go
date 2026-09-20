package browser

import (
	"context"
	"testing"
)

// Shared with the retained Chrome probe: walk remote prototypes, read a
// property and call a function. The checksum also detects missing iterations.
const frameValueEncoderProbe = `(()=>{const f=document.createElement('iframe');document.body.appendChild(f);try{const w=f.contentWindow;w.eval("globalThis.chain=Object.create(Object.create(Object.create(null)));globalThis.fn=x=>x+1;globalThis.obj={x:7}");let checks=0;for(let i=0;i<100;i++){let p=w.chain;while(p!==null){checks++;p=Object.getPrototypeOf(p)}checks+=w.obj.x;checks+=w.fn(i)}return checks}finally{f.remove()}})()`

func TestFrameValueEncoderPreservesRepeatedOperations(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), frameValueEncoderProbe)
		if err != nil || numberValue(value) != 6050 {
			t.Fatalf("bridge checksum: %v %v", value, err)
		}
	})
}
func TestFrameValueEncoderReflectsPrototypeChanges(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), `(()=>{const f=document.createElement('iframe');document.body.appendChild(f);const w=f.contentWindow;w.eval("globalThis.a={};globalThis.b={};globalThis.object=Object.create(a)");const object=w.object;const a=Object.getPrototypeOf(object);w.eval("Object.setPrototypeOf(object,b)");const b=Object.getPrototypeOf(object);const okay=a===w.a&&b===w.b&&a!==b;f.remove();return okay})()`)
		if err != nil || value != true {
			t.Fatalf("live prototype: %v %v", value, err)
		}
	})
}

func TestFrameValueEncoderPreservesSpecialNumbersAndPrivateIntrinsics(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), `(()=>{const f=document.createElement('iframe');document.body.appendChild(f);const w=f.contentWindow;try{
 w.eval("globalThis.nan=NaN;globalThis.inf=Infinity;globalThis.neg=-Infinity;globalThis.zero=-0;globalThis.value={};JSON.stringify=()=>{throw Error('poisoned stringify')};Object.prototype.toJSON=()=>{throw Error('poisoned toJSON')};Array.prototype[Symbol.iterator]=()=>{throw Error('poisoned iterator')}");
 return Number.isNaN(w.nan)&&w.inf===Infinity&&w.neg===-Infinity&&Object.is(w.zero,-0)&&w.value===w.value;
 }finally{f.remove()}})()`)
		if err != nil || value != true {
			t.Fatalf("private encoding: %v %v", value, err)
		}
	})
}

func TestFrameValueEncoderDoesNotCacheRevokedArrayShape(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), `(()=>{const f=document.createElement('iframe');document.body.appendChild(f);const w=f.contentWindow;try{
 w.eval("globalThis.rev=Proxy.revocable([],{});globalThis.array=rev.proxy");const first=w.array;
 w.eval("rev.revoke()");try{void w.array;return false}catch(e){return true}
 }finally{f.remove()}})()`)
		if err != nil || value != true {
			t.Fatalf("revoked array: %v %v", value, err)
		}
	})
}
