package webapi

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"

	"github.com/moreveal/mimic/compatibility"
)

// Surface is kept separate from semantic host implementations so it can be
// replaced by generated WebIDL bindings without changing browser services.
//
//go:embed surface.js
var handwrittenSurface string

//go:embed worker.js
var handwrittenWorkerSurface string

//go:embed capabilities.js
var capabilitySurface string

type surfaceKey struct {
	surface  *compatibility.WebAPISurface
	exposure string
}

type surfaceOutput struct{ Source, ExposureJSON string }

var surfaceSources = struct {
	sync.Mutex
	values map[surfaceKey]func() surfaceOutput
}{values: map[surfaceKey]func() surfaceOutput{}}

// SurfaceFor shares only immutable source text from a compatibility bundle.
// The bounded cache owns no Page, isolate, or mutable JavaScript object.
func SurfaceFor(surface *compatibility.WebAPISurface, name string) (string, string) {
	key := surfaceKey{surface, name}
	surfaceSources.Lock()
	build := surfaceSources.values[key]
	if build == nil {
		build = sync.OnceValue(func() surfaceOutput {
			exposure, ok := surface.Exposures[name]
			if !ok {
				return surfaceOutput{Source: Surface(surface.GeneratedJavaScript, nil)}
			}
			encoded, err := json.Marshal(exposure)
			if err != nil {
				panic(err)
			}
			return surfaceOutput{Source: composeSurface(surface.GeneratedJavaScript, "applyTargetExposure(JSON.parse(host.exposureJSON()));\n"), ExposureJSON: string(encoded)}
		})
		if len(surfaceSources.values) < 8 {
			surfaceSources.values[key] = build
		}
	}
	surfaceSources.Unlock()
	output := build()
	return output.Source, output.ExposureJSON
}
func Surface(generated string, exposure *compatibility.RealmExposure) string {
	return buildSurface(generated, exposure)
}
func buildSurface(generated string, exposure *compatibility.RealmExposure) string {
	// Tracking starts only after both handwritten semantics and the selected
	// bundle's generated surface have been installed. Otherwise feature
	// detection performed by the generator pollutes runtime API traces.
	exposureSource := ""
	if exposure != nil {
		encoded, err := json.Marshal(exposure)
		if err != nil {
			panic(err)
		}
		quoted, _ := json.Marshal(string(encoded))
		exposureSource = "applyTargetExposure(JSON.parse(" + string(quoted) + "));\n"
	}
	return composeSurface(generated, exposureSource)
}

func composeSurface(generated, exposureSource string) string {
	marker := "known=new Set(Reflect.ownKeys(globalThis));host.ready();"
	semanticFixups := `Object.defineProperty(CharacterData.prototype,'nodeName',{get(){const slot=elementSlot(this);return slot&&slot.type==='comment'?'#comment':'#text'},enumerable:true,configurable:true});Object.defineProperty(DocumentFragment.prototype,'nodeName',{get(){return'#document-fragment'},enumerable:true,configurable:true});`
	return strings.Replace(handwrittenSurface, marker, capabilitySurface+"\n"+generated+"\nfinalizeBindings();\n"+exposureSource+"installNavigatorCapabilities();\n"+semanticFixups+"\n"+marker, 1)
}

func WorkerSurface(generated string, exposure *compatibility.RealmExposure) string {
	exposureSource := ""
	if exposure != nil {
		encoded, err := json.Marshal(exposure)
		if err != nil {
			panic(err)
		}
		exposureSource = "const __workerExposure=" + string(encoded) + ";__applyWorkerExposure(__workerExposure);\n"
	}
	return handwrittenWorkerSurface + "\n" + generated + "\n" + exposureSource + "__finishWorkerSurface();if(typeof __workerExposure!=='undefined')__applyWorkerPrototypeExposure(__workerExposure);delete globalThis.__applyWorkerExposure;delete globalThis.__applyWorkerPrototypeExposure;delete globalThis.__finishWorkerSurface;delete globalThis.__mimic;delete globalThis.__mimicIDLExposure;"
}
