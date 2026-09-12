package browser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/textmetrics"
)

func installFontResourceHosts(host map[string]any, runtime engine.Runtime, resources func() *textmetrics.Engine) {
	host["shapeText"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		shape := resources().ShapeWithFonts
		if numarg(args, 7) != 0 {
			shape = resources().ShapeCanvasWithFonts
		}
		result, err := shape(strarg(args, 0), strarg(args, 1), numarg(args, 2), numarg(args, 3), numarg(args, 4) != 0, numarg(args, 5) != 0, numarg(args, 6) != 0, nil)
		if err != nil {
			return runtime.Value(`{"error":"Unsupported local font resource or shaping operation"}`), nil
		}
		data, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		return runtime.Value(string(data)), nil
	})
	host["fontLocal"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		id, err := resources().LocalFont(strarg(args, 0))
		reason := ""
		if err != nil {
			reason = err.Error()
		}
		return runtime.Value(map[string]any{"id": id, "error": reason}), nil
	})
	host["fontBinary"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		encoded := strarg(args, 0)
		if len(encoded) > 44<<20 {
			return nil, fmt.Errorf("font byte limit")
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		id := ""
		if err == nil {
			id, err = resources().RegisterFont(data)
		}
		reason := ""
		if err != nil {
			reason = "Invalid or unsupported font resource"
		}
		return runtime.Value(map[string]any{"id": id, "error": reason}), nil
	})
}

func (r *Realm) installFontCollection(host map[string]any) {
	host["fontCollection"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		var choices []textmetrics.FontReference
		if err := json.Unmarshal([]byte(strarg(args, 0)), &choices); err != nil {
			return nil, err
		}
		if len(choices) > 1024 {
			return nil, fmt.Errorf("font collection limit")
		}
		if !slices.Equal(r.fontChoices, choices) {
			if r.textCacheProfile != nil {
				r.textCacheProfile.FontInvalidations++
			}
			r.fontChoices = choices
			r.textShapeCache = nil
			// Geometry may be reused across reads within this task. FontFaceSet
			// mutations affect those metrics without changing a DOM attribute.
			// The shared document epoch also invalidates isolated-world views,
			// while font selection retains its existing realm ownership.
			r.document.InvalidateObservations()
		}
		return nil, nil
	})
}
