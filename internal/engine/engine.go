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

// ValueReleaser releases an embedder-owned root after its last use. It never
// destroys the JavaScript object while that object is reachable from script.
// Values retained by another Go owner must not be released through this API.
type ValueReleaser interface {
	ReleaseValue(Value)
}

// ThrownValue preserves the identity and type of a JavaScript exception.
// The caller may release this root after handling the exception.
type ThrownValue interface {
	ThrownValue() Value
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

// ReentrantRuntime permits a synchronous call into another actor-owned realm
// while servicing calls back into this realm on its original actor thread.
// The operation must use the supplied context and return before RunNested
// returns; it must not dispose the calling runtime from inside the operation.
type ReentrantRuntime interface {
	RunNested(context.Context, func(context.Context) error) error
}

// OwnerRuntime groups synchronous engine operations on their owning thread.
// It does not pump tasks or perform a microtask checkpoint. The operation must
// not close the runtime or retain thread-local state after returning.
type OwnerRuntime interface {
	RunOnOwner(context.Context, func(context.Context) error) error
}

// BootstrapRuntime may reuse a compiled embedder function body across runtimes.
// Only immutable code is shared; execution and all objects remain realm-local.
// The body must install state explicitly on globalThis (no global var bindings).
// Page scripts must use Eval, even if they choose an internal-looking filename.
type BootstrapRuntime interface {
	EvalBootstrap(context.Context, string, string) (Value, error)
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

// EvalSourceRuntime lets the browser recognize branded code objects without
// replacing native eval (which would destroy direct eval's lexical scope).
// The realm-owned resolver returns a source string for a recognized object,
// or undefined to preserve native eval's non-string identity behavior.
type EvalSourceRuntime interface {
	SetEvalSourceResolver(Value) error
}

type Factory interface{ New() Runtime }

// UndetectableRuntime creates a native callable object with HTMLDDA operator
// semantics. Property operations are delegated to realm-owned JS handlers;
// an ordinary JS Proxy cannot preserve the native undetectable flag.
type UndetectableRuntime interface {
	NewUndetectableObject(handlers Value) (Value, error)
}

// InterceptedObjectRuntime supplies detectable, noncallable exotic objects.
// Unlike JS Proxy targets, their descriptors can change when a Window navigates.
type InterceptedObjectRuntime interface {
	NewInterceptedObject(handlers Value) (Value, error)
}

// ArrayBufferDetacher supplies real backing-store detachment where the engine
// does not expose ArrayBuffer.prototype.transfer. Call only on the realm actor.
type ArrayBufferDetacher interface {
	DetachArrayBuffer(Value) error
}
