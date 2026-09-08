package chrome152

import (
	"net/http"
	"strings"

	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/moreveal/mimic/chrome/152/generated"
	"github.com/moreveal/mimic/compatibility"
	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/state"
)

const (
	Milestone        = 152
	Version          = "152.0.7977.82"
	ChromiumCommit   = "d04cdb24d67b081f6cf80200ffc5233f44b61109"
	ChromiumRevision = 1669021
)

// Environment returns the canonical Windows x64 Stable profile for this
// compatibility bundle. Runtime state such as the wall clock is captured by
// the base constructor, while product identity is pinned here.
func environment() state.Environment {
	environment := state.ChromeDesktopWindows(state.Product{Name: "Chrome", Version: "152.0.0.0", FullVersion: Version, UserAgentProduct: "HeadlessChrome", UserAgentBrands: []state.UserAgentBrand{{Brand: "Not?A_Brand", Version: "24", FullVersion: "24.0.0.0"}, {Brand: "Chromium", Version: "152", FullVersion: Version}}})
	environment.Platform.Architecture = "x86_64"
	for _, name := range strings.Fields("background-fetch background-sync accelerometer gyroscope magnetometer screen-wake-lock clipboard-write payment-handler storage-access pointer-lock") {
		environment.Permissions[name] = "granted"
	}
	environment.Permissions["periodic-background-sync"] = "denied"
	environment.Features = map[string]bool{"GenericSensorExtraClasses": false, "WebNFC": false, "SystemWakeLock": false, "SpeakerSelection": false, "WebAppInstallation": false, "ApproximateGeolocationPermission": false}
	environment.Capabilities = state.Capabilities{
		StorageQuotaBytes: 10 * 1024 * 1024 * 1024,
		Devices:           state.DeviceCapabilities{Posture: "continuous"},
		Media: state.MediaCapabilities{
			Kinds:                []string{"audioinput", "videoinput", "audiooutput"},
			DecodingContentTypes: []string{`video/mp4; codecs="avc1.42E01E"`},
			SupportedConstraints: strings.Fields("aspectRatio autoGainControl brightness channelCount colorTemperature contrast deviceId displaySurface echoCancellation exposureCompensation exposureMode exposureTime facingMode focusDistance focusMode frameRate groupId height iso latency noiseSuppression pan pointsOfInterest resizeMode restrictOwnAudio sampleRate sampleSize saturation sharpness suppressLocalAudioPlayback tilt torch voiceIsolation whiteBalanceMode width zoom"),
		},
	}
	environment.Capabilities.KeyboardLayout = map[string]string{
		"Backquote": "`", "Minus": "-", "Equal": "=", "BracketLeft": "[", "BracketRight": "]",
		"Backslash": "\\", "IntlBackslash": "\\", "Semicolon": ";", "Quote": "'", "Comma": ",", "Period": ".", "Slash": "/",
	}
	for letter := 'A'; letter <= 'Z'; letter++ {
		environment.Capabilities.KeyboardLayout["Key"+string(letter)] = strings.ToLower(string(letter))
	}
	for digit := '0'; digit <= '9'; digit++ {
		environment.Capabilities.KeyboardLayout["Digit"+string(digit)] = string(digit)
	}
	environment.Platform.OSVersion = "19.0.0"
	environment.Hardware.LogicalProcessors = 28
	environment.Hardware.DeviceMemoryGB = 32
	environment.Display.PhysicalWidth = 800
	environment.Display.PhysicalHeight = 600
	environment.Display.AvailableWidth = 800
	environment.Display.AvailableHeight = 600
	environment.Window.OuterWidth = 780
	environment.Window.OuterHeight = 580
	environment.Window.ViewportWidth = 772
	environment.Window.ViewportHeight = 433
	environment.Locale.Languages = []string{"ru-RU"}
	environment.Locale.IntlLocale = "ru"
	environment.Locale.Timezone = "Asia/Tbilisi"
	// Live transport phases are measured, not rescaled to match one capture.
	// Browser-visible Resource Timing, CDP timing and scheduler delivery all use
	// those same measurements. Machine-dependent synthetic latency belongs in a
	// separately selected environment profile, never in Chrome-version data.
	environment.Time.ExecutionScale = 1
	environment.Time.NavigationScale = 1
	environment.Time.NetworkScale = 1
	environment.Graphics.WebGPU.Vendor = "nvidia"
	environment.Graphics.WebGPU.Architecture = "blackwell"
	// This latency belongs to the pinned Windows/headless differential machine,
	// not to Chrome 152. It models asynchronous graphics-process adapter
	// discovery without requiring a GPU in Mimic.
	environment.Graphics.WebGPU.InitializationDelayMillis = 250
	environment.Network.WireProfile = "chrome-152-windows-x64"
	// These are properties of the pinned differential-test machine/network,
	// not of a target site. They remain canonical and externally configurable
	// state, while the Chrome 152 bundle supplies the reproducible default.
	environment.Network.ICE.HostCandidateCount = 3
	environment.Network.ICE.PortOffsets = []int{0, 2, 7}
	// Host and server-reflexive counts are independent environment facts;
	// neither is a Chrome-version constant. This reproducible profile selects
	// the three mappings observed in its baseline capture even though live
	// differential runs may see fewer transient STUN results.
	environment.Network.ICE.ReflexiveCandidateCount = 3
	environment.Network.ICE.ReflexivePortOffsets = []int{0, 7, 2}
	environment.Network.ICE.PublicAddress = "94.43.38.7"
	environment.Network.ICE.NetworkCost = 999
	environment.Network.ICE.HostDelayMillis = 21
	environment.Network.ICE.ReflexiveDelayMillis = 30
	environment.Network.ICE.EndDelayMillis = 130
	return environment
}

func newTransport() (http.RoundTripper, error) {
	// The PSK-capable profile omits extension 41 on a cold connection and emits
	// a real pre_shared_key only after this session's cache has received a TLS
	// ticket. This preserves Chrome's cold fingerprint while enabling genuine
	// resumption on a later connection; it is not a declared/faked fingerprint.
	return network.NewTLSClientTransport(profiles.Chrome_152_PSK,
		tls_client.WithNotFollowRedirects(),
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithRandomTLSExtensionOrder(),
		tls_client.WithProtocolRacing(),
		tls_client.WithTransportOptions(&tls_client.TransportOptions{DisableCompression: true}),
	)
}

type Bundle struct{}

func New() *Bundle { return &Bundle{} }

func (*Bundle) Version() compatibility.ChromeVersion {
	return compatibility.ChromeVersion{Milestone: Milestone, Version: Version, ChromiumCommit: ChromiumCommit, ChromiumRevision: ChromiumRevision}
}
func (*Bundle) Surface() *compatibility.WebAPISurface {
	return &compatibility.WebAPISurface{
		GeneratedJavaScript: generated.Surface,
		Exposures: map[string]compatibility.RealmExposure{
			"window.insecure.non-isolated": generated.InsecureWindowExposure(),
			"window.secure.non-isolated":   generated.SecureWindowExposure(),
			"window.secure.isolated":       generated.SecureIsolatedWindowExposure(),
			"worker.secure.non-isolated":   generated.SecureWorkerExposure(),
		},
	}
}
func (*Bundle) CDP() *compatibility.ProtocolSchema {
	return &compatibility.ProtocolSchema{Methods: generated.ProtocolMethods(), Events: generated.ProtocolEvents()}
}
func (*Bundle) Environment() *compatibility.EnvironmentProfile {
	return &compatibility.EnvironmentProfile{State: environment(), NewTransport: newTransport}
}
func (*Bundle) Expectations() *compatibility.CompatExpectations {
	return &compatibility.CompatExpectations{Platform: "windows-x64", Channel: "stable"}
}
