package browser

import (
	"encoding/json"
	"os"
	"testing"
)

func TestDialogLifecycleChrome152(t *testing.T) {
	parallelBrowserTest(t)
	source, err := os.ReadFile("testdata/dialog_lifecycle_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	wire, err := os.ReadFile("testdata/dialog_lifecycle_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected any
	if err := json.Unmarshal(wire, &expected); err != nil {
		t.Fatal(err)
	}
	compact, _ := json.Marshal(expected)
	p := bootstrapSnapshotPage(t)
	historyEval(t, p, "("+string(source)+").then(value=>JSON.stringify(value))", string(compact))
}
