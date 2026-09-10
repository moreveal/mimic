package browser

import (
	"context"
	"testing"
	"time"
)

// Frozen Chrome 152 advances both clocks within a synchronous evaluation and
// its Promise jobs. Polling the clock itself makes a frozen clock fail by the
// context deadline instead of relying on a machine-specific loop duration.
func TestEvaluationClockRunsThroughBodyAndMicrotasks(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, `(()=>{
			const samples=[];
			function sample(){const d=Date.now(),p=performance.now();while(performance.now()-p<8){};samples.push(Date.now()-d>=5)}
			sample();return Promise.resolve().then(()=>{sample();return samples.every(Boolean)})
		})()`)
		if err != nil || value != true {
			t.Fatalf("live evaluation clocks: %v, %v", value, err)
		}
		if _, err := p.Evaluate(ctx, `throw new Error('expected')`); err == nil {
			t.Fatal("missing synchronous exception")
		}
		value, err = p.Evaluate(ctx, `(()=>{const start=performance.now();while(performance.now()-start<3){};return true})()`)
		if err != nil || value != true {
			t.Fatalf("clock after failed turn: %v, %v", value, err)
		}
		if _, err := p.Evaluate(ctx, `document.body.appendChild(document.createElement('iframe'));void 0`); err != nil {
			t.Fatal(err)
		}
		if len(p.Top.children) != 1 {
			t.Fatal("missing child fixture")
		}
		for _, frame := range p.Top.children {
			value, err := p.EvaluateFrame(ctx, frame.ID, `(()=>{const d=Date.now(),p=performance.now();while(performance.now()-p<8){};return Date.now()-d>=5})()`)
			if err != nil || value != true {
				t.Fatalf("child context evaluation clock: %v, %v", value, err)
			}
		}
	})
}
