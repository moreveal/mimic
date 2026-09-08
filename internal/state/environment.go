package state

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type UserAgentBrand struct{ Brand, Version, FullVersion string }
type Product struct {
	Name, Version, FullVersion string
	// UserAgentProduct is the product token exposed consistently in the
	// legacy user agent and HTTP User-Agent header. Empty means Chrome.
	UserAgentProduct string
	UserAgentBrands  []UserAgentBrand
}
type Platform struct {
	OS, OSVersion, Architecture string
	Mobile                      bool
}
type Hardware struct {
	LogicalProcessors int
	DeviceMemoryGB    float64
}
type Display struct {
	PhysicalWidth, PhysicalHeight   int
	AvailableWidth, AvailableHeight int
	DeviceScaleFactor               float64
	ColorDepth                      int
}
type Window struct{ X, Y, OuterWidth, OuterHeight, ViewportWidth, ViewportHeight int }
type Graphics struct {
	Vendor, Renderer string
	MaxTextureSize   int
	WebGPU           GPUAdapter
}
type GPUAdapter struct {
	Vendor, Architecture, Device, Description string
	Features                                  []string
	// InitializationDelayMillis is selected by the machine/environment profile.
	// Adapter discovery is asynchronous in Chrome and depends on the graphics
	// process, driver and host state; it is not a browser-version constant.
	InitializationDelayMillis float64
}
type Locale struct {
	Languages  []string
	IntlLocale string
	Timezone   string
}
type Preferences struct {
	ColorScheme   string
	ReducedMotion bool
	DoNotTrack    bool
}
type Time struct {
	WallOrigin      time.Time
	MonotonicOrigin time.Duration
	// The virtual browser clock is independent of host implementation cost.
	// These factors map host execution and transport durations into the pinned
	// compatibility profile's observable timeline.
	ExecutionScale  float64
	NavigationScale float64
	NetworkScale    float64
}
type Network struct {
	SaveData                bool
	Online                  bool
	EffectiveType           string
	DownlinkMbps, RTTMillis float64
	CookiesEnabled          bool
	// WireProfile identifies the low-level TLS/HTTP behavior which must agree
	// with Product. Its implementation is supplied by the compatibility bundle.
	WireProfile string
	// ICE is canonical network state observed by WebRTC. Candidate generation
	// must project this state instead of inventing an unrelated network view in
	// the JavaScript binding.
	ICE ICEProfile
}
type ICEProfile struct {
	HostCandidateCount      int
	ReflexiveCandidateCount int
	// PortOffsets describes socket allocation relative to the first host
	// candidate and therefore belongs to canonical network state.
	PortOffsets []int
	// ReflexivePortOffsets preserves the host/network stack's observable STUN
	// completion order. It may differ from local candidate enumeration order.
	ReflexivePortOffsets []int
	PublicAddress        string
	NetworkCost          int
	// Candidate discovery timing is machine/network state. It is kept apart
	// from browser-version rules and reproduced only by the selected profile.
	HostDelayMillis      float64
	ReflexiveDelayMillis float64
	EndDelayMillis       float64
}
type Permissions map[string]string

type BrowserMode string

const (
	BrowserModeHeadful  BrowserMode = "headful"
	BrowserModeHeadless BrowserMode = "headless"
)

type Presentation struct {
	// Mode is part of the selected environment, never inferred from individual
	// Navigator or Window properties.
	Mode BrowserMode
}

type Environment struct {
	ProfileID    string
	Presentation Presentation
	Product      Product
	Platform     Platform
	Hardware     Hardware
	Display      Display
	Window       Window
	Graphics     Graphics
	Locale       Locale
	Preferences  Preferences
	Time         Time
	Network      Network
	Permissions  Permissions
	Capabilities Capabilities
	// Explicit feature overrides restrict the bundle's captured exposure.
	Features map[string]bool
}

func ChromeDesktopWindows(product Product) Environment {
	// The product profile is versioned, but wall-clock time is execution state,
	// not the Chrome release date. Capture it once so Date, Performance and the
	// scheduler all project the same current clock.
	now := time.Now().UTC()
	if len(product.UserAgentBrands) == 0 {
		major := strings.SplitN(product.Version, ".", 2)[0]
		product.UserAgentBrands = []UserAgentBrand{{"Not?A_Brand", "24", "24.0.0.0"}, {"Chromium", major, product.FullVersion}, {"Google Chrome", major, product.FullVersion}}
	}
	if product.UserAgentProduct == "" {
		product.UserAgentProduct = "Chrome"
	}
	return Environment{
		ProfileID:    "chrome-desktop-windows-headful-default",
		Presentation: Presentation{Mode: BrowserModeHeadful},
		Product:      product,
		Platform:     Platform{"Windows", "10.0.0", "x86", false},
		Hardware:     Hardware{8, 8},
		Display:      Display{PhysicalWidth: 1920, PhysicalHeight: 1080, AvailableWidth: 1920, AvailableHeight: 1040, DeviceScaleFactor: 1, ColorDepth: 24},
		Window:       Window{0, 0, 1280, 800, 1280, 720},
		Graphics: Graphics{
			Vendor: "Google Inc. (Intel)", Renderer: "ANGLE (Intel, Intel(R) UHD Graphics Direct3D11 vs_5_0 ps_5_0, D3D11)", MaxTextureSize: 16384,
			WebGPU: GPUAdapter{Vendor: "intel", Architecture: "gen-12lp", Features: []string{"bgra8unorm-storage", "clip-distances", "core-features-and-limits", "depth-clip-control", "depth32float-stencil8", "dual-source-blending", "float32-blendable", "float32-filterable", "indirect-first-instance", "primitive-index", "rg11b10ufloat-renderable", "shader-f16", "subgroup-size-control", "subgroups", "texture-component-swizzle", "texture-compression-bc", "texture-compression-bc-sliced-3d", "texture-formats-tier1", "texture-formats-tier2", "timestamp-query"}},
		},
		Locale:      Locale{Languages: []string{"en-US", "en"}, IntlLocale: "en-US", Timezone: "UTC"},
		Preferences: Preferences{ColorScheme: "dark"},
		Time:        Time{WallOrigin: now, ExecutionScale: 1, NavigationScale: 1, NetworkScale: 1},
		Network:     Network{Online: true, EffectiveType: "4g", DownlinkMbps: 10, RTTMillis: 50, CookiesEnabled: true, ICE: ICEProfile{HostCandidateCount: 1, PortOffsets: []int{0}, NetworkCost: 999}},
		Permissions: Permissions{"geolocation": "prompt", "notifications": "default"},
	}
}

func (e Environment) Validate() error {
	if e.ProfileID == "" {
		return fmt.Errorf("environment profile ID is required")
	}
	if e.Presentation.Mode != BrowserModeHeadful && e.Presentation.Mode != BrowserModeHeadless {
		return fmt.Errorf("browser presentation mode must be headful or headless")
	}
	if e.Product.Name != "Chrome" || e.Product.Version == "" {
		return fmt.Errorf("product must name a concrete Chrome version")
	}
	if e.Display.PhysicalWidth <= 0 || e.Display.PhysicalHeight <= 0 || e.Display.DeviceScaleFactor <= 0 {
		return fmt.Errorf("invalid display")
	}
	if e.Window.ViewportWidth <= 0 || e.Window.ViewportHeight <= 0 || e.Window.OuterWidth < e.Window.ViewportWidth || e.Window.OuterHeight < e.Window.ViewportHeight {
		return fmt.Errorf("incoherent window and viewport")
	}
	if e.Hardware.LogicalProcessors < 1 || len(e.Locale.Languages) == 0 {
		return fmt.Errorf("incomplete hardware or locale")
	}
	if _, err := time.LoadLocation(e.Locale.Timezone); err != nil {
		return fmt.Errorf("timezone: %w", err)
	}
	if e.Preferences.ColorScheme != "light" && e.Preferences.ColorScheme != "dark" {
		return fmt.Errorf("color scheme must be light or dark")
	}
	if e.Time.WallOrigin.IsZero() {
		return fmt.Errorf("wall clock origin is required")
	}
	if e.Time.ExecutionScale <= 0 || e.Time.NavigationScale <= 0 || e.Time.NetworkScale <= 0 {
		return fmt.Errorf("clock scales must be positive")
	}
	if e.Network.ICE.HostCandidateCount < 0 || e.Network.ICE.ReflexiveCandidateCount < 0 || e.Network.ICE.NetworkCost < 0 {
		return fmt.Errorf("invalid ICE profile")
	}
	if e.Graphics.WebGPU.InitializationDelayMillis < 0 {
		return fmt.Errorf("invalid WebGPU initialization delay")
	}
	if e.Network.RTTMillis < 0 {
		return fmt.Errorf("invalid network timing profile")
	}
	if e.Network.ICE.HostDelayMillis < 0 || e.Network.ICE.ReflexiveDelayMillis < e.Network.ICE.HostDelayMillis || e.Network.ICE.EndDelayMillis < e.Network.ICE.ReflexiveDelayMillis {
		return fmt.Errorf("invalid ICE candidate timing profile")
	}
	if len(e.Network.ICE.PortOffsets) > 0 && len(e.Network.ICE.PortOffsets) != e.Network.ICE.HostCandidateCount {
		return fmt.Errorf("ICE port offsets must match host candidate count")
	}
	if len(e.Network.ICE.ReflexivePortOffsets) > 0 && len(e.Network.ICE.ReflexivePortOffsets) != e.Network.ICE.ReflexiveCandidateCount {
		return fmt.Errorf("ICE reflexive port offsets must match reflexive candidate count")
	}
	return nil
}

type NavigatorView struct {
	UserAgent, Platform   string
	Languages             []string
	HardwareConcurrency   int
	DeviceMemory          float64
	Online, CookieEnabled bool
}

func (e Environment) Navigator() NavigatorView {
	os := "Windows NT 10.0; Win64; x64"
	return NavigatorView{
		UserAgent: fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) %s/%s Safari/537.36", os, e.Product.UserAgentProduct, e.Product.Version),
		Platform:  "Win32", Languages: append([]string(nil), e.Locale.Languages...), HardwareConcurrency: e.Hardware.LogicalProcessors, DeviceMemory: e.Hardware.DeviceMemoryGB, Online: e.Network.Online, CookieEnabled: e.Network.CookiesEnabled,
	}
}

type ScreenView struct {
	Width, Height, AvailWidth, AvailHeight, ColorDepth, PixelDepth int
	DevicePixelRatio                                               float64
}

func (e Environment) Screen() ScreenView {
	d := e.Display.DeviceScaleFactor
	availableWidth, availableHeight := e.Display.AvailableWidth, e.Display.AvailableHeight
	if availableWidth <= 0 {
		availableWidth = e.Display.PhysicalWidth
	}
	if availableHeight <= 0 {
		availableHeight = e.Display.PhysicalHeight
	}
	return ScreenView{int(math.Round(float64(e.Display.PhysicalWidth) / d)), int(math.Round(float64(e.Display.PhysicalHeight) / d)), int(math.Round(float64(availableWidth) / d)), int(math.Round(float64(availableHeight) / d)), e.Display.ColorDepth, e.Display.ColorDepth, d}
}
func (e Environment) RequestHeaders() map[string]string {
	n := e.Navigator()
	languages := ""
	for index, language := range e.Locale.Languages {
		if index == 0 {
			languages = language
		} else {
			languages += fmt.Sprintf(",%s;q=%.1f", language, math.Max(.1, 1-float64(index)/10))
		}
	}
	if len(e.Locale.Languages) == 1 {
		if base, _, found := strings.Cut(e.Locale.Languages[0], "-"); found {
			languages += "," + base + ";q=0.9"
		}
	}
	brands := make([]string, 0, len(e.Product.UserAgentBrands))
	for _, brand := range e.Product.UserAgentBrands {
		brands = append(brands, fmt.Sprintf(`%q;v=%q`, brand.Brand, brand.Version))
	}
	return map[string]string{"User-Agent": n.UserAgent, "Accept-Language": languages, "Sec-CH-UA": strings.Join(brands, ", "), "Sec-CH-UA-Mobile": "?0", "Sec-CH-UA-Platform": `"Windows"`}
}

// ClientHintHeaders projects only hints accepted by the origin. This keeps
// network-visible identity tied to the same Product and Platform as Navigator.
func (e Environment) ClientHintHeaders(accepted map[string]bool) map[string]string {
	full := e.Product.FullVersion
	arch, bitness := "x86", "64"
	if strings.Contains(e.Platform.Architecture, "arm") {
		arch = "arm"
	}
	if !strings.Contains(e.Platform.Architecture, "64") {
		bitness = "32"
	}
	fullBrands := make([]string, 0, len(e.Product.UserAgentBrands))
	for _, brand := range e.Product.UserAgentBrands {
		fullBrands = append(fullBrands, fmt.Sprintf(`%q;v=%q`, brand.Brand, brand.FullVersion))
	}
	values := map[string]string{
		"Sec-CH-UA-Arch":              fmt.Sprintf("%q", arch),
		"Sec-CH-UA-Bitness":           fmt.Sprintf("%q", bitness),
		"Sec-CH-UA-Full-Version":      fmt.Sprintf("%q", full),
		"Sec-CH-UA-Full-Version-List": strings.Join(fullBrands, ", "),
		"Sec-CH-UA-Model":             `""`,
		"Sec-CH-UA-Platform-Version":  fmt.Sprintf("%q", e.Platform.OSVersion),
	}
	out := map[string]string{}
	for name, value := range values {
		if accepted[strings.ToLower(name)] {
			out[name] = value
		}
	}
	return out
}
