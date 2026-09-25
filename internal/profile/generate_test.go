package profile

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestGeneratedProfileRoundtripAndDiversity(t *testing.T) {
	base := testBase()
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		raw := []byte(fmt.Sprintf(`{"seed":"account-%d","browser":"chrome","version":152,"platform":"windows"}`, i))
		d, descriptor, err := Generate(raw, base)
		if err != nil {
			t.Fatal(err)
		}
		next, restored, err := ResolveToken(descriptor.Token(), base)
		if err != nil || !reflect.DeepEqual(next, d) || restored.ProfileID != descriptor.ProfileID {
			t.Fatalf("roundtrip: %v", err)
		}
		if seen[descriptor.ProfileID] {
			t.Fatal("duplicate in fixed 1000-profile corpus", i)
		}
		seen[descriptor.ProfileID] = true
		if !reflect.DeepEqual(d.Graphics, base.Graphics) || !reflect.DeepEqual(d.Hardware, base.Hardware) || !reflect.DeepEqual(d.Fonts, base.Fonts) {
			t.Fatal("invented device recipe")
		}
		if d.Window.OuterWidth > d.Display.AvailableWidth || d.Window.OuterHeight > d.Display.AvailableHeight {
			t.Fatal("window exceeds recipe bounds")
		}
		if d.Window.X < 0 || d.Window.Y < 0 || d.Window.X+d.Window.OuterWidth > d.Display.AvailableWidth || d.Window.Y+d.Window.OuterHeight > d.Display.AvailableHeight {
			t.Fatal("window position exceeds available display")
		}
		if d.Audio.SampleRate != 44100 && d.Audio.SampleRate != 48000 {
			t.Fatal("unsupported audio device rate")
		}
	}
	_, a, err := Generate([]byte(`{}`), base)
	if err != nil {
		t.Fatal(err)
	}
	_, b, err := Generate([]byte(`{}`), base)
	if err != nil {
		t.Fatal(err)
	}
	if a.Seed == b.Seed || len(a.Seed) != 64 {
		t.Fatal("random seed generation")
	}
}

func TestProfileGenerationRejectsInvalidInputs(t *testing.T) {
	for _, raw := range []string{`{"seed":""}`, `{"seed":null}`, `{"version":0}`, `{"version":151}`, `{"version":"152"}`, `{"platform":"linux"}`, `{"browser":""}`, `{"browser":"firefox"}`, `{"extra":1}`} {
		if _, _, err := Generate([]byte(raw), testBase()); err == nil {
			t.Fatal("accepted", raw)
		}
	}
	_, descriptor, _ := Generate([]byte(`{"seed":"fixed"}`), testBase())
	descriptor.ProfileID = "tampered"
	if _, _, err := ResolveToken(descriptor.Token(), testBase()); err == nil {
		t.Fatal("accepted tampering")
	}
}

func TestManualImportChecksAndRoundtrip(t *testing.T) {
	base := testBase()
	d, descriptor, err := ImportManual([]byte(`{"hardware":{"logicalProcessors":4},"locale":{"languages":["fr-FR","en"]}}`), base)
	if err != nil {
		t.Fatal(err)
	}
	roundtrip, _, err := ResolveToken(descriptor.Token(), base)
	if err != nil || !reflect.DeepEqual(d, roundtrip) {
		t.Fatalf("manual roundtrip: %v", err)
	}
	for _, raw := range []string{`{"window":{"viewportWidth":99999}}`, `{"graphics":{"renderer":"fake"}}`, `{"graphics":{"webGPU":{"vendor":"fake"}}}`, `{"fonts":{"sansSerif":"fake"}}`, `{"identity":{"platform":"Linux"}}`, `{"network":{"proxy":{}}}`, `{"schemaVersion":1}`} {
		if _, _, err := ImportManual([]byte(raw), base); err == nil {
			t.Fatal("accepted", raw)
		}
	}
	descriptor.Environment["hardware"].(map[string]any)["logicalProcessors"] = 7
	raw, _ := json.Marshal(descriptor)
	if _, _, err := Restore(raw, base); err == nil {
		t.Fatal("accepted modified export without explicit manual import")
	}
}

func TestManualAudioUsesModeledOutputDevice(t *testing.T) {
	base := testBase()
	d, descriptor, err := ImportManual([]byte(`{"audio":{"sampleRate":44100}}`), base)
	if err != nil || d.Audio.SampleRate != 44100 || d.Apply(base).Audio.SampleRate != 44100 {
		t.Fatalf("manual audio output: %v", err)
	}
	restored, _, err := ResolveToken(descriptor.Token(), base)
	if err != nil || restored.Audio != d.Audio {
		t.Fatalf("audio descriptor roundtrip: %v", err)
	}
	for _, raw := range []string{`{"audio":{"sampleRate":96000}}`, `{"audio":{"channels":8}}`, `{"audio":{"bufferDuration":0}}`} {
		if _, _, err := ImportManual([]byte(raw), base); err == nil {
			t.Fatal("accepted unsupported audio recipe", raw)
		}
	}
}

func TestProxyDoesNotChangeEnvironmentIdentity(t *testing.T) {
	base := testBase()
	d, _, err := Generate([]byte(`{"seed":"same-device"}`), base)
	if err != nil {
		t.Fatal(err)
	}
	want := d.Apply(base).ProfileID
	d.Network.Proxy = Proxy{Server: "socks5://localhost:1080", Username: "private", Password: "secret"}
	if got := d.Apply(base).ProfileID; got != want {
		t.Fatal("routing changed environment identity")
	}
}

func TestGenerationModeDoesNotChangeProfileIdentity(t *testing.T) {
	base := testBase()
	d, generated, err := Generate([]byte(`{"seed":"same-device"}`), base)
	if err != nil {
		t.Fatal(err)
	}
	fields := d.Public()
	delete(fields, "schemaVersion")
	delete(fields, "baseProfile")
	delete(fields["network"].(map[string]any), "proxy")
	raw, _ := json.Marshal(fields)
	manualDoc, manual, err := ImportManual(raw, base)
	if err != nil || generated.ProfileID != manual.ProfileID || !reflect.DeepEqual(d, manualDoc) {
		t.Fatalf("mode changed profile identity: %v", err)
	}
}
