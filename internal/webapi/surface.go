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

//go:embed dom_compatibility.js
var domCompatibilitySurface string

//go:embed templates_compatibility.js
var templatesCompatibilitySurface string

//go:embed shadow_serialization.js
var shadowSerializationSurface string

//go:embed cssom_compatibility.js
var cssomCompatibilitySurface string

//go:embed traversal_compatibility.js
var traversalCompatibilitySurface string

//go:embed document_compatibility.js
var documentCompatibilitySurface string

//go:embed attributes_compatibility.js
var attributesCompatibilitySurface string

//go:embed events_compatibility.js
var eventsCompatibilitySurface string

//go:embed document_stream.js
var documentStreamSurface string

//go:embed selectors_vendor.js
var selectorsVendorSurface string

//go:embed selectors_compatibility.js
var selectorsCompatibilitySurface string

//go:embed streams_vendor.js
var streamsVendorSurface string

//go:embed fetch_compatibility.js
var fetchCompatibilitySurface string

//go:embed fetch_primitives.js
var fetchPrimitivesSurface string

//go:embed abort_encoding.js
var abortEncodingSurface string

//go:embed canvas_path_observations.js
var canvasPathObservationsSurface string

//go:embed canvas_state.js
var canvasStateSurface string

//go:embed glsl_observations.js
var glslObservationsSurface string

//go:embed webgl_framebuffer_observations.js
var webglFramebufferObservationsSurface string

//go:embed webgl_program_observations.js
var webglProgramObservationsSurface string

func webglObservationsSource() string {
	return strings.NewReplacer("/* shared_glsl_observations */", glslObservationsSurface, "/* shared_webgl_programs */", webglProgramObservationsSurface, "/* shared_webgl_framebuffers */", webglFramebufferObservationsSurface).Replace(webglStateSurface)
}

//go:embed webgpu_state.js
var webgpuStateSurface string

//go:embed webgl_state.js
var webglStateSurface string

//go:embed dom_matrix.js
var domMatrixSurface string

//go:embed form_controls.js
var formControlsSurface string

type surfaceKey struct {
	surface  *compatibility.WebAPISurface
	exposure string
}

type surfaceOutput struct{ Source, ExposureJSON, CatalogJSON string }

var surfaceSources = struct {
	sync.Mutex
	values map[surfaceKey]func() surfaceOutput
}{values: map[surfaceKey]func() surfaceOutput{}}

// SurfaceFor shares only immutable source text from a compatibility bundle.
// The bounded cache owns no Page, isolate, or mutable JavaScript object.
func SurfaceFor(surface *compatibility.WebAPISurface, name string) (string, string) {
	output := bootstrapFor(surface, name)
	return output.Source, output.ExposureJSON
}

// BootstrapFor shares only immutable, selected-profile data across Pages.
func BootstrapFor(surface *compatibility.WebAPISurface, name string) (string, string, string) {
	output := bootstrapFor(surface, name)
	return output.Source, output.ExposureJSON, output.CatalogJSON
}

func bootstrapFor(surface *compatibility.WebAPISurface, name string) surfaceOutput {
	key := surfaceKey{surface, name}
	surfaceSources.Lock()
	build := surfaceSources.values[key]
	if build == nil {
		build = sync.OnceValue(func() surfaceOutput {
			exposure, ok := surface.Exposures[name]
			if !ok {
				return surfaceOutput{Source: Surface(surface.GeneratedJavaScript, nil), CatalogJSON: surface.GeneratedCatalogJSON}
			}
			encoded, err := json.Marshal(exposure)
			if err != nil {
				panic(err)
			}
			return surfaceOutput{Source: composeSurface(surface.GeneratedJavaScript, "applyTargetExposure(JSON.parse(host.exposureJSON()));\n"), ExposureJSON: string(encoded), CatalogJSON: selectedCatalog(surface.GeneratedCatalogJSON, exposure)}
		})
		if len(surfaceSources.values) < 8 {
			surfaceSources.values[key] = build
		}
	}
	surfaceSources.Unlock()
	return build()
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
	// Generated shape alone does not establish transferable structured cloning.
	// Keep the upstream stream bundle unchanged, and select only a functioning
	// transfer primitive for its fallback path (engines without one copy bytes).
	streamPrelude := `{let structuredClone;try{const input=new ArrayBuffer(1),clone=globalThis.structuredClone;if(typeof clone==='function'){const output=clone(input,{transfer:[input]});if(output instanceof ArrayBuffer&&output.byteLength===1&&input.byteLength===0)structuredClone=clone}}catch{}`
	parts := []string{capabilitySurface, generated, "finalizeBindings();", exposureSource,
		"installNavigatorCapabilities();", semanticFixups, strings.Replace(domCompatibilitySurface, "/* shared_abort_encoding */", abortEncodingSurface, 1),
		templatesCompatibilitySurface, selectorsVendorSurface, selectorsCompatibilitySurface, cssomCompatibilitySurface, streamPrelude,
		streamsVendorSurface, "}", fetchCompatibilitySurface, formControlsSurface, traversalCompatibilitySurface, documentCompatibilitySurface, attributesCompatibilitySurface, eventsCompatibilitySurface, documentStreamSurface, shadowSerializationSurface, strings.Replace(canvasStateSurface, "/* shared_canvas_paths */", canvasPathObservationsSurface, 1), webglObservationsSource(), webgpuStateSurface, marker}
	base := strings.Replace(handwrittenSurface, "/* shared_fetch_primitives */", fetchPrimitivesSurface, 1)
	base = strings.Replace(base, "/* shared_dom_matrix */", domMatrixSurface, 1)
	return strings.Replace(base, marker, strings.Join(parts, "\n"), 1)
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
	shared := fetchPrimitivesSurface + "\nObject.assign(globalThis,{TextEncoder,DOMException,URL,URLSearchParams,Blob,File,Headers});\n" + abortEncodingSurface + "\n{let structuredClone;\n" + streamsVendorSurface + "\n}\n" + fetchCompatibilitySurface
	base := strings.Replace(handwrittenWorkerSurface, "/* shared_worker_fetch */", shared, 1)
	return base + "\n" + generated + "\n" + exposureSource + "__finishWorkerSurface();if(typeof __workerExposure!=='undefined')__applyWorkerPrototypeExposure(__workerExposure);delete globalThis.__applyWorkerExposure;delete globalThis.__applyWorkerPrototypeExposure;delete globalThis.__finishWorkerSurface;delete globalThis.__mimic;delete globalThis.__mimicIDLExposure;\n{const host=__workerHost;\n" + domMatrixSurface + "\n" + strings.Replace(canvasStateSurface, "/* shared_canvas_paths */", canvasPathObservationsSurface, 1) + "\n" + webglObservationsSource() + "\n" + webgpuStateSurface + "\n}"
}
