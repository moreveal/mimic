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

func TestWindows1252DecoderMatchesChromeInWindowAndWorker(t *testing.T) {
	source, err := os.ReadFile("testdata/windows1252_decoder_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	capture, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/windows1252-decoder-chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Result any `json:"result"`
	}
	if err = json.Unmarshal(capture, &oracle); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		for _, worker := range []bool{false, true} {
			script := string(source)
			if worker {
				code := "onmessage=()=>postMessage(" + script + ")"
				script = `new Promise((resolve,reject)=>{const url=URL.createObjectURL(new Blob([` + strconv.Quote(code) + `]));const worker=new Worker(url);worker.onerror=e=>reject(Error(e.message));worker.onmessage=e=>{worker.terminate();URL.revokeObjectURL(url);resolve(e.data)};worker.postMessage(null)})`
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			value, err := p.Evaluate(ctx, script)
			cancel()
			if err != nil {
				t.Fatalf("worker=%v: %v", worker, err)
			}
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var got any
			if err = json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, oracle.Result) {
				t.Fatalf("worker=%v: decoder differs from frozen Chrome\ngot %s", worker, data)
			}
		}
	})
}
