package generated

import (
	_ "embed"
	"encoding/json"
	"sync"

	"github.com/moreveal/mimic/compatibility"
)

// Window exposure is captured from the exact browser named by target.json.
// It deliberately lives outside bundle_data.go because that file is replaced
// by the IDL/CDP generator.

//go:embed window-secure.json
var windowSecureJSON []byte

//go:embed window-secure-isolated.json
var windowSecureIsolatedJSON []byte

//go:embed worker-secure.json
var workerSecureJSON []byte

//go:embed window-insecure.json
var windowInsecureJSON []byte

var windowInsecureOnce = sync.OnceValue(func() compatibility.RealmExposure {
	return parseWindowExposure(windowInsecureJSON)
})

func InsecureWindowExposure() compatibility.RealmExposure { return windowInsecureOnce() }

func parseWindowExposure(raw []byte) compatibility.RealmExposure {
	var capture struct {
		Properties []compatibility.SurfaceProperty            `json:"properties"`
		Prototypes map[string][]compatibility.SurfaceProperty `json:"prototypes"`
	}
	if err := json.Unmarshal(raw, &capture); err != nil {
		panic("invalid generated Chrome 152 Window exposure: " + err.Error())
	}
	return compatibility.RealmExposure{Properties: capture.Properties, Prototypes: capture.Prototypes}
}

var windowSecureOnce = sync.OnceValue(func() compatibility.RealmExposure {
	return parseWindowExposure(windowSecureJSON)
})

func SecureWindowExposure() compatibility.RealmExposure { return windowSecureOnce() }

var windowSecureIsolatedOnce = sync.OnceValue(func() compatibility.RealmExposure {
	return parseWindowExposure(windowSecureIsolatedJSON)
})

func SecureIsolatedWindowExposure() compatibility.RealmExposure { return windowSecureIsolatedOnce() }

var workerSecureOnce = sync.OnceValue(func() compatibility.RealmExposure {
	return parseWindowExposure(workerSecureJSON)
})

func SecureWorkerExposure() compatibility.RealmExposure { return workerSecureOnce() }
