package browser

import (
	"context"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
)

type countingFactory struct{ count int }

func (f *countingFactory) New() engine.Runtime {
	f.count++
	return (gojaengine.Factory{}).New()
}

func TestInitialBlankRealmIsDeferredAndIndependent(t *testing.T) {
	factory := &countingFactory{}
	b, err := New(factory, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	first, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if err := first.AdvanceTime(context.Background(), time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if factory.count != 0 {
		t.Fatalf("unobserved blank pages initialized %d runtimes", factory.count)
	}
	value, err := first.Evaluate(context.Background(), `Object.prototype.pageOnly=42;document.body.textContent='first';document.readyState==='complete'&&document.body.textContent==='first'`)
	if err != nil || value != true {
		t.Fatalf("initial blank: %v %v", value, err)
	}
	if factory.count != 1 {
		t.Fatalf("runtime count %d", factory.count)
	}
	value, err = second.Evaluate(context.Background(), `!('pageOnly' in {})&&document.body.textContent===''`)
	if err != nil || value != true {
		t.Fatalf("independent blank: %v %v", value, err)
	}
	if factory.count != 2 {
		t.Fatalf("runtime count %d", factory.count)
	}
}
