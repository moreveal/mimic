package browser

import (
	"context"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// Browser exposure shaping must not add brands, lock prototype properties,
// or change descriptors on ECMAScript objects supplied by the engine.
func TestBrowserPreservesIntrinsicPrototypeDescriptors(t *testing.T) {
	parallelBrowserTest(t)
	const snapshot = `JSON.stringify(['Object','Function','Array','RegExp','String','Number','Boolean','Promise','Map','Set','WeakMap','WeakSet','ArrayBuffer','DataView','Uint8Array','Float64Array'].map(name=>{
 const ctor=globalThis[name],p=ctor.prototype;
 const describe=d=>({enumerable:d.enumerable,configurable:d.configurable,writable:d.writable,type:'value'in d?typeof d.value:'accessor',name:typeof d.value==='function'?d.value.name:undefined,length:typeof d.value==='function'?d.value.length:undefined,tag:typeof d.value==='string'?d.value:undefined,get:d.get&&[d.get.name,d.get.length],set:d.set&&[d.set.name,d.set.length]});
 return [name,describe(Object.getOwnPropertyDescriptor(ctor,'prototype')),Reflect.ownKeys(p).map(key=>[String(key),describe(Object.getOwnPropertyDescriptor(p,key))])];
}))`
	for name, factory := range map[string]engine.Factory{"goja": gojaengine.Factory{}, "v8": v8engine.Factory{}} {
		t.Run(name, func(t *testing.T) {
			bare := factory.New()
			defer bare.Close()
			want, err := bare.Eval(context.Background(), snapshot, "intrinsics-snapshot.js")
			if err != nil {
				t.Fatal(err)
			}
			b, err := New(factory, chrome152.New())
			if err != nil {
				t.Fatal(err)
			}
			c := b.NewContext()
			defer c.Close()
			p, err := c.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			got, err := p.Evaluate(context.Background(), snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if got != want.String() {
				t.Fatalf("intrinsic descriptors changed:\nwant %s\ngot %s", want.String(), got)
			}
			value, err := p.Evaluate(context.Background(), `(()=>{
 class A extends Array{};const a=new A(1,2,3),s=a.slice(1);if(!(s instanceof A)||s.join(',')!=='2,3')return false;
 class R extends RegExp{};if('a b'.split(new R(' ')).join(',')!=='a,b')return false;
 const before=Object.prototype.toString.call(/x/);const x=document.createElement('div');
 return before==='[object RegExp]'&&Object.prototype.toString.call(x)==='[object HTMLDivElement]'&&!Object.prototype.hasOwnProperty.call(Object.prototype,Symbol.toStringTag);
})()`)
			if err != nil || value != true {
				t.Fatalf("species/brands: %v %v", value, err)
			}
		})
	}
}
