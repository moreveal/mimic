package browser

import "github.com/moreveal/mimic/internal/engine"

func transientRuntimeFunction(runtime engine.Runtime, function engine.Function) any {
	if owner, ok := runtime.(interface{ TransientFunction(engine.Function) any }); ok {
		return owner.TransientFunction(function)
	}
	return runtime.Function(function)
}

func retainRuntimeValue(runtime engine.Runtime, value engine.Value) engine.Value {
	if owner, ok := runtime.(engine.ValueRetainer); ok {
		return owner.RetainValue(value)
	}
	return value
}

func releaseRuntimeValues(runtime engine.Runtime, values ...engine.Value) {
	if owner, ok := runtime.(engine.ValueReleaser); ok {
		for _, value := range values {
			owner.ReleaseValue(value)
		}
	}
}
