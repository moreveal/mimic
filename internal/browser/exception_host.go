package browser

import (
	"github.com/moreveal/mimic/internal/engine"
)

func installExceptionDescription(host map[string]any, runtime engine.Runtime) {
	host["describeException"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		var value engine.Value
		if len(args) > 0 {
			value = args[0]
		}
		if inspector, ok := runtime.(engine.ExceptionInspector); ok {
			if details, ok := inspector.DescribeException(value); ok {
				return runtime.Value(map[string]any{"message": details.Message, "filename": details.Filename, "lineno": details.Line, "colno": details.Column}), nil
			}
		}
		// Non-native engines explicitly lack source position metadata. Do not
		// access a user-controlled .stack getter to manufacture a location.
		return runtime.Value(nil), nil
	})
}
