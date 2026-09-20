package browser

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestTextCacheProfileCountsAndBoundedTracking(t *testing.T) {
	parallelBrowserTest(t)
	profile, cache := newTextCacheProfile(), &textShapeCache{}
	observe := func(key textShapeKey, value string) {
		_, hit := cache.get(key)
		profile.observe(key, hit)
		if !hit {
			before := len(cache.values)
			cache.put(key, value)
			profile.success(key, value, before, cache)
		}
	}
	key := textShapeKey{text: "sample", families: "Arial", flags: 1 << 4}
	observe(key, "metrics")
	observe(key, "metrics")
	if profile.Compact.Calls != 2 || profile.Compact.Hits != 1 || profile.Compact.Misses != 1 || profile.Compact.DistinctSuccessfulKeys != 1 || profile.Compact.TrackedWorkingSetBytes != textShapeEntryBytes(key, "metrics") {
		t.Fatalf("unexpected compact counts: %+v", profile.Compact)
	}
	key.flags = 0
	observe(key, "glyphs")
	if profile.Full.DistinctTrackedKeys != 1 {
		t.Fatal("full and compact fingerprints alias")
	}
	profile.FontInvalidations++
	cache = &textShapeCache{}
	observe(key, "glyphs")
	if profile.Full.DistinctTrackedKeys != 2 {
		t.Fatal("font generations alias")
	}
	for i := 0; i < textCacheProfileKeys+10; i++ {
		observe(textShapeKey{text: fmt.Sprint(i)}, "x")
	}
	if len(profile.keys) != textCacheProfileKeys || profile.Full.UntrackedObservations == 0 || profile.Full.EvictedEntries == 0 {
		t.Fatalf("profile did not report bounds/eviction: tracked=%d counts=%+v", len(profile.keys), profile.Full)
	}
	if profile.PeakEntries > textShapeCacheEntries || profile.PeakBytes > textShapeCacheBytes {
		t.Fatal("profile peak exceeded cache limits")
	}
	observe(textShapeKey{text: "oversized"}, strings.Repeat("x", textShapeCacheBytes))
	if profile.Full.OversizedMisses != 1 {
		t.Fatal("oversized admission not reported")
	}
}

func TestTextCacheProfileIsOptInAndReleased(t *testing.T) {
	serialBrowserTest(t)
	t.Setenv("MIMIC_PROFILE_TEXT_CACHE", "")
	p := newAsyncModulePage(t)
	navigateCapabilityFixture(t, p)
	if p.Top.Realm.textCacheProfile != nil {
		t.Fatal("text cache profiling enabled by default")
	}
	t.Setenv("MIMIC_PROFILE_TEXT_CACHE", "1")
	navigateCapabilityFixture(t, p)
	exposeTextProjectionHosts(t, p)
	_, err := p.Evaluate(context.Background(), `(()=>{shapeTextMetrics('AV','Arial',16,400,0,0,0,0);shapeTextMetrics('AV','Arial',16,400,0,0,0,0);shapeTextMetrics('x','Arial',NaN,400,0,0,0,0)})()`)
	if err != nil {
		t.Fatal(err)
	}
	old := p.Top.Realm
	if old.textCacheProfile == nil {
		t.Fatal("explicit profile not enabled")
	}
	counts := old.textCacheProfile.Compact
	if counts.Calls != 3 || counts.Hits != 1 || counts.Misses != 2 || counts.SuccessfulMisses != 1 || counts.FailedMisses != 1 {
		t.Fatalf("host cache counts: %+v", counts)
	}
	navigateCapabilityFixture(t, p)
	if old.textCacheProfile != nil {
		t.Fatal("closed realm retained profile")
	}
}
