package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestBase64WorkerChrome(t *testing.T) {
	fixture, err := os.ReadFile("testdata/base64_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/base64-worker-chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var capture struct {
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(data, &capture); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		code := "onmessage=()=>postMessage(JSON.stringify(" + string(fixture) + "))"
		expression := `new Promise((resolve,reject)=>{const u=URL.createObjectURL(new Blob([` + strconv.Quote(code) + `])),w=new Worker(u);w.onmessage=e=>{w.terminate();URL.revokeObjectURL(u);resolve(e.data)};w.onerror=e=>reject(Error(e.message));w.postMessage(null)})`
		value, err := p.Evaluate(context.Background(), expression)
		if err != nil {
			t.Fatal(err)
		}
		var actual map[string]any
		if err := json.Unmarshal([]byte(value.(string)), &actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, capture.Result) {
			t.Fatalf("worker: got %v want %v", actual, capture.Result)
		}
	})
}

func TestDocumentCompatibilityOracle(t *testing.T) {
	for _, name := range []string{"document-visibility", "html-enumerated", "form-reflection", "webgl-color-space", "document-handlers", "element-handlers", "base64", "navigator-power", "cross-realm-nodes", "svg-bbox", "css-font-size", "css-priority", "svg-css-transform"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/" + strings.ReplaceAll(name, "-", "_") + "_oracle.js")
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/" + name + "-chrome152.json")
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
				expression := "JSON.stringify(" + string(source) + ")"
				if name == "navigator-power" {
					expression = "Promise.resolve(" + string(source) + ").then(JSON.stringify)"
				}
				value, err := p.Evaluate(context.Background(), expression)
				if err != nil {
					t.Fatal(err)
				}
				var actual map[string]any
				if err = json.Unmarshal([]byte(value.(string)), &actual); err != nil {
					t.Fatal(err)
				}
				for key, want := range capture.Result {
					if !reflect.DeepEqual(actual[key], want) {
						t.Errorf("%s: got %v want %v", key, actual[key], want)
					}
				}
			})
		})
	}
}
