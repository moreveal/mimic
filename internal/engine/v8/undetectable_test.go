//go:build (windows || linux) && amd64

package v8

import (
	"context"
	"testing"

	"github.com/moreveal/mimic/internal/engine"
)

func TestHTMLDDAObject(t *testing.T) {
	r := (Factory{}).New()
	defer r.Close()
	_ = r.Set("makeDDA", r.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return r.(*adapter).NewUndetectableObject(args[0])
	}))
	source := `const handler={
 call(args,construct){if(construct)throw new TypeError("no constructor");return args[0]+1},
 get(k){if(k==="error")throw globalThis.sentinel;return k==="0"?[true,42]:[false]},
 getOwnPropertyDescriptor(k){return k==="0"?[true,{value:42,writable:false,enumerable:true,configurable:true}]:[false]},
 ownKeys(){return ["0"]},
 set(k,v){return k==="0"?[true,false]:[false]},
 deleteProperty(k){return k==="0"?[true,false]:[false]},
 defineProperty(k,d){if(k==="capture"){globalThis.captured=d;return [true,true]}return k==="0"?[true,false]:[false]}
 }; const x=makeDDA(handler);
 const checks=[typeof x==="undefined",!x,x==null,x!==undefined,x!==null,x(2)===3,x[0]===42,"0" in x,Reflect.ownKeys(x).join()==="0",Object.getOwnPropertyDescriptor(x,"0").value===42];
 x.extra=5;checks.push(x.extra===5,delete x.extra,x.extra===undefined,Reflect.deleteProperty(x,"0")===false);
 Object.setPrototypeOf(x,{foo:7,[Symbol.toStringTag]:"HTMLAllCollection"});checks.push(x.foo===7,Object.prototype.toString.call(x)==="[object HTMLAllCollection]");
 try{new x();checks.push(false)}catch(e){checks.push(e instanceof TypeError)};
 const getter=()=>9;const setter=v=>{};Object.defineProperty(x,"capture",{get:getter,set:setter,enumerable:true,configurable:true});checks.push(captured.get===getter,captured.set===setter,captured.enumerable,captured.configurable);
 checks.push(Reflect.set(x,"0",5)===false,Reflect.defineProperty(x,"0",{value:5})===false);
 globalThis.sentinel={reason:"identity"};try{x.error;checks.push(false)}catch(e){checks.push(e===sentinel)};
 try{(()=>{"use strict";x[0]=5})();checks.push(false)}catch(e){checks.push(e instanceof TypeError)};
 const symbol=Symbol("custom");const sym=makeDDA({get(k){return k===symbol?[true,17]:[false]},ownKeys(){return [symbol]},getOwnPropertyDescriptor(k){return k===symbol?[true,{value:17,writable:true,enumerable:true,configurable:true}]:[false]}});checks.push(sym[symbol]===17,Reflect.ownKeys(sym)[0]===symbol,Object.keys(sym).length===0,Object.getOwnPropertySymbols(sym)[0]===symbol);
 const throws=makeDDA({get get(){throw sentinel}});try{throws.any;checks.push(false)}catch(e){checks.push(e===sentinel)};
 checks.every(Boolean)`
	got, err := r.Eval(context.Background(), source, "dda.js")
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Set("nativeTypeOf", r.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return r.Value(r.TypeOf(args[0])), nil
	}))
	callbackType, err := r.Eval(context.Background(), "nativeTypeOf(x)", "dda-callback-typeof.js")
	if err != nil || callbackType.Export() != "undefined" {
		t.Fatalf("callback typeof: %v %v", callbackType, err)
	}
	x, err := r.Eval(context.Background(), "x", "dda-native-typeof.js")
	if err != nil {
		t.Fatal(err)
	}
	if typ := r.TypeOf(x); typ != "undefined" {
		t.Fatalf("adapter typeof = %q", typ)
	}
	if got.Export() != true {
		t.Fatalf("HTMLDDA check failed: %v", got.Export())
	}
}

func TestHTMLDDAEnumeration(t *testing.T) {
	r := (Factory{}).New()
	defer r.Close()
	_ = r.Set("makeDDA", r.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return r.(*adapter).NewUndetectableObject(args[0])
	}))
	got, err := r.Eval(context.Background(), `const x=makeDDA({nonMasking:true,get(k){return k==="hidden"?[true,4]:[false]},ownKeys(){return ["hidden","item"]},getOwnPropertyDescriptor(k){return k==="hidden"?[true,{value:4,enumerable:false,writable:true,configurable:true}]:[false]}});Object.setPrototypeOf(x,{item:8});const keys=[];for(const k in x)keys.push(k);JSON.stringify({own:Reflect.ownKeys(x),keys:Object.keys(x),hasOwn:Object.hasOwn(x,"item"),descriptor:Object.getOwnPropertyDescriptor(x,"item"),forIn:keys})`, "dda-enum.js")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"own":["hidden","item"],"keys":[],"hasOwn":false,"forIn":["item"]}`
	if got.Export() != want {
		t.Fatalf("enumeration = %s", got.Export())
	}
	value, err := r.Eval(context.Background(), `let added=false;const late=makeDDA({nonMasking:true,get(k){return added&&k==="future"?[true,"element"]:[false]}});late.future="expando";added=true;late.future==="expando"`, "dda-expando.js")
	if err != nil || value.Export() != true {
		t.Fatalf("expando precedence: %v %v", value, err)
	}
}
