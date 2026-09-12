package browser

import (
	"encoding/json"

	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/state"
	"github.com/moreveal/mimic/internal/textmetrics"
	"github.com/moreveal/mimic/internal/trace"
)

func newTextMetricsEngine(fonts state.Fonts) *textmetrics.Engine {
	result := textmetrics.New()
	for generic, family := range map[string]string{"serif": fonts.Serif, "sans-serif": fonts.SansSerif, "monospace": fonts.Monospace, "system-ui": fonts.SystemUI} {
		result.SetGenericFamily(generic, family)
	}
	return result
}
func (p *Page) textMetricsEngine() *textmetrics.Engine {
	if p.textMetrics == nil {
		p.textMetrics = newTextMetricsEngine(p.Environment().Fonts)
	}
	return p.textMetrics
}

func (r *Realm) installTextMetrics(host map[string]any) {
	installFontResourceHosts(host, r.runtime, func() *textmetrics.Engine {
		p := r.agent.Page()
		return p.textMetricsEngine()
	})
	r.installFontCollection(host)

	host["shapeText"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		p := r.agent.Page()
		shape := p.textMetricsEngine().ShapeWithFonts
		if numarg(args, 7) != 0 {
			shape = p.textMetricsEngine().ShapeCanvasWithFonts
		}
		result, err := shape(strarg(args, 0), strarg(args, 1), numarg(args, 2), numarg(args, 3), numarg(args, 4) != 0, numarg(args, 5) != 0, numarg(args, 6) != 0, r.fontChoices)
		if err != nil {
			// Private trace only: author-facing exceptions must not expose local paths.
			bounded := func(value string, limit int) string {
				runes := []rune(value)
				if len(runes) > limit {
					return string(runes[:limit])
				}
				return value
			}
			p.trace.Add(trace.SemanticMissing, "Text.shapeFailure", map[string]any{
				"realm": r.ID, "reason": bounded(err.Error(), 512), "reasonAvailable": true, "category": "unsupported-boundary", "site": "text_metrics.go/installTextMetrics", "operation": "shapeText",
				"text": bounded(strarg(args, 0), 160), "textRunes": len([]rune(strarg(args, 0))),
				"families": bounded(strarg(args, 1), 256), "size": numarg(args, 2), "weight": numarg(args, 3),
				"italic": numarg(args, 4) != 0, "noKern": numarg(args, 5) != 0, "noLigatures": numarg(args, 6) != 0,
			})
			return r.val(`{"error":"Unsupported local font resource or shaping operation"}`), nil
		}
		data, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		return r.val(string(data)), nil
	})
}
