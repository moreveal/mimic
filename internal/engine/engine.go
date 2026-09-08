package engine

import (
	"context"
	"time"
)

// Value is deliberately engine-neutral. Browser semantics must not depend on
// the representation used by the selected ECMAScript implementation.
type Value interface {
	Export() any
	String() string
}

type Function func(this Value, args []Value) (Value, error)

type Promise struct {
	Value   Value
	Resolve func(any) error
	Reject  func(any) error
}

type Runtime interface {
	Eval(context.Context, string, string) (Value, error)
	Set(string, any) error
	Get(string) Value
	Value(any) Value
	GetProperty(Value, string) Value
	SetProperty(Value, string, any) error
	TypeOf(Value) string
	StrictEqual(Value, Value) bool
	Call(context.Context, Value, Value, ...Value) (Value, error)
	Function(Function) any
	NewPromise() Promise
	Await(Value) (Value, bool, error)
	SetTimeSource(func() time.Time)
	MicrotaskCheckpoint() error
	SetGlobalAccessObserver(func(name string, supported bool))
	Close() error
}

// ModuleLoader resolves one static module request. referrer is the canonical
// resource name supplied for the importing module; resourceName becomes the
// identity and base URL of the returned source.
type ModuleLoader func(specifier, referrer string) (source, resourceName string, err error)

// ModuleRuntime is implemented by engines which can compile, link, and
// evaluate ECMAScript SourceTextModule graphs.
type ModuleRuntime interface {
	EvalModule(context.Context, string, string, ModuleLoader) (Value, error)
}

type Factory interface{ New() Runtime }
