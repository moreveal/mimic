//go:build (windows || linux) && amd64

package browser

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// This opt-in diagnostic records ordinary reclamation, without forced GC,
// retries, or a change to the frozen gate's memory recovery policy.
func TestDetachedFrameMemoryProfile(t *testing.T) {
	serialBrowserTest(t)
	output := os.Getenv("MIMIC_DETACHED_MEMORY_OUTPUT")
	if output == "" {
		t.Skip("set MIMIC_DETACHED_MEMORY_OUTPUT for lifetime memory attribution")
	}
	p := bootstrapSnapshotPage(t)
	var records []map[string]any
	sample := func(phase string) {
		r := processMemory(t)
		r["phase"] = phase
		r["realms"] = len(p.realmOwners)
		r["retained_frames"] = 0
		if p.Top.Realm != nil {
			r["retained_frames"] = len(p.Top.Realm.retainedFrames)
		}
		records = append(records, r)
	}
	sample("initial")
	for _, phase := range []string{"removed8", "removed16", "removed24", "removed32"} {
		bootstrapSnapshotEvaluate(t, p, `(()=>{for(let i=0;i<8;i++){const f=document.createElement('iframe');document.body.appendChild(f);f.remove()}return true})()`)
		sample(phase)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	sample("closed")
	time.Sleep(250 * time.Millisecond)
	sample("closed250ms")
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(output, data, 0644); err != nil {
		t.Fatal(err)
	}
}
