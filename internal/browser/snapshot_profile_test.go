package browser

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// An opt-in live-site diagnostic, not a deterministic compatibility benchmark.
// It profiles complete hydration turns after the parser returns and retains
// the interrupted document for inspection if a turn exceeds the sample window.
func TestSnapshotWorkloadProfile(t *testing.T) {
	url, dir := os.Getenv("MIMIC_SNAPSHOT_PROFILE_URL"), os.Getenv("MIMIC_SNAPSHOT_PROFILE_DIR")
	if url == "" || dir == "" {
		t.Skip("set MIMIC_SNAPSHOT_PROFILE_URL and MIMIC_SNAPSHOT_PROFILE_DIR")
	}
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
	navContext, cancelNavigation := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelNavigation()
	started := time.Now()
	err = p.NavigateReserved(navContext, url, p.ReserveNavigation())
	t.Logf("parser: %v, error: %v", time.Since(started), err)
	finish, err := p.Top.Realm.runtime.(interface {
		ProfileWorkloadCPU() (func() (any, error), error)
	}).ProfileWorkloadCPU()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for ctx.Err() == nil {
		if _, err := p.AdvanceTimeBudget(ctx, 20*time.Millisecond, 32); err != nil {
			t.Log(err)
		}
	}
	profile, err := finish()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cpu.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(p.LiveDiagnostics())
	if err := os.WriteFile(filepath.Join(dir, "hosts.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	value, err := p.Top.Realm.runtime.Eval(context.Background(), `JSON.stringify({url:location.href,ready:document.readyState,title:document.title,elements:document.querySelectorAll('*').length,images:document.images.length,imagesWithSource:Array.from(document.images).filter(image=>!!image.getAttribute('src')).length})`, "snapshot-profile-observations")
	if err != nil {
		t.Log(err)
	} else if err := os.WriteFile(filepath.Join(dir, "dom.json"), []byte(value.String()), 0644); err != nil {
		t.Fatal(err)
	}
}
