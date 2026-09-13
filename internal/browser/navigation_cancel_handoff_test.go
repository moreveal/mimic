//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/trace"
)

func TestNavigationCancellationSurvivesCommitHandoff(t *testing.T) {
	for _, boundary := range []string{"before-handoff", "after-ack"} {
		t.Run(boundary, func(t *testing.T) {
			p := bootstrapSnapshotPage(t)
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/new" {
					fmt.Fprint(w, `<script>globalThis.recovered=42</script>`)
					return
				}
				fmt.Fprint(w, `<script>globalThis.ranAfterStop=true</script>`)
			}))
			defer fixture.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			stopped := make(chan bool, 1)
			unsubscribe := p.Trace().Subscribe(func(event trace.Event) {
				if boundary == "before-handoff" && event.Kind == trace.Lifecycle && event.Name == "frameNavigated" {
					stopped <- p.CancelNavigation()
				}
			})
			defer unsubscribe()
			initial := p.Top.Realm
			if err := p.StartNavigation(ctx, fixture.URL, p.ReserveNavigation(), func(err error) {
				if boundary == "after-ack" && err == nil {
					stopped <- p.CancelNavigation()
				}
			}); err != nil {
				t.Fatal(err)
			}
			for p.Top.Realm == initial {
				if err := p.AdvanceTime(ctx, time.Millisecond); err != nil {
					t.Fatal(err)
				}
				if ctx.Err() != nil {
					t.Fatal("navigation did not reach commit")
				}
				time.Sleep(time.Millisecond)
			}
			unsubscribe()
			select {
			case ok := <-stopped:
				if !ok {
					t.Fatal("commit had no cancellation handle")
				}
			default:
				t.Fatal("cancellation boundary did not run")
			}
			stream := p.Top.Realm.documentStream
			if stream == nil || stream.ctx.Err() != context.Canceled {
				t.Fatal("commit replaced canceled navigation with a live parser")
			}
			value, err := p.EvaluateCommand(ctx, "", `typeof ranAfterStop==='undefined'`)
			if err != nil || value != true || p.LoadEventEnded() {
				t.Fatalf("stopped parser executed or completed: value=%v err=%v loaded=%v", value, err, p.LoadEventEnded())
			}
			if err := p.StartNavigation(ctx, fixture.URL+"/new", p.ReserveNavigation()); err != nil {
				t.Fatal(err)
			}
			for !p.LoadEventEnded() {
				if err := p.AdvanceTime(ctx, time.Millisecond); err != nil {
					t.Fatal(err)
				}
				if ctx.Err() != nil {
					t.Fatal("replacement did not recover after cancellation")
				}
				time.Sleep(time.Millisecond)
			}
			value, err = p.EvaluateCommand(ctx, "", `recovered===42&&typeof ranAfterStop==='undefined'`)
			if err != nil || value != true {
				t.Fatalf("replacement inherited canceled lifetime: %v %v", value, err)
			}
		})
	}
}
