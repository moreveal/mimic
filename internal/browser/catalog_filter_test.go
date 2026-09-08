package browser

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestSelectedCatalogPreservesExposedDescriptors(t *testing.T) {
	bundle := chrome152.New()
	full := *bundle.Surface()
	quoted, _ := json.Marshal(full.GeneratedCatalogJSON)
	// Correctness-only control: feed the original catalog directly, bypassing
	// the filter while retaining identical generator and exposure semantics.
	full.GeneratedJavaScript = strings.Replace(full.GeneratedJavaScript, "JSON.parse(host.catalogJSON())", "JSON.parse("+string(quoted)+")", 1)
	full.GeneratedCatalogJSON = ""
	control := surfaceTestBundle{Bundle: bundle, surface: full}
	const snapshot = `JSON.stringify(Reflect.ownKeys(globalThis).filter(k=>typeof k==='string'&&!k.startsWith('__')).map(k=>{
 const shape=d=>({enumerable:d.enumerable,configurable:d.configurable,writable:d.writable,type:'value'in d?typeof d.value:'accessor',name:typeof d.value==='function'?d.value.name:undefined,length:typeof d.value==='function'?d.value.length:undefined,primitive:typeof d.value==='string'||typeof d.value==='number'||typeof d.value==='boolean'?d.value:undefined,get:d.get&&[d.get.name,d.get.length],set:d.set&&[d.set.name,d.set.length]});
 const d=Object.getOwnPropertyDescriptor(globalThis,k),ctor=d.value,p=typeof ctor==='function'&&ctor.prototype;
 return [k,shape(d),p?Reflect.ownKeys(p).map(n=>[String(n),shape(Object.getOwnPropertyDescriptor(p,n))]):null,p&&Object.getPrototypeOf(p)&&Object.getPrototypeOf(p).constructor.name];
}))`
	get := func(b *Browser) string {
		c := b.NewContext()
		defer c.Close()
		p, e := c.NewPage()
		if e != nil {
			t.Fatal(e)
		}
		v, e := p.Evaluate(context.Background(), snapshot)
		if e != nil {
			t.Fatal(e)
		}
		return v.(string)
	}
	before, e := New(v8engine.Factory{}, control)
	if e != nil {
		t.Fatal(e)
	}
	after, e := New(v8engine.Factory{}, bundle)
	if e != nil {
		t.Fatal(e)
	}
	want, got := get(before), get(after)
	if want != got {
		_ = os.MkdirAll("../../.build", 0755)
		_ = os.WriteFile("../../.build/catalog-control-snapshot.json", []byte(want), 0644)
		_ = os.WriteFile("../../.build/catalog-filtered-snapshot.json", []byte(got), 0644)
		t.Fatal("catalog filtering changed descriptors; snapshots saved under .build")
	}
}
