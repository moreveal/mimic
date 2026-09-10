package browser

import (
	"context"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"testing"
)

func BenchmarkFrameBridgeOperations(b *testing.B) {
	browser, e := New(v8engine.Factory{}, chrome152.New())
	if e != nil {
		b.Fatal(e)
	}
	p, e := browser.NewContext().NewPage()
	if e != nil {
		b.Fatal(e)
	}
	defer p.Close()
	for i := 0; i < b.N; i++ {
		v, e := p.Evaluate(context.Background(), frameValueEncoderProbe)
		if e != nil || numberValue(v) != 6050 {
			b.Fatalf("%v %v", v, e)
		}
	}
}
