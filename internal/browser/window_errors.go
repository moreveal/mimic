package browser

import (
	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/trace"
)

func (r *Realm) installWindowExceptionReporting(host map[string]any) {
	installExceptionDescription(host, r.runtime)
	host["installWindowErrorReporter"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		r.windowErrorReporter = args[0]
		return nil, nil
	})
	host["reportUnhandledException"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		r.agent.Page().trace.Add(trace.Exception, "uncaught", map[string]any{
			"error": strarg(args, 0), "url": strarg(args, 1), "realm": r.ID,
			"lineNumber":   max(int(numarg(args, 2))-1, 0),
			"columnNumber": max(int(numarg(args, 3))-1, 0),
			"stackFrames":  arg(args, 4),
			"valueType":    strarg(args, 5),
			"errorObject":  len(args) > 6 && args[6] != nil && args[6].Export() == true,
			"errorStack":   strarg(args, 7),
		})
		return nil, nil
	})
}
