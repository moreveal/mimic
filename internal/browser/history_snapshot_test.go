//go:build (windows || linux) && amd64

package browser

import "testing"

func TestHistoryCloneRestoredBootstrap(t *testing.T) {
	serialBrowserTest(t)
	for _, mode := range []string{"ordinary", "snapshot"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "ordinary" {
				t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "1")
			} else {
				t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "0")
			}
			seed := bootstrapSnapshotPage(t)
			navigateCapabilityFixture(t, seed)
			if mode == "snapshot" {
				bootstrapSnapshotWarm(t, seed)
			}
			p, err := seed.ctx.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = p.Close() })
			navigateCapabilityFixture(t, p)
			if mode == "snapshot" && !p.Top.Realm.bootstrapRestored {
				t.Fatal("warm fixture did not restore a snapshot")
			}
			historyEval(t, p, `(()=>{let reads=0;const value={get x(){reads++;return {n:1}}};history.replaceState(value,'');const stored=history.state;if(stored===value||stored.x.n!==1||reads!==1)return false;try{history.pushState(()=>{},'');return false}catch(e){return e.name==='DataCloneError'&&history.state===stored}})()`, true)
			historyEval(t, p, `(()=>{const saved=history.state;try{history.pushState({blob:new Blob(['x'])},'');return false}catch(e){return e.name==='NotSupportedError'&&history.state===saved}})()`, true)
		})
	}
}
