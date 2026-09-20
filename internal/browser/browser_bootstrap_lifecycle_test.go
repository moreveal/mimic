//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"fmt"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestBrowserBootstrapSnapshotKeySeparatesContextProfiles(t *testing.T) {
	serialBrowserTest(t)
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	first := b.NewContext()
	defer first.Close()
	p, err := first.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Evaluate(context.Background(), "true"); err != nil {
		t.Fatal(err)
	}
	firstKey := p.Top.Realm.bootstrapSource().key

	profile := fmt.Sprintf(`{"schemaVersion":1,"baseProfile":%q,"locale":{"timezone":"America/New_York","intlLocale":"en-US"}}`, b.env.ProfileID)
	second, err := b.NewContextWithProfile([]byte(profile))
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	q, err := second.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = q.Evaluate(context.Background(), "true"); err != nil {
		t.Fatal(err)
	}
	if q.Top.Realm.bootstrapSource().key == firstKey {
		t.Fatal("distinct Context profiles shared a bootstrap snapshot key")
	}
}

func TestBootstrapSnapshotSharedAcrossContexts(t *testing.T) {
	serialBrowserTest(t)
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	seed := b.NewContext()
	p, err := seed.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Evaluate(context.Background(), "true"); err != nil {
		t.Fatal(err)
	}
	key := p.Top.Realm.bootstrapSource().key
	seed.ClosePage(p.ID)
	trigger, err := seed.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := trigger.Evaluate(context.Background(), "true"); err != nil {
		t.Fatal(err)
	}
	seed.ClosePage(trigger.ID)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := b.bootstrapSnapshots.wait(ctx); err != nil {
		t.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}

	sibling := b.NewContext()
	defer sibling.Close()
	restored, err := sibling.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restored.Evaluate(context.Background(), "true"); err != nil {
		t.Fatal(err)
	}
	defer sibling.ClosePage(restored.ID)
	if restored.Top.Realm.bootstrapSource().key != key || !restored.Top.Realm.bootstrapRestored {
		b.bootstrapSnapshots.mu.Lock()
		entry := b.bootstrapSnapshots.entries[key]
		var snapshotReady bool
		var snapshotErr error
		if entry != nil {
			snapshotReady, snapshotErr = entry.snapshot != nil, entry.err
		}
		b.bootstrapSnapshots.mu.Unlock()
		t.Fatalf("new Context did not restore the Browser-owned bootstrap snapshot: keyMatch=%v ready=%v err=%v", restored.Top.Realm.bootstrapSource().key == key, snapshotReady, snapshotErr)
	}
}

func TestBrowserCloseDisposesSharedBootstrapAndRejectsPages(t *testing.T) {
	serialBrowserTest(t)
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	if _, err := c.NewPage(); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := b.NewContext().NewPage(); err == nil {
		t.Fatal("closed Browser admitted a new Page")
	}
	b.bootstrapSnapshots.mu.Lock()
	closed := b.bootstrapSnapshots.closed
	entries := len(b.bootstrapSnapshots.entries)
	b.bootstrapSnapshots.mu.Unlock()
	if !closed || entries != 0 {
		t.Fatalf("bootstrap cache after Browser.Close: closed=%v entries=%d", closed, entries)
	}
}
