package browser

import (
	"context"
	"testing"
)

func TestFrameConstructionPreservesRealmAndNewTarget(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		value, err := page.Evaluate(context.Background(), `(()=>{
 const frame=document.createElement('iframe');document.body.appendChild(frame);const child=frame.contentWindow;
 const array=new child.Array(2,3);if(array.length!==2||array[1]!==3||Object.getPrototypeOf(array)!==child.Array.prototype)return 'native '+array.length+':'+array[1]+':'+(Object.getPrototypeOf(array)===child.Array.prototype);
 if(!Array.isArray(array))return 'array brand';
 const arrow=child.eval('()=>{}');let arrowRejected=false;try{new arrow()}catch(e){arrowRejected=e instanceof TypeError}if(!arrowRejected)return 'arrow constructor';
 child.eval('globalThis.C=class C{constructor(value){this.value=value;this.target=new.target}};globalThis.D=class D extends C{}');
 const C=child.C,D=child.D,object=new C(array);
 if(object.value!==array||object.target!==C||Object.getPrototypeOf(object)!==C.prototype)return 'identity';
 const derived=Reflect.construct(C,[17],D);
 if(derived.value!==17||derived.target!==D||Object.getPrototypeOf(derived)!==D.prototype)return 'newTarget';
 child.eval('Reflect.construct=function(){throw Error("patched") }');
 if(new C(19).value!==19)return 'captured intrinsic';
 const Return=child.eval('(function(value){return value})');if(new Return(array)!==array)return 'returned identity';
 child.eval('globalThis.failure=new Error("constructor failure");globalThis.Bad=function(){throw failure};globalThis.Primitive=function(){throw 19}');
 let threw=false;try{new child.Bad()}catch(e){threw=e===child.failure}if(!threw)return 'exception identity';
 threw=false;try{new child.Primitive()}catch(e){threw=e===19}if(!threw)return 'exception primitive';
 if(new C(123n).value!==123n)return 'bigint';
 const symbol=Symbol('local');if(new C(symbol).value!==symbol)return 'symbol';
 let reads=0;const local={get value(){reads++;return 1}};
 for(const operation of [()=>Reflect.construct(C,[],function Local(){})]){let blocked=false;try{operation()}catch(e){blocked=e.name==='NotSupportedError'}if(!blocked)return 'unsupported newTarget/argument'}
 if(new C(local).value!==local||reads!==0)return 'local argument identity';
 return true;
 })()`)
		if err != nil || value != true {
			t.Fatalf("remote construction: %v, %v", value, err)
		}
	})
}
