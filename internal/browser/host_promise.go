package browser

import "github.com/moreveal/mimic/internal/engine"

// Only use while returning the Promise from a synchronous host callback.
// Async work retains the resolver closures, never the borrowed Value.
func newHostPromise(runtime engine.Runtime) engine.Promise {
	if owner, ok := runtime.(engine.HostPromiseRuntime); ok {
		return owner.NewHostPromise()
	}
	return runtime.NewPromise()
}
