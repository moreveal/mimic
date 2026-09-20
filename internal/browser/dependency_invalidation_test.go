package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestIrrelevantDataAttributeRetainsGeometryDependencies(t *testing.T) {
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
	styleReads := func() uint64 {
		t.Helper()
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
		return result.Costs["host:styleObservationState"].Count
	}

	eval(`document.head.innerHTML='<style>.scope .target{width:40px;height:20px}.scope[data-wide] .target{width:80px}</style>';
document.body.innerHTML='<section class="scope"><div class="target">target</div>'+Array.from({length:1000},(_,i)=>'<div>row '+i+'</div>').join('')+'</section>';
window.scope=document.body.firstChild;window.target=scope.firstChild;target.getBoundingClientRect().width`)
	initial := styleReads()
	if initial == 0 {
		t.Fatal("initial geometry did not read style state")
	}
	if got := eval(`scope.lastChild.setAttribute('data-probe-unused','1');target.getBoundingClientRect().width`); fmt.Sprint(got) != "40" {
		t.Fatalf("irrelevant attribute width: %v", got)
	}
	if after := styleReads(); after != initial {
		t.Fatalf("irrelevant data attribute rebuilt style/geometry: %d -> %d", initial, after)
	}

	if got := eval(`scope.setAttribute('data-wide','');target.getBoundingClientRect().width`); fmt.Sprint(got) != "80" {
		t.Fatalf("selector attribute width: %v", got)
	}
	usedAttribute := styleReads()
	if usedAttribute <= initial {
		t.Fatalf("selector attribute did not invalidate dependencies: %d -> %d", initial, usedAttribute)
	}

	if got := eval(`target.textContent='changed text';target.getBoundingClientRect().width`); fmt.Sprint(got) != "80" {
		t.Fatalf("text mutation width: %v", got)
	}
	textMutation := styleReads()
	if textMutation <= usedAttribute {
		t.Fatalf("text mutation did not invalidate geometry: %d -> %d", usedAttribute, textMutation)
	}

	if got := eval(`scope.insertBefore(document.createElement('div'),target);target.getBoundingClientRect().width`); fmt.Sprint(got) != "80" {
		t.Fatalf("topology mutation width: %v", got)
	}
	if topology := styleReads(); topology <= textMutation {
		t.Fatalf("topology mutation did not invalidate geometry: %d -> %d", textMutation, topology)
	}

	navigateCapabilityFixture(t, p)
	eval(`document.head.innerHTML='<style>.target{width:13px;height:9px}</style>';document.body.innerHTML='<div class="target"></div>';window.target=document.body.firstChild;target.getBoundingClientRect().width`)
	navigated := styleReads()
	if got := eval(`target.setAttribute('data-wide','');target.getBoundingClientRect().width`); fmt.Sprint(got) != "13" {
		t.Fatalf("post-navigation irrelevant attribute width: %v", got)
	}
	if after := styleReads(); after != navigated {
		t.Fatalf("navigation retained stale attribute dependencies: %d -> %d", navigated, after)
	}
}
