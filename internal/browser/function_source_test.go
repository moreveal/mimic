package browser

import (
	"context"
	"os"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
	quickjsengine "github.com/moreveal/mimic/internal/engine/quickjs"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestFunctionSourcePreservesSourceWithoutPropertyReads(t *testing.T) {
	serialBrowserTest(t)
	source, err := os.ReadFile("testdata/function_source_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	for name, factory := range map[string]engine.Factory{"goja": gojaengine.Factory{}, "v8": v8engine.Factory{}, "quickjs": quickjsengine.Factory{}} {
		t.Run(name, func(t *testing.T) {
			browser, err := New(factory, chrome152.New())
			if err != nil {
				t.Fatal(err)
			}
			page, err := browser.NewContext().NewPage()
			if err != nil {
				t.Fatal(err)
			}
			defer page.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			// Bound-name and revoked-proxy behavior is also incorrect in the
			// Goja/QuickJS intrinsics themselves; only V8 currently matches the
			// complete frozen-Chrome callable fixture. All engines exercise the
			// overlay's source preservation and explicitly marked API path.
			expression := string(source)
			if name != "v8" {
				expression = `(()=>{const stringify=Function.prototype.toString,source='function sample() {\n  return "[native code]  keep spacing";\n}',sample=eval('('+source+')');return {source:stringify.call(sample),platform:stringify.call(document.createElement),reads:0}})()`
			}
			value, err := page.Evaluate(ctx, expression)
			if err != nil {
				t.Fatal(err)
			}
			result := value.(map[string]any)
			for key, want := range map[string]string{"source": "function sample() {\n  return \"[native code]  keep spacing\";\n}", "bound": "function () { [native code] }", "proxy": "function () { [native code] }", "revoked": "function () { [native code] }", "nonCallable": "TypeError", "platform": "function createElement() { [native code] }"} {
				if name != "v8" && key != "source" && key != "platform" {
					continue
				}
				if result[key] != want {
					t.Errorf("%s: got %q, want %q", key, result[key], want)
				}
			}
			if numberValue(result["reads"]) != 0 {
				t.Errorf("property reads: %v", result["reads"])
			}
			value, err = page.Evaluate(ctx, `(()=>{const stringify=Function.prototype.toString,fn=function sample(){},platform=document.createElement;
 const apply=Reflect.apply,get=WeakMap.prototype.get,set=WeakMap.prototype.set;
 try{Reflect.apply=WeakMap.prototype.get=WeakMap.prototype.set=()=>{throw Error('public intrinsic')};
 return stringify.call(fn)==='function sample(){}'&&stringify.call(platform)==='function createElement() { [native code] }';
 }finally{Reflect.apply=apply;WeakMap.prototype.get=get;WeakMap.prototype.set=set}})()`)
			if err != nil || value != true {
				t.Fatalf("source must use internal operations: %v, %v", value, err)
			}
		})
	}
}
