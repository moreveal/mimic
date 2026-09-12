package browser

import (
	"encoding/base64"
	"errors"

	"github.com/moreveal/mimic/internal/engine"
)

// Strings keep native wire bytes opaque to the generic host-value transport.
// In particular, no JS object graph is exported through Go for message delivery.
func installStructuredCloneHost(host map[string]any, runtime engine.Runtime) {
	if detector, ok := runtime.(engine.StructuredCloneProxyRuntime); ok {
		host["cloneIsProxy"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
			return runtime.Value(detector.IsStructuredCloneProxy(args[0])), nil
		})
	}
	codec, ok := runtime.(engine.StructuredCloneCodec)
	if !ok {
		return
	}
	host["serializeClone"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		bytes, err := codec.SerializeStructuredClone(args[0], args[1])
		var cloneError *engine.DataCloneError
		if errors.As(err, &cloneError) {
			return runtime.Value([]any{false, cloneError.Message}), nil
		}
		if err != nil {
			return nil, err
		}
		return runtime.Value([]any{true, base64.StdEncoding.EncodeToString(bytes)}), nil
	})
	host["deserializeClone"] = runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		bytes, err := base64.StdEncoding.DecodeString(args[0].String())
		if err != nil {
			return nil, err
		}
		return codec.DeserializeStructuredClone(bytes)
	})
}
