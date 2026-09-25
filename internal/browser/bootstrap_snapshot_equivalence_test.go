//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"github.com/moreveal/mimic/internal/profile"
)

func TestBootstrapSnapshotObservationalEquivalence(t *testing.T) {
	serialBrowserTest(t)
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
			if err := ordinary.ctx.browser.bootstrapSnapshots.wait(ctx); err != nil {
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
	serialBrowserTest(t)
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

func TestBootstrapSnapshotRebindsAcrossProfileContexts(t *testing.T) {
	serialBrowserTest(t)
	if os.Getenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT") == "1" {
		t.Skip("requires snapshot restoration")
	}
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	create := func(seed string) *Page {
		t.Helper()
		_, descriptor, err := profile.Generate([]byte(fmt.Sprintf(`{"seed":%q}`, seed)), b.env)
		if err != nil {
			t.Fatal(err)
		}
		c, err := b.NewProfileContext(descriptor.Token(), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.Close() })
		p, err := c.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	const observe = `(()=>{const canvas=document.createElement('canvas');const gl=canvas.getContext('webgl');gl.getExtension('WEBGL_debug_renderer_info');const a=new AudioContext();const twoD=document.createElement('canvas').getContext('2d');twoD.font='17px sans-serif';const value={screen:[screen.width,screen.height,screen.availWidth,screen.availHeight,screenX,screenY],window:[outerWidth,outerHeight,innerWidth,innerHeight,devicePixelRatio],preferences:[matchMedia('(prefers-color-scheme: dark)').matches,matchMedia('(prefers-reduced-motion: reduce)').matches],audio:[a.sampleRate,a.baseLatency],graphics:[gl.getParameter(37445),gl.getParameter(37446),gl.getParameter(3379)],font:[document.fonts.check('12px Arial'),twoD.measureText('Mimic 152').width],identity:[navigator.userAgent,navigator.platform,navigator.hardwareConcurrency]};a.close();return value})()`
	seed := create("snapshot-profile-seed")
	bootstrapSnapshotWarm(t, seed)
	baseline := bootstrapSnapshotEvaluate(t, seed, observe)
	control := create("snapshot-profile-consumer")
	t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "1")
	expected := bootstrapSnapshotEvaluate(t, control, observe)
	t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "")
	consumer := create("snapshot-profile-consumer")
	actual := bootstrapSnapshotEvaluate(t, consumer, observe)
	if got := actual.(map[string]any)["audio"].([]any)[0]; got != consumer.ctx.env.Audio.SampleRate {
		t.Fatalf("audio output rate diverged from profile: %v vs %v", got, consumer.ctx.env.Audio.SampleRate)
	}
	if !consumer.Top.Realm.bootstrapRestored {
		t.Fatal("different profile did not reuse browser snapshot")
	}
	if seed.Top.Realm.bootstrapSource().key != consumer.Top.Realm.bootstrapSource().key {
		t.Fatal("profile values selected another bootstrap artifact")
	}
	if reflect.DeepEqual(baseline, expected) {
		t.Fatal("distinct generated profiles did not change observations")
	}
	if difference := bootstrapSnapshotDifference("$", expected, actual); difference != "" {
		t.Fatal(difference)
	}
	if difference := bootstrapSnapshotDifference("$", baseline, bootstrapSnapshotEvaluate(t, seed, observe)); difference != "" {
		t.Fatal("seed profile changed: " + difference)
	}
}

func TestManagedProfilePreparesBootstrapBeforeFirstEvaluation(t *testing.T) {
	serialBrowserTest(t)
	if os.Getenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT") == "1" {
		t.Skip("requires snapshot restoration")
	}
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	_, descriptor, err := profile.Generate([]byte(`{"seed":"first-managed-page"}`), b.env)
	if err != nil {
		t.Fatal(err)
	}
	c, err := b.NewProfileContext(descriptor.Token(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	bootstrapSnapshotEvaluate(t, p, "true")
	if !p.Top.Realm.bootstrapRestored {
		t.Fatal("first managed Page did not restore the shared bootstrap graph")
	}
	if !b.bootstrapSnapshots.hasSnapshotKey(p.Top.Realm.bootstrapSource().key) {
		t.Fatal("managed Page used a missing bootstrap artifact")
	}
}

func TestManagedProfilesRestoreNavigatedDocument(t *testing.T) {
	serialBrowserTest(t)
	if os.Getenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT") == "1" {
		t.Skip("requires snapshot restoration")
	}
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<!doctype html><title>Profile document</title>"))
	}))
	defer server.Close()
	const observe = `(async()=>{const audio=new AudioContext();const adapter=await navigator.gpu.requestAdapter();const gl=document.createElement('canvas').getContext('webgl');gl.getExtension('WEBGL_debug_renderer_info');const result={title:document.title,audio:audio.sampleRate,gpu:adapter.info.vendor,webgl:gl.getParameter(37445),history:history.length,location:location.href,origin:location.origin};audio.close();return result})()`
	_, controlDescriptor, err := profile.Generate([]byte(`{"seed":"navigated-a"}`), b.env)
	if err != nil {
		t.Fatal(err)
	}
	control, err := b.NewProfileContext(controlDescriptor.Token(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "1")
	controlPage, err := control.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if err := controlPage.Navigate(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	expected := bootstrapSnapshotEvaluate(t, controlPage, observe)
	if err := control.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", "")
	var keys [2][32]byte
	for i, seed := range []string{"navigated-a", "navigated-b"} {
		_, descriptor, err := profile.Generate([]byte(fmt.Sprintf(`{"seed":%q}`, seed)), b.env)
		if err != nil {
			t.Fatal(err)
		}
		c, err := b.NewProfileContext(descriptor.Token(), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		p, err := c.NewPage()
		if err != nil {
			_ = c.Close()
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err = p.Navigate(ctx, server.URL)
		cancel()
		if err != nil {
			_ = c.Close()
			t.Fatal(err)
		}
		value := bootstrapSnapshotEvaluate(t, p, observe).(map[string]any)
		if value["title"] != "Profile document" || value["audio"] != c.env.Audio.SampleRate || value["gpu"] != c.env.Graphics.WebGPU.Vendor || value["webgl"] != c.env.Graphics.Vendor || !p.Top.Realm.bootstrapRestored {
			_ = c.Close()
			t.Fatalf("managed navigation did not restore its environment: value=%v restored=%t", value, p.Top.Realm.bootstrapRestored)
		}
		if i == 0 {
			if difference := bootstrapSnapshotDifference("$", expected, value); difference != "" {
				_ = c.Close()
				t.Fatal("navigated snapshot changed observations: " + difference)
			}
		}
		keys[i] = p.Top.Realm.bootstrapSource().key
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if keys[0] != keys[1] {
		t.Fatal("distinct generated identities selected different document graphs")
	}
}
