//go:build windows && amd64

package gov8

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

var (
	functionNewDirectOnce sync.Once
	functionNewDirectAddr uintptr
)

func ensureFunctionNewDirectProc() {
	functionNewDirectOnce.Do(func() {
		functionNewDirectAddr = proc("gov8_fa_function_new_direct").Addr()
	})
}

// Templates: FunctionTemplate and ObjectTemplate.
//
// Parity mapping against the pinned Rust crate (v8 =152.2.0):
//   - v8::FunctionTemplate::builder(cb)[.length(n)][.data(v)].build(scope)
//     and v8::FunctionTemplate::new(scope, cb) map to
//     Isolate.NewFunctionTemplate(s, cb, opts) with a nil opts for the
//     defaults (Go has no default arguments; the option struct is explicit).
//   - v8::Function::builder(cb)... maps to Isolate.NewFunction — the
//     function object is created directly in the context (v8::Function::New).
//   - Templates are scope-local handles exactly as in Rust: they live in the
//     Scope they were created in and every method refuses to run once that
//     scope closes. The engine keeps template-derived objects (functions,
//     instances, accessor properties) alive on its own; a template wrapper
//     carries no engine-persistent state of its own beyond the callback
//     registration released by ReleaseIsolateHostState.
//   - Native callbacks are Go FunctionCallbacks dispatched through the
//     registry (see callback.go); the embedder data is passed verbatim and
//     observed via args.Data().

// PropertyAttribute mirrors v8::PropertyAttribute.
type PropertyAttribute uint8

const (
	AttrNone       PropertyAttribute = 0
	AttrReadOnly   PropertyAttribute = 1
	AttrDontEnum   PropertyAttribute = 2
	AttrDontDelete PropertyAttribute = 4
)

// ConstructorBehavior mirrors v8::ConstructorBehavior. The zero value
// selects the engine default (kAllow), matching the crate's builder default.
type ConstructorBehavior uint8

const (
	// ConstructorBehaviorDefault uses the engine default (kAllow).
	ConstructorBehaviorDefault ConstructorBehavior = iota
	// ConstructorBehaviorThrow maps to kThrow: `new F()` rejects with a
	// TypeError and the function has no .prototype.
	ConstructorBehaviorThrow
	// ConstructorBehaviorAllow maps to kAllow.
	ConstructorBehaviorAllow
)

// FunctionOptions mirrors the FunctionTemplate/Function builder knobs the
// pinned oracle exercises. Zero-value (or nil) selects the engine defaults
// (length 0, no data, no signature, kAllow).
type FunctionOptions struct {
	// Length is the value of the JS function's `length` property. Every int32
	// value is accepted; V8 stores the observable API-function length in its
	// uint16 representation. Values outside int32 are rejected before V8.
	Length int
	// Data is the callback data observed via args.Data(); zero Value means
	// none. It must belong to the same isolate as the template/function.
	Data Value
	// Signature restricts the valid receivers to instances of the
	// signature's function template (or of templates inheriting from it);
	// nil means unrestricted. Template creation only.
	Signature *Signature
	// ConstructorBehavior controls whether the template's function can be
	// constructed (`new`). Zero value = engine default (allow).
	ConstructorBehavior ConstructorBehavior
	// SideEffectType is debugger metadata for throwOnSideEffect evaluation.
	// The zero value is the engine default (HasSideEffect).
	SideEffectType SideEffectType
}

// Signature restricts which receivers a templated function accepts (v8
// Signature::New). It is a scope-local template-side value like the
// templates it is built from.
type Signature struct {
	iso *Isolate
	sc  *Scope
	h   uintptr
}

func (s *Signature) check() error {
	if s.h == 0 {
		return fmt.Errorf("gov8: zero signature handle")
	}
	return s.sc.check()
}

// NewSignature builds a Signature over ft (v8::Signature::New): receivers
// created from ft — or from templates inheriting from it — pass the check.
func (i *Isolate) NewSignature(s *Scope, ft *FunctionTemplate) (*Signature, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if err := ft.check(); err != nil {
		return nil, err
	}
	if ft.iso != i {
		return nil, foreignIsolate("function template")
	}
	ih, err := i.handleChecked()
	if err != nil {
		return nil, err
	}
	h, err := callHandle("Signature.New", proc("gov8_signature_new"),
		ih, s.handle, ft.h)
	if err != nil {
		return nil, err
	}
	return &Signature{iso: i, sc: s, h: h}, nil
}

// FunctionTemplate is a scope-local template for creating JS functions (and,
// through new, constructor-backed objects).
type FunctionTemplate struct {
	iso *Isolate
	sc  *Scope
	h   uintptr
}

func (t *FunctionTemplate) check() error {
	if t.h == 0 {
		return fmt.Errorf("gov8: zero template handle")
	}
	return t.sc.check()
}

// NewFunctionTemplate creates a function template whose native callback is
// cb. opts may be nil. The template lives in the given scope.
func (i *Isolate) NewFunctionTemplate(s *Scope, cb FunctionCallback, opts *FunctionOptions) (*FunctionTemplate, error) {
	if cb == nil {
		return nil, fmt.Errorf("gov8: nil callback")
	}
	if err := validFunctionOptions(opts); err != nil {
		return nil, err
	}
	var data Value
	var sig *Signature
	behavior := ConstructorBehaviorDefault
	sideEffectType := SideEffectHasSideEffect
	if opts != nil {
		data = opts.Data
		sig = opts.Signature
		behavior = opts.ConstructorBehavior
		sideEffectType = opts.SideEffectType
	}
	handle, err := registerFunctionCallback(i, cb, data)
	if err != nil {
		return nil, err
	}
	if err := s.check(); err != nil {
		dropHostCallback(handle)
		return nil, err
	}
	ih, err := i.handleChecked()
	if err != nil {
		dropHostCallback(handle)
		return nil, err
	}
	if sig != nil {
		if err := sig.check(); err != nil {
			dropHostCallback(handle)
			return nil, err
		}
		if sig.iso != i {
			dropHostCallback(handle)
			return nil, foreignIsolate("signature")
		}
	}
	length := 0
	if opts != nil {
		length = opts.Length
	}
	// throw_behavior != 0 selects ConstructorBehavior::kThrow; the Go
	// Default and Allow values both map to the engine default kAllow.
	throwBehavior := int32(0)
	if behavior == ConstructorBehaviorThrow {
		throwBehavior = 1
	}
	entry := lookupHostCallback(handle)
	if entry == nil {
		return nil, fmt.Errorf("gov8: callback registration lost")
	}
	h, err := callHandle("FunctionTemplate.New",
		proc("gov8_fa_function_template_new"), ih, s.handle, entry.ctx,
		uintptr(int32(length)), sigHandle(sig), uintptr(throwBehavior),
		uintptr(sideEffectType))
	if err != nil {
		dropHostCallback(handle)
		return nil, err
	}
	return &FunctionTemplate{iso: i, sc: s, h: h}, nil
}

func sigHandle(s *Signature) uintptr {
	if s == nil {
		return 0
	}
	return s.h
}

// SetClassName sets the constructor function's `name` (v8
// FunctionTemplate::SetClassName).
func (t *FunctionTemplate) SetClassName(name string) error {
	if err := t.check(); err != nil {
		return err
	}
	nameV, err := t.sc.NewString(name)
	if err != nil {
		return err
	}
	return callErr("FunctionTemplate.SetClassName",
		proc("gov8_function_template_set_class_name"),
		t.iso.handle, t.sc.handle, t.h, nameV.h)
}

// GetFunction instantiates the template's function object in the context
// (the unique function per context, exactly as in the oracle). The returned
// Function is bound to the given scope for its wire.
func (t *FunctionTemplate) GetFunction(s *Scope, c *Context) (*Function, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	if c.iso != t.iso {
		return nil, foreignIsolate("context")
	}
	if s.iso != t.iso {
		return nil, foreignIsolate("scope")
	}
	h, err := callHandle("FunctionTemplate.GetFunction",
		proc("gov8_function_template_get_function"),
		t.iso.handle, c.handle, s.handle, t.h)
	if err != nil {
		return nil, err
	}
	return &Function{Value: Value{iso: t.iso, sc: s, h: h}, ctx: c}, nil
}

// Set adds a property to the function itself (a template-level "static"):
// every function instantiated from this template in any context carries it
// (v8 Template::Set). Values must belong to the template's isolate.
func (t *FunctionTemplate) Set(key string, value Value) error {
	return t.SetWithAttr(key, value, AttrNone)
}

// SetWithAttr adds a primitive property with explicit attributes to the
// function object instantiated from this template. Use SetDataWithAttr for
// nested templates and other non-Value Data.
func (t *FunctionTemplate) SetWithAttr(key string, value Value, attr PropertyAttribute) error {
	data, err := value.Data()
	if err != nil {
		return err
	}
	return t.SetDataWithAttr(key, data, attr)
}

// Inherit makes t inherit from parent (v8 FunctionTemplate::Inherit): the
// derived template's prototype's [[Prototype]] becomes the parent's
// prototype, so `instanceof` works for both constructors and prototype
// properties chain. Template-level statics do NOT inherit.
func (t *FunctionTemplate) Inherit(parent *FunctionTemplate) error {
	if err := t.check(); err != nil {
		return err
	}
	if err := parent.check(); err != nil {
		return err
	}
	if parent.iso != t.iso {
		return foreignIsolate("parent template")
	}
	return callErr("FunctionTemplate.Inherit",
		proc("gov8_function_template_inherit"),
		t.iso.handle, t.sc.handle, t.h, parent.h)
}

// ReadOnlyPrototype sets the ReadOnly attribute on the `prototype` property
// of functions created from this template: sloppy-mode assignment silently
// fails (v8 FunctionTemplate::ReadOnlyPrototype).
func (t *FunctionTemplate) ReadOnlyPrototype() error {
	if err := t.check(); err != nil {
		return err
	}
	return callErr("FunctionTemplate.ReadOnlyPrototype",
		proc("gov8_function_template_read_only_prototype"),
		t.iso.handle, t.sc.handle, t.h)
}

// RemovePrototype removes the `prototype` property from functions created
// from this template; `new` rejects with "not a constructor" (v8
// FunctionTemplate::RemovePrototype).
func (t *FunctionTemplate) RemovePrototype() error {
	if err := t.check(); err != nil {
		return err
	}
	return callErr("FunctionTemplate.RemovePrototype",
		proc("gov8_function_template_remove_prototype"),
		t.iso.handle, t.sc.handle, t.h)
}

// PrototypeTemplate returns the object template used as the prototype of
// instances created by this template's constructor.
func (t *FunctionTemplate) PrototypeTemplate() (*ObjectTemplate, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	h, err := callHandle("FunctionTemplate.PrototypeTemplate",
		proc("gov8_function_template_prototype_template"),
		t.iso.handle, t.sc.handle, t.h)
	if err != nil {
		return nil, err
	}
	return &ObjectTemplate{iso: t.iso, sc: t.sc, h: h}, nil
}

// InstanceTemplate returns the object template used for instances created
// when the function is called as a constructor.
func (t *FunctionTemplate) InstanceTemplate() (*ObjectTemplate, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	h, err := callHandle("FunctionTemplate.InstanceTemplate",
		proc("gov8_function_template_instance_template"),
		t.iso.handle, t.sc.handle, t.h)
	if err != nil {
		return nil, err
	}
	return &ObjectTemplate{iso: t.iso, sc: t.sc, h: h}, nil
}

// SetAccessorProperty installs getter (and optionally setter) function
// templates as an accessor property on the template itself — for a function
// template this is a *static* accessor on the constructor function (v8
// FunctionTemplate::SetAccessorProperty). Exactly one of getter/setter must
// be non-nil.
func (t *FunctionTemplate) SetAccessorProperty(key string, getter, setter *FunctionTemplate, attr PropertyAttribute) error {
	if err := t.check(); err != nil {
		return err
	}
	if getter == nil && setter == nil {
		return fmt.Errorf("gov8: accessor property requires a getter or a setter")
	}
	for _, side := range []*FunctionTemplate{getter, setter} {
		if side != nil {
			if err := side.check(); err != nil {
				return err
			}
			if side.iso != t.iso {
				return foreignIsolate("accessor template")
			}
		}
	}
	k, err := t.sc.NewString(key)
	if err != nil {
		return err
	}
	var getterWire, setterWire uintptr
	if getter != nil {
		getterWire = getter.h
	}
	if setter != nil {
		setterWire = setter.h
	}
	return callErr("FunctionTemplate.SetAccessorProperty",
		proc("gov8_function_template_set_accessor_property"),
		t.iso.handle, t.sc.handle, t.h, k.h, getterWire, setterWire,
		uintptr(attr))
}

// Set adds a property to every object created from this template (v8
// Template::Set with PropertyAttribute::None).
func (t *ObjectTemplate) Set(key string, value Value) error {
	return t.SetWithAttr(key, value, AttrNone)
}

// SetWithAttr is Template::Set with explicit property attributes.
func (t *ObjectTemplate) SetWithAttr(key string, value Value, attr PropertyAttribute) error {
	data, err := value.Data()
	if err != nil {
		return err
	}
	return t.SetDataWithAttr(key, data, attr)
}

// ObjectTemplate is a scope-local template for creating objects.
type ObjectTemplate struct {
	iso *Isolate
	sc  *Scope
	h   uintptr
}

func (t *ObjectTemplate) check() error {
	if t.h == 0 {
		return fmt.Errorf("gov8: zero template handle")
	}
	return t.sc.check()
}

// NewObjectTemplate creates an empty object template.
func (i *Isolate) NewObjectTemplate(s *Scope) (*ObjectTemplate, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	ih, err := i.handleChecked()
	if err != nil {
		return nil, err
	}
	h, err := callHandle("ObjectTemplate.New", proc("gov8_object_template_new"),
		ih, s.handle, 0)
	if err != nil {
		return nil, err
	}
	return &ObjectTemplate{iso: i, sc: s, h: h}, nil
}

// NewObjectTemplateFromFunction creates an object template derived from a
// function template (v8 ObjectTemplate::new_from_template): instances inherit
// the function template's prototype object.
func (i *Isolate) NewObjectTemplateFromFunction(s *Scope, ft *FunctionTemplate) (*ObjectTemplate, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if err := ft.check(); err != nil {
		return nil, err
	}
	if ft.iso != i {
		return nil, foreignIsolate("function template")
	}
	ih, err := i.handleChecked()
	if err != nil {
		return nil, err
	}
	h, err := callHandle("ObjectTemplate.NewFromTemplate",
		proc("gov8_object_template_new"), ih, s.handle, ft.h)
	if err != nil {
		return nil, err
	}
	return &ObjectTemplate{iso: i, sc: s, h: h}, nil
}

// NewInstance creates a new object from the template in the context; ok is
// false when creation threw.
func (t *ObjectTemplate) NewInstance(s *Scope, c *Context) (*Object, bool, error) {
	if err := t.check(); err != nil {
		return nil, false, err
	}
	if err := c.check(); err != nil {
		return nil, false, err
	}
	if c.iso != t.iso {
		return nil, false, foreignIsolate("context")
	}
	if s.iso != t.iso {
		return nil, false, foreignIsolate("scope")
	}
	var out uintptr
	r1, _, _ := proc("gov8_object_template_new_instance").Call(
		t.iso.handle, c.handle, s.handle, t.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		serr := shimError("ObjectTemplate.NewInstance", r1)
		if IsException(serr) {
			return nil, false, nil
		}
		return nil, false, serr
	}
	return &Object{Value{iso: t.iso, sc: s, h: out}}, true, nil
}

// SetInternalFieldCount configures every instance created from this template
// to have n internal fields. The bool return mirrors the crate: false only
// when n is out of range.
func (t *ObjectTemplate) SetInternalFieldCount(n int) (bool, error) {
	if err := t.check(); err != nil {
		return false, err
	}
	if n < 0 || n > int(int32max) {
		return false, nil
	}
	if err := callErr("ObjectTemplate.SetInternalFieldCount",
		proc("gov8_object_template_set_internal_field_count"),
		t.iso.handle, t.sc.handle, t.h, uintptr(int32(n))); err != nil {
		return false, err
	}
	return true, nil
}

// InternalFieldCount returns the number of internal fields configured on
// this template.
func (t *ObjectTemplate) InternalFieldCount() (int, error) {
	if err := t.check(); err != nil {
		return 0, err
	}
	r1, _, _ := proc("gov8_object_template_internal_field_count").Call(
		t.iso.handle, t.sc.handle, t.h)
	if int64(r1) < 0 {
		return 0, shimError("ObjectTemplate.InternalFieldCount", r1)
	}
	return int(r1), nil
}

// SetAccessorWithSetter installs a native data property: every read invokes
// the getter, every write the setter, and no backing storage exists (v8
// ObjectTemplate::SetNativeDataProperty). Exactly one callback may be nil to
// install a setter-less or getter-less accessor.
func (t *ObjectTemplate) SetAccessorWithSetter(key string, getter AccessorGetterCallback, setter AccessorSetterCallback) error {
	if err := t.check(); err != nil {
		return err
	}
	if getter == nil && setter == nil {
		return fmt.Errorf("gov8: accessor requires a getter or a setter")
	}
	if getter == nil {
		// The native template installer always exposes a getter trampoline.
		// Keep the legacy setter-only shape safe by dispatching that read to a
		// no-op getter, whose empty ReturnValue is JavaScript undefined.
		getter = func(_ *CallbackScope, _ PropertyCallbackArguments, _ ReturnValue) {}
	}
	handle, err := registerAccessorCallbacks(t.iso, getter, setter, Value{})
	if err != nil {
		return err
	}
	k, err := t.sc.NewString(key)
	if err != nil {
		dropHostCallback(handle)
		return err
	}
	entry := lookupHostCallback(handle)
	if entry == nil {
		return fmt.Errorf("gov8: accessor registration lost")
	}
	withSetter := int32(0)
	if setter != nil {
		withSetter = 1
	}
	err = callErr("ObjectTemplate.SetAccessorWithSetter",
		proc("gov8_object_template_set_native_data_property"),
		t.iso.handle, t.sc.handle, t.h, k.h, entry.ctx, uintptr(withSetter), 0)
	if err != nil {
		dropHostCallback(handle)
	}
	return err
}

// Function is a JS function object created natively (from a template via
// GetFunction, or directly via Isolate.NewFunction). It is a scope-local
// value bound to the context it was created for.
type Function struct {
	Value
	ctx             *Context
	compileMetadata *functionCompileMetadata
}

func (f *Function) check() error {
	if err := f.Value.check(); err != nil {
		return err
	}
	// Value.check already proved the shared isolate's lifecycle and affinity.
	// A Function's context is immutable and belongs to that same isolate, so
	// repeating the thread-id syscall here cannot observe a distinct state.
	return f.ctx.checkAssumingIsolate()
}

// NewFunction creates a native function object directly in the context (v8
// Function::builder(cb)[.length(n)][.data(v)].build / Function::new).
// opts may be nil.
func (i *Isolate) NewFunction(s *Scope, c *Context, cb FunctionCallback, opts *FunctionOptions) (*Function, error) {
	if cb == nil {
		return nil, fmt.Errorf("gov8: nil callback")
	}
	if err := validFunctionOptions(opts); err != nil {
		return nil, err
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	if c.iso != i {
		return nil, foreignIsolate("context")
	}
	if s.iso != i {
		return nil, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	var data Value
	if opts != nil {
		data = opts.Data
	}
	handle, entry, err := registerFunctionCallbackAssumingIsolate(i, cb, data)
	if err != nil {
		return nil, err
	}
	length := 0
	behavior := ConstructorBehaviorDefault
	sideEffectType := SideEffectHasSideEffect
	if opts != nil {
		length = opts.Length
		behavior = opts.ConstructorBehavior
		sideEffectType = opts.SideEffectType
	}
	ensureFunctionNewDirectProc()
	r1, _, _ := syscall.Syscall9(functionNewDirectAddr, 7,
		i.handle, c.handle, sh, entry.ctx,
		uintptr(int32(length)), uintptr(behavior), uintptr(sideEffectType),
		0, 0)
	if int64(r1) < 0 {
		dropHostCallback(handle)
		return nil, shimError("Function.New", r1)
	}
	if r1 == 0 {
		dropHostCallback(handle)
		return nil, shimError("Function.New", 0)
	}
	return &Function{Value: Value{iso: i, sc: s, h: r1}, ctx: c}, nil
}

// Call invokes the function (v8 Function::Call). The receiver and arguments
// must belong to the same isolate; the result wire lives in the given scope.
// ok is false when the call threw (the exception is recorded by the active
// TryCatch).
func (f *Function) Call(s *Scope, recv Value, args ...Value) (Value, bool, error) {
	if err := f.check(); err != nil {
		return Value{}, false, err
	}
	if s != f.sc {
		// Keep the public validation and error ordering unchanged for calls that
		// do not use the Function's creation scope. The common same-scope case
		// was already checked through f.check above.
		if err := s.check(); err != nil {
			return Value{}, false, err
		}
	}
	if s.iso != f.iso {
		return Value{}, false, foreignIsolate("scope")
	}
	if recv.sc == s {
		if recv.h == 0 {
			return Value{}, false, fmt.Errorf("gov8: zero value handle")
		}
	} else {
		if err := recv.check(); err != nil {
			return Value{}, false, err
		}
	}
	for _, a := range args {
		if a.sc == s {
			if a.h == 0 {
				return Value{}, false, fmt.Errorf("gov8: zero value handle")
			}
		} else {
			if err := a.check(); err != nil {
				return Value{}, false, err
			}
		}
		if a.iso != f.iso {
			return Value{}, false, foreignIsolate("argument")
		}
	}
	wires := valueWires(args)
	var argv uintptr
	if len(wires) > 0 {
		argv = uintptr(unsafe.Pointer(&wires[0]))
	}
	var out uintptr
	r1, _, _ := proc("gov8_function_call_ctx").Call(
		f.iso.handle, f.ctx.handle, s.handle, f.h, recv.h,
		uintptr(len(wires)), argv, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		err := shimError("Function.Call", r1)
		if IsException(err) {
			return Value{}, false, nil
		}
		return Value{}, false, err
	}
	return Value{iso: f.iso, sc: s, h: out}, true, nil
}

// NewInstance performs a host-side construct call (v8
// Function::new_instance): the construct callback receives the freshly
// created instance as its receiver. ok is false when the call threw.
func (f *Function) NewInstance(s *Scope, args ...Value) (*Object, bool, error) {
	if err := f.check(); err != nil {
		return nil, false, err
	}
	if err := s.check(); err != nil {
		return nil, false, err
	}
	if s.iso != f.iso {
		return nil, false, foreignIsolate("scope")
	}
	for _, a := range args {
		if err := a.check(); err != nil {
			return nil, false, err
		}
		if a.iso != f.iso {
			return nil, false, foreignIsolate("argument")
		}
	}
	wires := valueWires(args)
	var argv uintptr
	if len(wires) > 0 {
		argv = uintptr(unsafe.Pointer(&wires[0]))
	}
	var out uintptr
	r1, _, _ := proc("gov8_function_new_instance").Call(
		f.iso.handle, f.ctx.handle, s.handle, f.h,
		uintptr(len(wires)), argv, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		err := shimError("Function.NewInstance", r1)
		if IsException(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &Object{Value{iso: f.iso, sc: s, h: out}}, true, nil
}

// Name returns the function's `name` property text.
func (f *Function) Name() (string, error) {
	if err := f.check(); err != nil {
		return "", err
	}
	h, err := callHandle("Function.Name", proc("gov8_function_get_name"),
		f.iso.handle, f.sc.handle, f.h)
	if err != nil {
		return "", err
	}
	name := Value{iso: f.iso, sc: f.sc, h: h}
	return name.ToString(f.ctx)
}

// AsFunction converts a function-valued scope-local value into a *Function
// bound to the given context (the Go analog of the crate's
// Local<Function>::try_from). The value and context must belong to the same
// isolate; ok is false when the value is not a function.
func AsFunction(v Value, c *Context) (*Function, bool, error) {
	if err := v.check(); err != nil {
		return nil, false, err
	}
	if err := c.check(); err != nil {
		return nil, false, err
	}
	if c.iso != v.iso {
		return nil, false, foreignIsolate("context")
	}
	isFn, err := v.IsFunction()
	if err != nil {
		return nil, false, err
	}
	if !isFn {
		return nil, false, nil
	}
	return &Function{Value: v, ctx: c}, true, nil
}
