package browser

import (
	"encoding/json"

	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/textmetrics"
	"github.com/moreveal/mimic/internal/trace"
)

func (r *Realm) installTextMetrics(host map[string]any) {
	host["shapeText"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		p := r.agent.Page()
		if p.textMetrics == nil {
			p.textMetrics = textmetrics.New()
		}
		result, err := p.textMetrics.Shape(strarg(args, 0), strarg(args, 1), numarg(args, 2), numarg(args, 3), numarg(args, 4) != 0, numarg(args, 5) != 0, numarg(args, 6) != 0)
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
