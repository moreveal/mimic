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

//go:embed base64.js
var base64Surface string

//go:embed native_functions.js
var nativeFunctionsSurface string

//go:embed webkit_css_names.js
var webkitCSSNamesSurface string

//go:embed webkit_css.js
var webkitCSSSurface string

//go:embed css_supports.js
var cssSupportsSurface string

//go:embed css_shorthands.js
var cssShorthandsSurface string

//go:embed css_value_grammar.js
var cssValueGrammarSurface string

//go:embed css_font_metrics.js
var cssFontMetricsSurface string

//go:embed css_animation_grammar.js
var cssAnimationGrammarSurface string

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

//go:embed intersection_observer.js
var intersectionObserverSurface string

//go:embed traversal_compatibility.js
var traversalCompatibilitySurface string

//go:embed document_compatibility.js
var documentCompatibilitySurface string

//go:embed document_all.js
var documentAllSurface string

//go:embed attributes_compatibility.js
var attributesCompatibilitySurface string

//go:embed svg_geometry.js
var svgGeometrySurface string

//go:embed svg_text.js
var svgTextSurface string

//go:embed svg_boundaries.js
var svgBoundariesSurface string

//go:embed svg_types.js
var svgTypesSurface string

//go:embed svg_coordinates.js
var svgCoordinatesSurface string

//go:embed svg_path_metrics.js
var svgPathMetricsSurface string

//go:embed svg_use.js
var svgUseSurface string

//go:embed svg_attribute_defaults.js
var svgAttributeDefaultsSurface string

//go:embed svg_attribute_semantics.js
var svgAttributeSemanticsSurface string

//go:embed svg_reflections.js
var svgReflectionsSurface string

//go:embed svg_css_transform.js
var svgCSSTransformSurface string

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

//go:embed offline_audio.js
var offlineAudioSurface string

//go:embed font_faces.js
var fontFacesSurface string

//go:embed webgpu_state.js
var webgpuStateSurface string

//go:embed webgl_state.js
var webglStateSurface string

//go:embed document_state.js
var documentStateSurface string

//go:embed image_resources.js
var imageResourcesSurface string

//go:embed screen_focus.js
var screenFocusSurface string

//go:embed css_colors.js
var cssColorsSurface string

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
		"installNavigatorCapabilities();", cssSupportsSurface, semanticFixups, strings.Replace(domCompatibilitySurface, "/* shared_abort_encoding */", abortEncodingSurface, 1),
		templatesCompatibilitySurface, selectorsVendorSurface, selectorsCompatibilitySurface, cssomCompatibilitySurface, strings.Replace(strings.Replace(strings.Replace(strings.Replace(svgGeometrySurface, "/* shared_svg_boundaries */", svgBoundariesSurface, 1), "/* shared_svg_text */", svgTextSurface, 1), "/* shared_svg_css_transform */", svgCSSTransformSurface, 1), "/* shared_svg_types */", svgTypesSurface+svgCoordinatesSurface+svgReflectionsSurface+svgPathMetricsSurface+svgUseSurface+svgAttributeDefaultsSurface+svgAttributeSemanticsSurface, 1), streamPrelude,
		streamsVendorSurface, "}", fetchCompatibilitySurface, formControlsSurface, traversalCompatibilitySurface, strings.Replace(documentCompatibilitySurface, "/* shared_document_state */", documentStateSurface, 1), documentAllSurface, attributesCompatibilitySurface, imageResourcesSurface, screenFocusSurface, eventsCompatibilitySurface, documentStreamSurface, shadowSerializationSurface, strings.Replace(canvasStateSurface, "/* shared_canvas_paths */", canvasPathObservationsSurface, 1), webglObservationsSource(), webgpuStateSurface, fontFacesSurface, offlineAudioSurface, marker}
	base := strings.Replace(handwrittenSurface, "/* shared_fetch_primitives */", fetchPrimitivesSurface, 1)
	base = strings.Replace(base, "/* shared_native_functions */", nativeFunctionsSurface, 1)
	base = strings.Replace(base, "/* shared_base64 */", base64Surface, 1)
	base = strings.Replace(base, "/* shared_webkit_css */", webkitCSSNamesSurface+webkitCSSSurface+cssShorthandsSurface+cssValueGrammarSurface+cssAnimationGrammarSurface+cssFontMetricsSurface, 1)
	base = strings.Replace(base, "/* shared_dom_matrix */", cssColorsSurface+domMatrixSurface, 1)
	base = strings.Replace(base, "/* shared_intersection_observer */", intersectionObserverSurface, 1)
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
	shared := fetchPrimitivesSurface + "\ninstallFileReader();Object.assign(globalThis,{TextEncoder,DOMException,URL,URLSearchParams,Blob,File,FormData,FileReader,Headers});\n" + abortEncodingSurface + "\n{let structuredClone;\n" + streamsVendorSurface + "\n}\n" + fetchCompatibilitySurface
	base := strings.Replace(handwrittenWorkerSurface, "/* shared_worker_fetch */", shared, 1)
	return base + "\n" + generated + "\n" + exposureSource + "__finishWorkerSurface();if(typeof __workerExposure!=='undefined')__applyWorkerPrototypeExposure(__workerExposure);delete globalThis.__applyWorkerExposure;delete globalThis.__applyWorkerPrototypeExposure;delete globalThis.__finishWorkerSurface;delete globalThis.__mimic;delete globalThis.__mimicIDLExposure;\n{const host=__workerHost;\n" + nativeFunctionsSurface + base64Surface + cssColorsSurface + "\nfor(const [name,value] of Object.entries({atob,btoa}))Object.defineProperty(WorkerGlobalScope.prototype,name,{value,writable:true,enumerable:true,configurable:true});\n" + domMatrixSurface + "\n" + strings.Replace(canvasStateSurface, "/* shared_canvas_paths */", canvasPathObservationsSurface, 1) + "\n" + webglObservationsSource() + "\n" + webgpuStateSurface + "\n" + fontFacesSurface + "\n}"
}
