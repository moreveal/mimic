//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestBootstrapSnapshotObservationalEquivalence(t *testing.T) {
	if os.Getenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT") == "1" {
		t.Skip("requires snapshot restoration")
	}
	source, err := os.ReadFile("testdata/bootstrap_snapshot_equivalence.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []struct {
		name             string
		secure, isolated bool
	}{
		{"insecure", false, false},
		{"secure", true, false},
		{"isolated", true, true},
	} {
		t.Run(profile.name, func(t *testing.T) {
			ordinary := bootstrapSnapshotPage(t)
			configure := func(p *Page) {
				address := "http://equivalence.invalid/"
				if profile.secure {
					address = "http://localhost/"
				}
				u, _ := url.Parse(address)
				p.mu.Lock()
				p.current = u
				p.documentSecurity = documentSecurity{secureContext: profile.secure, crossOriginIsolated: profile.isolated}
				p.Top.Realm.url = u
				p.Top.Realm.origin = originOf(address)
				p.mu.Unlock()
			}
			configure(ordinary)
			baseline := bootstrapSnapshotEvaluate(t, ordinary, string(source)).(string)
			if ordinary.Top.Realm.bootstrapRestored {
				t.Fatal("baseline unexpectedly restored")
			}
			childSource := "(()=>{const f=document.createElement('iframe');document.body.appendChild(f);const result=f.contentWindow.eval(" + strconv.Quote(string(source)) + ");f.remove();return result})()"
			baselineChild := bootstrapSnapshotEvaluate(t, ordinary, childSource).(string)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := ordinary.ctx.bootstrapSnapshots.wait(ctx); err != nil {
				t.Fatal(err)
			}
			restored, err := ordinary.ctx.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			configure(restored)
			actual := bootstrapSnapshotEvaluate(t, restored, string(source)).(string)
			if !restored.Top.Realm.bootstrapRestored {
				t.Fatal("candidate did not restore")
			}
			actualChild := bootstrapSnapshotEvaluate(t, restored, childSource).(string)
			bootstrapSnapshotAssertRestored(t, restored, 1)
			for _, pair := range []struct{ name, before, after string }{{"main", baseline, actual}, {"iframe", baselineChild, actualChild}} {
				var before, after any
				if err := json.Unmarshal([]byte(pair.before), &before); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(pair.after), &after); err != nil {
					t.Fatal(err)
				}
				if difference := bootstrapSnapshotDifference("$", before, after); difference != "" {
					for _, direction := range []struct {
						label    string
						from, to any
					}{{"ordinary-only", before, after}, {"restored-only", after, before}} {
						keys := map[string]bool{}
						for _, row := range direction.to.(map[string]any)["globals"].([]any) {
							keys[fmt.Sprint(row.(map[string]any)["key"])] = true
						}
						for _, row := range direction.from.(map[string]any)["globals"].([]any) {
							key := fmt.Sprint(row.(map[string]any)["key"])
							if !keys[key] {
								t.Logf("%s %s global %s", pair.name, direction.label, key)
							}
						}
					}
					t.Errorf("%s: %s", pair.name, difference)
				}
			}
		})
	}
}

func bootstrapSnapshotDifference(path string, a, b any) string {
	if reflect.DeepEqual(a, b) {
		return ""
	}
	if left, ok := a.([]any); ok {
		if right, ok := b.([]any); ok {
			if len(left) != len(right) {
				return fmt.Sprintf("%s length: ordinary=%d restored=%d", path, len(left), len(right))
			}
			for i := range left {
				if diff := bootstrapSnapshotDifference(fmt.Sprintf("%s[%d]", path, i), left[i], right[i]); diff != "" {
					return diff
				}
			}
		}
	}
	if left, ok := a.(map[string]any); ok {
		if right, ok := b.(map[string]any); ok {
			for k, v := range left {
				w, present := right[k]
				if !present {
					return path + " missing " + k
				}
				if diff := bootstrapSnapshotDifference(path+"."+k, v, w); diff != "" {
					return diff
				}
			}
			for k := range right {
				if _, ok := left[k]; !ok {
					return path + " added " + k
				}
			}
		}
	}
	return fmt.Sprintf("%s: ordinary=%v restored=%v", path, a, b)
}

// Locale and display observations deliberately do not select a different
// source profile: a consumer must read its own host state after restoration.
func TestBootstrapSnapshotRebindsChangedEnvironment(t *testing.T) {
	if os.Getenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT") == "1" {
		t.Skip("requires snapshot restoration")
	}
	seed := bootstrapSnapshotPage(t)
	bootstrapSnapshotWarm(t, seed)
	const observe = `(()=>({language:navigator.language,languages:[...navigator.languages],date:Intl.DateTimeFormat().resolvedOptions(),number:Intl.NumberFormat().resolvedOptions(),screen:[screen.width,screen.height,screen.availWidth,screen.availHeight,screen.colorDepth],viewport:[innerWidth,innerHeight,outerWidth,outerHeight,devicePixelRatio],media:[matchMedia('(prefers-color-scheme: dark)').matches,matchMedia('(prefers-reduced-motion: reduce)').matches]}))()`
	seedState := bootstrapSnapshotEvaluate(t, seed, observe)
	configure := func(p *Page) {
		p.mu.Lock()
		p.env.Locale.Languages = []string{"fr-CA", "fr", "en"}
		p.env.Locale.IntlLocale = "fr-CA"
		p.env.Locale.Timezone = "Europe/Paris"
		p.env.Display.PhysicalWidth = 1800
		p.env.Display.PhysicalHeight = 1200
		p.env.Display.AvailableWidth = 1790
		p.env.Display.AvailableHeight = 1150
		p.env.Display.ColorDepth = 30
		p.env.Display.DeviceScaleFactor = 1.5
		p.env.Window.OuterWidth = 1400
		p.env.Window.OuterHeight = 1000
		p.env.Preferences.ColorScheme = "dark"
		p.env.Preferences.ReducedMotion = true
		p.mu.Unlock()
		if err := p.SetViewport(811, 633); err != nil {
			t.Fatal(err)
		}
	}
	// A fresh, ordinary realm is the reference for the changed environment.
	control := bootstrapSnapshotPage(t)
	configure(control)
	t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "1")
	expected := bootstrapSnapshotEvaluate(t, control, observe)
	if control.Top.Realm.bootstrapRestored {
		t.Fatal("reference restored")
	}
	t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "")
	consumer, err := seed.ctx.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	configure(consumer)
	actual := bootstrapSnapshotEvaluate(t, consumer, observe)
	if !consumer.Top.Realm.bootstrapRestored {
		t.Fatal("changed environment did not reuse snapshot")
	}
	if seed.Top.Realm.bootstrapSource().key != consumer.Top.Realm.bootstrapSource().key {
		t.Fatal("changed environment selected another profile")
	}
	if reflect.DeepEqual(seedState, expected) {
		t.Fatal("environment mutation did not change observations")
	}
	if difference := bootstrapSnapshotDifference("$", expected, actual); difference != "" {
		t.Fatal(difference)
	}
	// Restoration must also leave the original live Page's environment intact.
	if difference := bootstrapSnapshotDifference("$", seedState, bootstrapSnapshotEvaluate(t, seed, observe)); difference != "" {
		t.Fatal("seed changed: " + difference)
	}
}
