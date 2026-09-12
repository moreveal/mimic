package browser

import "github.com/moreveal/mimic/internal/engine"

func installConsoleKind(host map[string]any, runtime engine.Runtime) {
	if classifier, ok := runtime.(engine.ConsoleValueRuntime); ok {
		host["consoleValueKind"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
			if len(args) == 0 {
				return runtime.Value(""), nil
			}
			kind, err := classifier.ConsoleValueKind(args[0])
			if err != nil {
				return nil, err
			}
			return runtime.Value(kind), nil
		})
	}
}
