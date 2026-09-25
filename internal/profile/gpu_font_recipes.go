package profile

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moreveal/mimic/internal/state"
)

// This catalog contains only jointly observed fields supported by Mimic. The
// source collection itself is deliberately not embedded: pixel hashes, raw font
// inventories, browser identity, and network details are not profile inputs.
//
//go:embed gpu_font_recipes.json
var gpuFontRecipesJSON []byte

type webGLRecipe struct {
	Parameters map[string]json.RawMessage `json:"parameters"`
	Precision  map[string]json.RawMessage `json:"precision"`
	Extensions []string                   `json:"extensions"`
	Anisotropy *int                       `json:"anisotropy"`
}

type gpuFontRecipe struct {
	ID                string                 `json:"ID"`
	SourceSHA256      string                 `json:"SourceSHA256"`
	SourceChromeMajor int                    `json:"SourceChromeMajor"`
	Graphics          state.Graphics         `json:"Graphics"`
	Fonts             state.Fonts            `json:"Fonts"`
	WebGL             map[string]webGLRecipe `json:"WebGL"`
}

var gpuFontRecipes = loadGPUFontRecipes()

func loadGPUFontRecipes() []gpuFontRecipe {
	var recipes []gpuFontRecipe
	if err := json.Unmarshal(gpuFontRecipesJSON, &recipes); err != nil {
		panic(fmt.Errorf("invalid embedded GPU/font recipes: %w", err))
	}
	for i := range recipes {
		r := &recipes[i]
		if r.ID == "" || r.SourceChromeMajor < 140 || len(r.SourceSHA256) != 64 || !strings.Contains(r.Graphics.Renderer, "Direct3D11") {
			panic("invalid embedded GPU/font recipe provenance or backend")
		}
		if !strings.Contains(strings.ToLower(r.Graphics.Vendor), r.Graphics.WebGPU.Vendor) || r.Graphics.WebGPU.Limits["maxTextureDimension2D"] != uint64(r.Graphics.MaxTextureSize) {
			panic("incoherent embedded WebGL/WebGPU identity or limits")
		}
		var capabilities map[string]map[string]any
		if err := json.Unmarshal([]byte((state.Graphics{}).WebGLCapabilities()), &capabilities); err != nil {
			panic(err)
		}
		for kind, overrides := range r.WebGL {
			entry, ok := capabilities[kind]
			if !ok {
				panic("unknown WebGL recipe kind")
			}
			parameters := entry["parameters"].(map[string]any)
			for name, raw := range overrides.Parameters {
				if _, known := parameters[name]; !known {
					panic("unknown WebGL recipe parameter")
				}
				var value any
				if err := json.Unmarshal(raw, &value); err != nil {
					panic(err)
				}
				parameters[name] = value
			}
			precision := entry["precision"].(map[string]any)
			for name, raw := range overrides.Precision {
				if _, known := precision[name]; !known {
					panic("unknown WebGL recipe precision")
				}
				var value any
				if err := json.Unmarshal(raw, &value); err != nil {
					panic(err)
				}
				precision[name] = value
			}
			entry["extensions"] = overrides.Extensions
			if overrides.Anisotropy != nil {
				entry["anisotropy"] = *overrides.Anisotropy
			}
		}
		for _, kind := range []string{"webgl", "webgl2"} {
			parameters := capabilities[kind]["parameters"].(map[string]any)
			maxRenderbuffer := parameters["34024"].(map[string]any)["value"].(float64)
			if int(maxRenderbuffer) != r.Graphics.MaxTextureSize || len(r.WebGL[kind].Extensions) == 0 {
				panic("incoherent embedded WebGL texture limit or extensions")
			}
		}
		encoded, err := json.Marshal(capabilities)
		if err != nil {
			panic(err)
		}
		r.Graphics.WebGLCapabilitiesJSON = string(encoded)
		r.WebGL = nil // Keep only the resolved immutable observation string.
	}
	return recipes
}

func (r gpuFontRecipe) apply(d *Document) {
	// WGSL language support follows the installed Chrome build; the source
	// adapter records do not establish another value. Discovery timing also
	// lacks a linked observation in this collection.
	wgsl := d.Graphics.WebGPU.WGSLLanguageFeatures
	initializationDelay := d.Graphics.WebGPU.InitializationDelayMillis
	d.Graphics = r.Graphics
	d.Graphics.WebGPU.WGSLLanguageFeatures = append([]string(nil), wgsl...)
	d.Graphics.WebGPU.InitializationDelayMillis = initializationDelay
	d.Graphics.WebGPU.Features = append([]string(nil), r.Graphics.WebGPU.Features...)
	d.Graphics.WebGPU.Limits = make(map[string]uint64, len(r.Graphics.WebGPU.Limits))
	for name, value := range r.Graphics.WebGPU.Limits {
		d.Graphics.WebGPU.Limits[name] = value
	}
	d.Fonts = r.Fonts
	d.Fonts.System = make(map[string]string, len(r.Fonts.System))
	for name, value := range r.Fonts.System {
		d.Fonts.System[name] = value
	}
}
