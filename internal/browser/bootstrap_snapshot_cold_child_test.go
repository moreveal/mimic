//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// Most snapshot tests capture a parent first. A profile can also be encountered
// first in a child, whose parent/top must not become native objects in the seed.
func TestBootstrapSnapshotColdChildRelations(t *testing.T) {
	serialBrowserTest(t)
	if os.Getenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT") == "1" || !(v8engine.Factory{}).BootstrapSnapshotsEnabled() {
		t.Skip("requires snapshot capture and restoration")
	}
	for _, crossOrigin := range []bool{false, true} {
		t.Run(fmt.Sprintf("cross-origin=%t", crossOrigin), func(t *testing.T) {
			fixture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `<!doctype html><body>local fixture<script>globalThis.fixtureLoaded=true</script>`)
			})
			parent := httptest.NewServer(fixture)
			defer parent.Close()
			foreign := httptest.NewServer(fixture)
			defer foreign.Close()
			childURL := parent.URL
			if crossOrigin {
				childURL = foreign.URL
			}
			p := bootstrapSnapshotPage(t)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := p.Navigate(ctx, parent.URL); err != nil {
				t.Fatal(err)
			}
			bootstrapSnapshotEvaluate(t, p, `globalThis.parentJSONReads=0;
Object.defineProperty(globalThis,'toJSON',{get(){parentJSONReads++;throw Error('parent must not be serialized')}});true`)
			// Force the first capture to belong to the child without relying on
			// a particular secure/isolation profile or parent warmup sequence.
			if err := p.ctx.bootstrapSnapshots.close(); err != nil {
				t.Fatal(err)
			}
			p.ctx.bootstrapSnapshots = bootstrapSnapshotCache{}
			makeChild := func() *Realm {
				t.Helper()
				before := make(map[*Realm]bool)
				for _, frame := range p.Top.Realm.childFrames {
					before[frame.Realm] = true
				}
				bootstrapSnapshotEvaluate(t, p, fmt.Sprintf(`(async()=>{
const frame=document.createElement('iframe');
const loaded=new Promise(resolve=>frame.onload=resolve);
frame.src=%q;document.body.appendChild(frame);await loaded;return true;
})()`, childURL))
				for _, frame := range p.Top.Realm.childFrames {
					if frame.Realm != nil && !before[frame.Realm] {
						return frame.Realm
					}
				}
				t.Fatal("new child realm missing")
				return nil
			}
			check := func(realm *Realm) {
				t.Helper()
				value, err := realm.runtime.Eval(ctx, fmt.Sprintf(`(()=>{
const owner=parent;let blocked=false;try{void owner.document}catch(e){blocked=e.name==='SecurityError'}
return parent===owner && top===owner && owner!==globalThis && window===globalThis && blocked===%t;
})()`, crossOrigin), "cold-child-relations")
				if err != nil || value.Export() != true {
					t.Fatalf("child relations: %v %v", value, err)
				}
			}
			first := makeChild()
			check(first)
			p.ctx.bootstrapSnapshots.mu.Lock()
			entry := p.ctx.bootstrapSnapshots.entries[first.bootstrapSource().key]
			captured := entry != nil && entry.err == nil && len(entry.seed) > 0
			p.ctx.bootstrapSnapshots.mu.Unlock()
			if !captured {
				t.Fatal("first child did not produce a reusable seed")
			}
			// A second use admits the build; the next child must restore it.
			check(makeChild())
			if err := p.ctx.bootstrapSnapshots.wait(ctx); err != nil {
				t.Fatal(err)
			}
			third := makeChild()
			if !third.bootstrapRestored {
				t.Fatal("child seed was not restored")
			}
			check(third)
			if value := bootstrapSnapshotEvaluate(t, p, "parentJSONReads"); numberValue(value) != 0 {
				t.Fatalf("capture inspected parent toJSON: %v", value)
			}
			// The same child-created seed must also bind correctly in a top Page.
			restored, err := p.ctx.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			if err := restored.Navigate(ctx, parent.URL); err != nil {
				t.Fatal(err)
			}
			value := bootstrapSnapshotEvaluate(t, restored, "parent===globalThis && top===globalThis && !Object.hasOwn(globalThis,'parentJSONReads')")
			if value != true || !restored.Top.Realm.bootstrapRestored {
				t.Fatalf("top Page retained seed relations: %v, restored=%t", value, restored.Top.Realm.bootstrapRestored)
			}
		})
	}
}
