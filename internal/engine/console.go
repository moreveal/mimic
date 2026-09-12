package engine

// ConsoleValueRuntime identifies the native values whose console message text
// uses ToString. Classification must not invoke author properties or proxy traps.
// An empty kind denotes an ordinary object or proxy, which is not coerced.
type ConsoleValueRuntime interface {
	ConsoleValueKind(Value) (string, error)
}
