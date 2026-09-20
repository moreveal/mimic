package browser

import (
	"context"
	"testing"
	"time"
)

// Dynamic inline script cleanup can start a Page checkpoint on a child owner
// thread. A pending parent job is allowed to synchronously read that child.
func TestCheckpointOwnerCanReenterFromForeignMicrotask(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := p.Evaluate(ctx, `const frame=document.createElement('iframe');document.body.append(frame);frame.contentWindow.marker=42`); err != nil {
			t.Fatal(err)
		}
		if _, native := p.Top.Realm.runtime.(interface {
			MicrotaskCheckpointContext(context.Context) error
		}); !native {
			t.Skip("requires explicitly queued native microtasks and owner threads")
		}
		parent := p.Top.Realm
		child := p.Top.Children()[0].Realm
		if _, err := parent.Evaluate(ctx, `Promise.resolve().then(()=>{globalThis.answer=frame.contentWindow.marker})`, "queued-parent-job"); err != nil {
			t.Fatal(err)
		}
		p.requireCheckpoint(parent)
		if err := child.runOnOwner(ctx, func(ctx context.Context) error { return child.checkpoint(ctx) }); err != nil {
			t.Fatal(err)
		}
		got, err := p.Evaluate(ctx, `answer`)
		if err != nil || got != float64(42) {
			t.Fatalf("parent job could not reenter checkpoint owner: %v, %v", got, err)
		}
	})
}
