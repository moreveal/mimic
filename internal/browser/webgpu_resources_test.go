package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestWebGPUResourceOracle(t *testing.T) {
	for _, name := range []string{"resources", "flags", "lifecycle", "navigator"} {
		t.Run(name, func(t *testing.T) { webGPUOracle(t, name) })
	}
}
func webGPUOracle(t *testing.T, name string) {
	source, err := os.ReadFile("testdata/webgpu_" + name + "_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/webgpu-" + name + "-chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var capture struct {
		Result map[string]any `json:"result"`
	}
	if err = json.Unmarshal(data, &capture); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		for _, worker := range []bool{false, true} {
			expression := "(async()=>JSON.stringify(await " + string(source) + "))()"
			if worker {
				code := "onmessage=async()=>{try{postMessage(await " + expression + ")}catch(e){postMessage(String(e))}}"
				expression = `new Promise((resolve,reject)=>{const u=URL.createObjectURL(new Blob([` + strconv.Quote(code) + `])),w=new Worker(u);w.onmessage=e=>{w.terminate();URL.revokeObjectURL(u);resolve(e.data)};w.onerror=e=>reject(Error(e.message));w.postMessage(null)})`
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			value, err := p.Evaluate(ctx, expression)
			cancel()
			if err != nil {
				t.Fatalf("worker=%v: %v", worker, err)
			}
			var actual map[string]any
			if err = json.Unmarshal([]byte(value.(string)), &actual); err != nil {
				t.Fatalf("worker=%v: %v (%v)", worker, value, err)
			}
			for key, want := range capture.Result {
				if !reflect.DeepEqual(actual[key], want) {
					t.Errorf("worker=%v %s: got %v want %v", worker, key, actual[key], want)
				}
			}
		}
	})
}

func TestWebGPUShaderCompilationIsSeparateFromExecution(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		v, err := p.Evaluate(context.Background(), `(async()=>{const a=await navigator.gpu.requestAdapter(),d=await a.requestDevice();const valid=d.createShaderModule({code:'@compute @workgroup_size(1) fn main() {}'}),invalid=d.createShaderModule({code:'fn broken('}),validInfo=await valid.getCompilationInfo(),invalidInfo=await invalid.getCompilationInfo();let unsupported=false;try{d.createTexture({size:[2,2],format:'rgba8unorm',sampleCount:4,usage:GPUTextureUsage.RENDER_ATTACHMENT})}catch(e){unsupported=e.name==='NotSupportedError'}d.destroy();return validInfo.messages.length===0&&invalidInfo.messages.length===1&&invalidInfo.messages[0].type==='error'&&unsupported})()`)
		if err != nil || v != true {
			t.Fatalf("%v %v", v, err)
		}
	})
}
