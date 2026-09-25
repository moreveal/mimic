package profile

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/moreveal/mimic/internal/state"
)

// Generation keeps the captured graphics/font/device and wire recipe intact.
// Window placement, viewport, preferences, and audio output rate are modeled
// independently by the same environment that backs all observable surfaces.
type Generation struct {
	Browser  string `json:"browser,omitempty"`
	Version  int    `json:"version,omitempty"`
	Platform string `json:"platform,omitempty"`
	Seed     string `json:"seed,omitempty"`
}

// Descriptor is portable data, not an API-version selector. ProfileID commits
// to the resolved environment: a changed bundle or generator fails restoration
// instead of silently changing the identity of a saved profile.
type Descriptor struct {
	Mode        string         `json:"mode"`
	BaseProfile string         `json:"baseProfile"`
	Seed        string         `json:"seed,omitempty"`
	ProfileID   string         `json:"profileId"`
	Environment map[string]any `json:"environment,omitempty"`
}

func strictObject(raw []byte, target any, fields ...string) error {
	obj, err := Decode(raw)
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, field := range fields {
		allowed[field] = true
	}
	for key, value := range obj {
		if !allowed[key] {
			return failure(key, "unknownField", "unknown profile parameter")
		}
		if value == nil {
			return failure(key, "invalidType", "null is not accepted")
		}
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return failure("$", "invalidType", "invalid profile parameter type")
	}
	return nil
}

func Generate(raw []byte, base state.Environment) (Document, Descriptor, error) {
	if base.Product.Name != "Chrome" || !strings.HasPrefix(base.Product.Version, "152.") || base.Platform.OS != "Windows" {
		return Document{}, Descriptor{}, failure("generate", "unsupported", "generator requires the installed Chrome 152 Windows bundle")
	}
	var options Generation
	if err := strictObject(raw, &options, "browser", "version", "platform", "seed"); err != nil {
		return Document{}, Descriptor{}, err
	}
	obj, _ := Decode(raw)
	if (options.Browser != "" && options.Browser != "chrome") || (options.Platform != "" && options.Platform != "windows") || (options.Version != 0 && (options.Version != 152 || !strings.HasPrefix(base.Product.Version, "152."))) {
		return Document{}, Descriptor{}, failure("generate", "unsupported", "select the installed Chrome 152 Windows bundle")
	}
	for _, key := range []string{"browser", "platform"} {
		if obj[key] == "" {
			return Document{}, Descriptor{}, failure(key, "invalidValue", "empty selector")
		}
	}
	if _, exists := obj["version"]; exists && options.Version == 0 {
		return Document{}, Descriptor{}, failure("version", "unsupported", "select the installed milestone")
	}
	if _, exists := obj["seed"]; exists && options.Seed == "" {
		return Document{}, Descriptor{}, failure("seed", "invalidValue", "seed must be nonempty")
	}
	if len(options.Seed) > 1024 {
		return Document{}, Descriptor{}, failure("seed", "invalidValue", "seed exceeds 1024 bytes")
	}
	if options.Seed == "" {
		var random [32]byte
		if _, err := rand.Read(random[:]); err != nil {
			return Document{}, Descriptor{}, err
		}
		options.Seed = hex.EncodeToString(random[:])
	}
	d := FromEnvironment(base, base.ProfileID, Proxy{})
	// Preserve the measured window chrome insets and screen/device recipe.
	dx := d.Window.OuterWidth - d.Window.ViewportWidth
	dy := d.Window.OuterHeight - d.Window.ViewportHeight
	maxWidth, maxHeight := d.Display.AvailableWidth-dx, d.Display.AvailableHeight-dy
	if maxWidth < 800 || maxHeight < 480 {
		return Document{}, Descriptor{}, failure("display", "unsupported", "baseline does not support the desktop window recipe")
	}
	choose := func(group string, count int) int {
		h := sha256.Sum256([]byte("mimic-profile\x00" + base.ProfileID + "\x00" + options.Seed + "\x00" + group))
		return int(binary.LittleEndian.Uint64(h[:8]) % uint64(count))
	}
	d.Window.ViewportWidth = 800 + choose("width", maxWidth-800+1)
	d.Window.ViewportHeight = 480 + choose("height", maxHeight-480+1)
	d.Window.OuterWidth = d.Window.ViewportWidth + dx
	d.Window.OuterHeight = d.Window.ViewportHeight + dy
	d.Window.X = choose("window-x", d.Display.AvailableWidth-d.Window.OuterWidth+1)
	d.Window.Y = choose("window-y", d.Display.AvailableHeight-d.Window.OuterHeight+1)
	d.Preferences.ColorScheme = []string{"light", "dark"}[choose("theme", 2)]
	d.Preferences.ReducedMotion = choose("motion", 2) == 1
	d.Audio.SampleRate = []float64{44100, 48000}[choose("audio-output-rate", 2)]
	if err := d.Validate(base); err != nil {
		return Document{}, Descriptor{}, err
	}
	return d, Descriptor{Mode: "generated", BaseProfile: base.ProfileID, Seed: options.Seed, ProfileID: d.fingerprintID()}, nil
}

func Restore(raw []byte, base state.Environment) (Document, Descriptor, error) {
	var saved Descriptor
	if err := strictObject(raw, &saved, "mode", "baseProfile", "seed", "profileId", "environment"); err != nil {
		return Document{}, Descriptor{}, err
	}
	if saved.BaseProfile != base.ProfileID || saved.ProfileID == "" {
		return Document{}, Descriptor{}, failure("profile", "incompatibleProfile", "incomplete descriptor or unavailable base profile")
	}
	if saved.Mode == "manual" {
		if saved.Seed != "" {
			return Document{}, Descriptor{}, failure("seed", "invalidValue", "manual profiles have no seed")
		}
		input, _ := json.Marshal(saved.Environment)
		d, resolved, err := ImportManual(input, base)
		if err != nil {
			return Document{}, Descriptor{}, err
		}
		if resolved.ProfileID != saved.ProfileID {
			return Document{}, Descriptor{}, failure("profile", "incompatibleProfile", "manual profile hash differs")
		}
		return d, resolved, nil
	}
	if saved.Mode != "generated" || saved.Seed == "" || saved.Environment != nil {
		return Document{}, Descriptor{}, failure("profile", "invalidValue", "expected a generated descriptor")
	}
	input, _ := json.Marshal(Generation{Seed: saved.Seed})
	d, resolved, err := Generate(input, base)
	if err != nil {
		return Document{}, Descriptor{}, err
	}
	if resolved.ProfileID != saved.ProfileID {
		return Document{}, Descriptor{}, failure("profile", "incompatibleProfile", "resolved profile differs; regenerate explicitly for this Mimic release")
	}
	return d, resolved, nil
}

func (d Descriptor) Token() string {
	raw, _ := json.Marshal(d)
	return "mimic:" + base64.RawURLEncoding.EncodeToString(raw)
}

func ResolveToken(token string, base state.Environment) (Document, Descriptor, error) {
	if len(token) > 1<<20 || !strings.HasPrefix(token, "mimic:") {
		return Document{}, Descriptor{}, failure("profile", "invalidValue", "expected a Mimic profile token (maximum 1 MiB)")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(token, "mimic:"))
	if err != nil {
		return Document{}, Descriptor{}, failure("profile", "invalidValue", "invalid profile token")
	}
	return Restore(raw, base)
}

// ImportManual exposes only existing modeled surfaces. Graphics and fonts stay
// at baseline until complete alternative recipes can be validated together.
func ImportManual(raw []byte, base state.Environment) (Document, Descriptor, error) {
	obj, err := Decode(raw)
	if err != nil {
		return Document{}, Descriptor{}, err
	}
	for _, key := range []string{"schemaVersion", "baseProfile"} {
		if _, exists := obj[key]; exists {
			return Document{}, Descriptor{}, failure(key, "unknownField", "profile format follows the installed Mimic release")
		}
	}
	if network, ok := obj["network"].(map[string]any); ok {
		if _, exists := network["proxy"]; exists {
			return Document{}, Descriptor{}, failure("network.proxy", "unknownField", "set proxy on createContext")
		}
	}
	obj["schemaVersion"], obj["baseProfile"] = 1, base.ProfileID
	encoded, _ := json.Marshal(obj)
	d, err := Normalize(encoded, base, nil)
	if err != nil {
		return Document{}, Descriptor{}, err
	}
	baseline := FromEnvironment(base, base.ProfileID, Proxy{})
	for key, pair := range map[string][2]any{"graphics": {d.Graphics, baseline.Graphics}, "fonts": {d.Fonts, baseline.Fonts}, "identity": {d.Identity, baseline.Identity}} {
		a, _ := json.Marshal(wire(reflect.ValueOf(pair[0])))
		b, _ := json.Marshal(wire(reflect.ValueOf(pair[1])))
		if string(a) != string(b) {
			return Document{}, Descriptor{}, failure(key, "unsupported", "manual changes require a complete validated recipe; keep the installed baseline")
		}
	}
	environment := d.Public()
	delete(environment, "schemaVersion")
	delete(environment, "baseProfile")
	delete(environment["network"].(map[string]any), "proxy")
	return d, Descriptor{Mode: "manual", BaseProfile: base.ProfileID, ProfileID: d.fingerprintID(), Environment: environment}, nil
}

func Warnings(mode string) []string {
	if mode == "manual" {
		return []string{"Manual mode: do not combine inconsistent surfaces. Known invalid combinations are rejected, but unmodeled cross-surface relationships cannot yet be certified.", "Graphics, fonts, browser identity and wire behavior remain the installed Chrome 152 recipe. A Context proxy does not route WebRTC or define locale/timezone."}
	}
	return []string{"Generated profiles preserve the installed Chrome 152 graphics, fonts, device and wire recipe. They vary window geometry, preferences and audio output rate; this does not certify equivalence to every physical machine or route WebRTC through the Context proxy."}
}

func ManualSchema(base state.Environment) map[string]any {
	schema := Schema(base)
	delete(schema, "required")
	properties := schema["properties"].(map[string]any)
	delete(properties, "schemaVersion")
	delete(properties, "baseProfile")
	delete(properties["network"].(map[string]any)["properties"].(map[string]any), "proxy")
	for _, key := range []string{"identity", "graphics", "fonts"} {
		properties[key].(map[string]any)["description"] = "Only unchanged installed-baseline values are accepted until another complete recipe is captured and validated."
	}
	properties["audio"].(map[string]any)["description"] = "Modeled stereo output device: 44100 or 48000 Hz; latency fields retain the installed recipe."
	schema["description"] = "Explicit manual import fields. Generated/imported Contexts are immutable; mutability annotations describe ordinary Page emulation only."
	return schema
}
