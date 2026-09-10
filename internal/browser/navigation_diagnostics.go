package browser

import (
	"os"

	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/trace"
)

// This opt-in trace can contain application source and credentials. It uses
// native engine frames instead of evaluating Error.stack in the application.
func (r *Realm) recordNavigationDiagnostic(raw string, options []bool) {
	if os.Getenv("MIMIC_NAVIGATION_DIAGNOSTICS") != "1" {
		return
	}
	data := map[string]any{"realm": r.ID, "url": raw, "replaceReload": options}
	if capture, ok := r.runtime.(engine.NativeStackCapture); ok {
		data["stack"] = capture.CaptureNativeStack(16, true)
	}
	r.agent.Page().trace.Add(trace.JS, "navigationIntent", data)
}
