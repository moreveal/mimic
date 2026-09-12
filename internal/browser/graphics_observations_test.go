package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func TestGraphicsObservationOracles(t *testing.T) {
	for _, fixture := range []struct{ name, capture string }{{"canvas_paths", "canvas-paths"}, {"canvas_relations", "canvas-relations"}, {"webgl_programs", "webgl-programs"}, {"webgl_framebuffers", "webgl-framebuffers"}, {"webgl_validation", "webgl-validation"}, {"webgl_geometry", "webgl-geometry"}, {"webgl_framebuffer_lifecycle", "webgl-framebuffer-lifecycle"}} {
		t.Run(fixture.name, func(t *testing.T) {
			parallelOracle(t)
			source, err := os.ReadFile("testdata/" + fixture.name + "_oracle.js")
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/" + fixture.capture + "-chrome152.json")
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
				for _, worker := range []bool{false, true} {
					expression := "JSON.stringify(" + string(source) + ")"
					if worker {
						code := "onmessage=()=>{try{postMessage(" + expression + ")}catch(e){postMessage(String(e))}}"
						expression = `new Promise((resolve,reject)=>{const u=URL.createObjectURL(new Blob([` + strconv.Quote(code) + `])),w=new Worker(u);w.onmessage=e=>{w.terminate();URL.revokeObjectURL(u);resolve(e.data)};w.onerror=e=>reject(Error(e.message));w.postMessage(null)})`
					}
					value, err := p.Evaluate(context.Background(), expression)
					if err != nil {
						t.Fatal(err)
					}
					var actual map[string]any
					if err = json.Unmarshal([]byte(value.(string)), &actual); err != nil {
						t.Fatalf("worker=%v result=%v: %v", worker, value, err)
					}
					for key, want := range capture.Result {
						if !reflect.DeepEqual(actual[key], want) {
							t.Errorf("worker=%v %s: got %v want %v", worker, key, actual[key], want)
						}
					}
				}
			})
		})
	}
}
