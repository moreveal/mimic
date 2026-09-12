package browser

import (
	"context"
	"github.com/moreveal/mimic/internal/engine"
)

func installNativeFunctionSource(runtime engine.Runtime) error {
	source := runtime.Get("__mimicNativeFunctionSources")
	if native, ok := runtime.(engine.NativeFunctionSourceRuntime); ok {
		if err := native.InstallNativeFunctionToString(runtime.GetProperty(source, "0"), runtime.GetProperty(source, "1")); err != nil {
			return err
		}
	}
	_, err := runtime.Eval(context.Background(), "delete globalThis.__mimicNativeFunctionSources", "mimic:hide-native-function-source")
	return err
}
