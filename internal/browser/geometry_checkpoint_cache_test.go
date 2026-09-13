package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestGeometryEpochSurvivesCheckpointAndInvalidatesMutations(t *testing.T) {
	t.Setenv("MIMIC_PROFILE_HOSTS", "1")
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
	ctx := context.Background()
	eval := func(source string) any {
		t.Helper()
		value, err := p.Evaluate(ctx, source)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	count := func() uint64 {
		data, err := json.Marshal(p.LiveDiagnostics())
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Costs map[string]struct{ Count uint64 }
		}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		return result.Costs["host:computedStyleAvailable"].Count
	}
	eval(`document.head.innerHTML='<style>.box{width:40px;height:20px}.wide{width:80px}</style>';document.body.innerHTML='<div class="box"></div>';window.box=document.body.firstChild;box.getBoundingClientRect().width`)
	before := count()
	if before == 0 {
		t.Fatal("initial read did not compute geometry")
	}
	for i := 0; i < 5; i++ {
		if err := p.AdvanceTime(ctx, time.Millisecond); err != nil {
			t.Fatal(err)
		}
		if got := eval(`box.getBoundingClientRect().width`); fmt.Sprint(got) != "40" {
			t.Fatalf("unchanged width: %v", got)
		}
	}
	if after := count(); after != before {
		t.Fatalf("checkpoint discarded unchanged geometry: %d -> %d", before, after)
	}
	for _, step := range []struct {
		source string
		want   int64
	}{
		{`box.classList.add('wide');box.getBoundingClientRect().width`, 80},
		{`box.classList.remove('wide');box.getBoundingClientRect().width`, 40},
		{`document.styleSheets[0].cssRules[0].style.width='60px';box.getBoundingClientRect().width`, 60},
		{`box.style.display='none';box.getBoundingClientRect().width`, 0},
		{`box.style.display='block';box.getBoundingClientRect().width`, 60},
	} {
		if got := eval(step.source); fmt.Sprint(got) != fmt.Sprint(step.want) {
			t.Fatalf("%s: %v want %d", step.source, got, step.want)
		}
	}
}
