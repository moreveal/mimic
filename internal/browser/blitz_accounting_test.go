package browser

import (
	"context"
	"fmt"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"testing"
)

func TestBlitzNativeGenerationTracksCleanAndMutatedObservations(t *testing.T) {
	t.Setenv("MIMIC_STYLE_ENGINE", "blitz")
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	navigateCapabilityFixture(t, p)
	eval := func(code string, want string) {
		t.Helper()
		got, err := p.Evaluate(context.Background(), code)
		if err != nil || fmt.Sprint(got) != want {
			t.Fatalf("observation %v err %v want%s", got, err, want)
		}
	}
	eval(`document.body.innerHTML='<div id="box" style="width:40px;height:20px"></div>';document.querySelector('#box').getBoundingClientRect().width`, "40")
	state := p.Top.Realm.blitz
	if state == nil || state.document.Owner == nil || state.fallback != "" {
		t.Fatal("native generation gate used fallback")
	}
	generation, builds := state.generation, state.document.Builds
	if generation == 0 || builds != 1 {
		t.Fatalf("first-build accounting generation%d builds%d", generation, builds)
	}
	eval(`document.querySelector('#box').getBoundingClientRect().width`, "40")
	if state.generation != generation || state.document.Builds != builds {
		t.Fatal("clean observation rebuilt native state")
	}
	eval(`document.querySelector('#box').style.width='80px';document.querySelector('#box').getBoundingClientRect().width`, "80")
	if state.generation <= generation || state.document.Builds != builds {
		t.Fatal("mutation did not rebuild products within same owner")
	}
}
