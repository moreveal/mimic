//go:build (windows || linux) && amd64

package browser

import "testing"

func TestUnobservedDetachedFramesAreReleased(t *testing.T) {
	parallelBrowserTest(t)
	p := bootstrapSnapshotPage(t)
	bootstrapSnapshotEvaluate(t, p, `(()=>{for(let i=0;i<8;i++){const f=document.createElement('iframe');document.body.appendChild(f);f.remove()}return true})()`)
	t.Logf("owners=%d retained=%d windowReferences=%d", len(p.realmOwners), len(p.Top.Realm.retainedFrames), len(p.Top.Realm.windowReferences))
	if len(p.realmOwners) != 1 {
		t.Fatalf("unobserved detached contexts retained: %d", len(p.realmOwners))
	}
}

func TestDetachedFrameRelationsMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "detached_frame")
}
