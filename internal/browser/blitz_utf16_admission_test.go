package browser

import (
	"context"
	"fmt"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"strings"
	"testing"
)

func TestBlitzUTF16AdmissionFallsBackAndRepairs(t *testing.T) {
	serialBrowserTest(t)
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
	eval := func(code, want string) {
		t.Helper()
		value, err := p.Evaluate(context.Background(), code)
		if err != nil || fmt.Sprint(value) != want {
			t.Fatalf("UTF16 observation %v err%v want%s", value, err, want)
		}
	}
	fallback := func() {
		t.Helper()
		s := p.Top.Realm.blitz
		if s == nil || s.document.Owner != nil || !strings.Contains(s.fallback, "UTF-16") {
			t.Fatalf("not explicit UTF16 fallback: %+v", s)
		}
	}
	native := func() {
		t.Helper()
		s := p.Top.Realm.blitz
		if s == nil || s.document.Owner == nil || s.fallback != "" {
			t.Fatalf("not repaired native owner: %+v", s)
		}
	}
	eval(`document.body.innerHTML='<div id="box" style="width:40px;height:20px">x</div>';globalThis.box=document.querySelector('#box');box.firstChild.data='\ud800';[box.firstChild.data.charCodeAt(0),box.getBoundingClientRect().width].join(',')`, "55296,40")
	fallback()
	eval(`box.textContent='repaired';box.getBoundingClientRect().width`, "40")
	native()
	eval(`box.firstChild.data='\udfff';[box.firstChild.data.charCodeAt(0),box.getBoundingClientRect().width].join(',')`, "57343,40")
	fallback()
	eval(`box.firstChild.remove();box.getBoundingClientRect().width`, "40")
	native()
}
