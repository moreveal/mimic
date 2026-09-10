package engine

// StructuredCloneRuntime copies ECMAScript values for browser-owned storage.
// Browser platform objects require separate serialization hooks.
type StructuredCloneRuntime interface {
	StructuredClone(value Value, rejectHostObject Value) (Value, error)
}

// StructuredCloneProxyRuntime lets a fallback reject proxies without executing traps.
type StructuredCloneProxyRuntime interface{ IsStructuredCloneProxy(Value) bool }

type DataCloneError struct{ Message string }

func (e *DataCloneError) Error() string { return e.Message }
