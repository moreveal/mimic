package webapi

import (
	_ "embed"
	"encoding/json"
	"strings"

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

func Surface(generated string, exposure *compatibility.RealmExposure) string {
	// Tracking starts only after both handwritten semantics and the selected
	// bundle's generated surface have been installed. Otherwise feature
	// detection performed by the generator pollutes runtime API traces.
	marker := "known=new Set(Reflect.ownKeys(globalThis));host.ready();"
	exposureSource := ""
	if exposure != nil {
		encoded, err := json.Marshal(exposure)
		if err != nil {
			panic(err)
		}
		exposureSource = "applyTargetExposure(" + string(encoded) + ");\n"
	}
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
