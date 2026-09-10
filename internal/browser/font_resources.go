package browser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/textmetrics"
)

func installFontResourceHosts(host map[string]any, runtime engine.Runtime, resources func() *textmetrics.Engine) {
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
		r.fontChoices = choices
		return nil, nil
	})
}
