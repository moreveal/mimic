package browser

import (
	"encoding/json"
	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/trace"
	"strings"
)

// Boundary names identify implementation branches, not inferred server causes.
// Do not inspect author objects or capture JS stacks: getters and stack hooks
// would introduce observable side effects into a diagnostic-only path.
func recordSemanticBoundary(recorder *trace.Recorder, ownerKind string, owner any, site string, args []engine.Value) {
	name := strarg(args, 0)
	category := "unsupported-boundary"
	if strings.Contains(strings.ToLower(name), "approximate") {
		category = "approximation"
	}
	data := map[string]any{ownerKind: owner, "site": site, "operation": name, "category": category, "reasonAvailable": false}
	if len(args) > 1 {
		detail := []rune(strarg(args, 1))
		if len(detail) > 2048 {
			detail = detail[:2048]
			data["detailTruncated"] = true
		}
		data["detail"] = string(detail)
		var context map[string]any
		if json.Unmarshal([]byte(string(detail)), &context) == nil && context != nil {
			data["context"] = context
			if reason, ok := context["reason"].(string); ok && reason != "" {
				data["reason"] = reason
				data["reasonAvailable"] = true
			}
		}
	}
	recorder.Add(trace.SemanticMissing, name, data)
}
