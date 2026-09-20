package browser

import (
	"context"
	"testing"
)

func TestFrameReflectionUsesCapturedIntrinsics(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, page *Page) {
		value, err := page.Evaluate(context.Background(), `(()=>{
 const frame=document.createElement('iframe');document.body.appendChild(frame);const child=frame.contentWindow;
 const object=child.eval('(()=>{const object={answer:42};object[Symbol.for("")]=19;Object.defineProperty(object,Symbol.toPrimitive,{get(){throw Error("coercion")}});return object})()');
 child.eval('Reflect.getOwnPropertyDescriptor=Reflect.ownKeys=Array.prototype.map=function(){throw Error("replaced intrinsic")};Array.prototype[Symbol.iterator]=function(){throw Error("replaced iterator")}');
 return Object.getOwnPropertyDescriptor(object,'answer').value===42 && Reflect.ownKeys(object).includes(Symbol.for('')) && object[Symbol.for('')]===19;
 })()`)
		if err != nil || value != true {
			t.Fatalf("captured reflection intrinsics: %v, %v", value, err)
		}
	})
}
