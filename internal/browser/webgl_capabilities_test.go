package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func TestWebGLCapabilityQueriesMatchChrome(t *testing.T) {
	parallelBrowserTest(t)
	fixture, err := os.ReadFile("testdata/webgl_capabilities_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/webgl-capabilities-chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var capture struct {
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(data, &capture); err != nil {
		t.Fatal(err)
	}
	// This batch implements capability observations for a bounded extension set.
	// Advertising every native extension would promise unsupported operations.
	for _, value := range capture.Result {
		delete(value.(map[string]any), "extensions")
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		for _, worker := range []bool{false, true} {
			expression := "JSON.stringify(" + string(fixture) + ")"
			if worker {
				code := "onmessage=()=>{try{postMessage(" + expression + ")}catch(e){postMessage(String(e))}}"
				expression = `new Promise((resolve,reject)=>{const u=URL.createObjectURL(new Blob([` + strconv.Quote(code) + `])),w=new Worker(u);w.onmessage=e=>{w.terminate();URL.revokeObjectURL(u);resolve(e.data)};w.onerror=e=>reject(Error(e.message));w.postMessage(null)})`
			}
			value, err := p.Evaluate(context.Background(), expression)
			if err != nil {
				t.Fatal(err)
			}
			var actual map[string]any
			if err := json.Unmarshal([]byte(value.(string)), &actual); err != nil {
				t.Fatal(err)
			}
			for kind, value := range actual {
				entry := value.(map[string]any)
				delete(entry, "extensions")
				for key, got := range entry {
					want := capture.Result[kind].(map[string]any)[key]
					if !reflect.DeepEqual(got, want) {
						t.Errorf("%s.%s differs: got %v want %v", kind, key, got, want)
					}
				}
			}
		}
	})
}

func TestWebGLCapabilityStateAndCopies(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), `(()=>{const a=new OffscreenCanvas(2,2).getContext('webgl2',{alpha:false,depth:false,stencil:true}),b=new OffscreenCanvas(2,2).getContext('webgl2');let range=a.getParameter(a.MAX_VIEWPORT_DIMS);range[0]=0;if(a.getParameter(a.MAX_VIEWPORT_DIMS)[0]===0)return 'shared range';let samples=a.getInternalformatParameter(a.RENDERBUFFER,a.RGBA8,a.SAMPLES);samples[0]=0;if(a.getInternalformatParameter(a.RENDERBUFFER,a.RGBA8,a.SAMPLES)[0]===0)return 'shared samples';a.stencilMaskSeparate(a.FRONT,17);if(a.getParameter(a.STENCIL_WRITEMASK)!==17||a.getParameter(a.STENCIL_BACK_WRITEMASK)!==4294967295||b.getParameter(b.STENCIL_WRITEMASK)!==4294967295)return 'stencil state';if(a.getParameter(a.ALPHA_BITS)!==0||a.getParameter(a.DEPTH_BITS)!==0||a.getParameter(a.STENCIL_BITS)!==8)return 'attributes';const e=a.getExtension('EXT_texture_filter_anisotropic');if(e!==a.getExtension('ext_texture_filter_anisotropic')||b.getParameter(34047)!==null||b.getError()!==1280)return 'extension isolation';a.hint(a.FRAGMENT_SHADER_DERIVATIVE_HINT,a.NICEST);if(a.getParameter(a.FRAGMENT_SHADER_DERIVATIVE_HINT)!==a.NICEST)return 'hint state';return true})()`)
		if err != nil || value != true {
			t.Fatalf("%v %v", value, err)
		}
	})
}
