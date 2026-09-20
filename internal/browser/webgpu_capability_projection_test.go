package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strconv"
	"testing"
)

func TestWebGPUCapabilityProjectionMatchesFrozenChrome152(t *testing.T) {
	parallelBrowserTest(t)
	source, err := os.ReadFile("testdata/webgpu_capability_projection_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/webgpu_capability_projection_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected struct {
		Observation map[string]any `json:"observation"`
	}
	if err = json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	normalize := func(v map[string]any) {
		// Adapter capacity is chosen by Environment. The baseline device contract
		// and the complete limit inventory must still match the reference.
		limits := v["adapterLimits"].(map[string]any)
		names := make([]string, 0, len(limits))
		for name := range limits {
			names = append(names, name)
		}
		sort.Strings(names)
		v["adapterLimits"] = names
		// Fresh Chrome processes independently vary set insertion order.
		for _, key := range []string{"features", "wgsl"} {
			a := v[key].([]any)
			sort.Slice(a, func(i, j int) bool { return a[i].(string) < a[j].(string) })
		}
	}
	normalize(expected.Observation)
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		for _, worker := range []bool{false, true} {
			expression := "(async()=>JSON.stringify(await " + string(source) + "))()"
			if worker {
				code := "onmessage=async()=>postMessage(await " + expression + ")"
				expression = `new Promise(resolve=>{const u=URL.createObjectURL(new Blob([` + strconv.Quote(code) + `])),w=new Worker(u);w.onmessage=e=>{w.terminate();URL.revokeObjectURL(u);resolve(e.data)};w.postMessage(null)})`
			}
			value, err := p.Evaluate(context.Background(), expression)
			if err != nil {
				t.Fatal(err)
			}
			var actual map[string]any
			if err = json.Unmarshal([]byte(value.(string)), &actual); err != nil {
				t.Fatal(err)
			}
			normalize(actual)
			if !reflect.DeepEqual(actual, expected.Observation) {
				t.Fatalf("worker=%v WebGPU contract mismatch: %v", worker, actual)
			}
		}
	})
}
