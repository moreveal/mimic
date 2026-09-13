//go:build (windows || linux) && amd64

package v8

import (
	"context"
	"testing"

	"github.com/moreveal/mimic/internal/engine"
)

func TestInterceptedObjectCanReplaceRealmDescriptors(t *testing.T) {
	r := (Factory{}).New()
	defer r.Close()
	_ = r.Set("makeWindow", r.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return r.(*adapter).NewInterceptedObject(args[0])
	}))
	value, err := r.Eval(context.Background(), `(()=>{
 let current={};Object.defineProperty(current,'fixed',{value:17,configurable:false});
 const sentinel={},x=makeWindow({
  get(k){if(k==='throws')throw sentinel;return [true,current[k]]},
  ownKeys(){return Reflect.ownKeys(current)},
  getOwnPropertyDescriptor(k){const d=Reflect.getOwnPropertyDescriptor(current,k);return d?[true,d]:[false]}
 });
 const checks=[typeof x==='object',!!x,x!=null,Object.getOwnPropertyDescriptor(x,'fixed').configurable===false,x.fixed===17];
 try{x();checks.push(false)}catch(e){checks.push(e instanceof TypeError)}
 try{x.throws;checks.push(false)}catch(e){checks.push(e===sentinel)}
 current={replacement:29};checks.push(Object.getOwnPropertyDescriptor(x,'fixed')===undefined,Reflect.ownKeys(x).join()==='replacement',x.replacement===29);
 return checks.every(Boolean);
})()`, "intercepted-object.js")
	if err != nil {
		t.Fatal(err)
	}
	if value.Export() != true {
		t.Fatalf("native intercepted object: %v", value.Export())
	}
}
