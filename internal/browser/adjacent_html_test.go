package browser

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestAdjacentHTMLMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	source, err := os.ReadFile("testdata/adjacent_html_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	capture, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/adjacent-html-chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Result json.RawMessage `json:"result"`
	}
	if err = json.Unmarshal(capture, &oracle); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), "JSON.stringify("+string(source)+")===JSON.stringify("+string(oracle.Result)+")")
		if err != nil || value != true {
			actual, _ := p.Evaluate(context.Background(), string(source))
			t.Fatalf("adjacent HTML: %v %v actual=%v", value, err, actual)
		}
	})
}
func TestAdjacentHTMLLifecycleMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	source, err := os.ReadFile("testdata/adjacent_html_lifecycle_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	capture, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/adjacent-html-lifecycle-chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Result json.RawMessage `json:"result"`
	}
	if err = json.Unmarshal(capture, &oracle); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		value, err := p.Evaluate(context.Background(), "(async()=>JSON.stringify(await "+string(source)+")===JSON.stringify("+string(oracle.Result)+"))()")
		if err != nil || value != true {
			actual, _ := p.Evaluate(context.Background(), string(source))
			t.Fatalf("adjacent HTML: %v %v actual=%v", value, err, actual)
		}
	})
}
