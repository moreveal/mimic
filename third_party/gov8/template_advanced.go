//go:build (windows || linux) && amd64

package gov8

import (
	"fmt"
)

// Advanced template/object host-interaction surface, pinned to the Rust
// oracle's conformance-template-advanced slice (crate v8 =152.2.0):
//
//   - Named/indexed property interceptors: the full handler family
//     (getter, setter, query, deleter, enumerator, definer, descriptor)
//     with PropertyHandlerFlags, Intercepted fall-through verdicts,
//     callback data round-trips, holder identity and ShouldThrowOnError.
//   - Accessor-SHAPED properties on object templates
//     (ObjectTemplate::set_accessor_property -- function-template getter and
//     setter pair, unlike the native data property of
//     SetAccessorWithSetter).
//   - Intrinsic data properties (Template::SetIntrinsicDataProperty).
//   - Constructor behavior and prototype controls live in template.go
//     (FunctionOptions.ConstructorBehavior, RemovePrototype,
//     ReadOnlyPrototype, Inherit).
//   - Context security tokens: the pinned crate does NOT bind
//     SetAccessCheckCallback; the whole observable access-check surface is
//     the token API below (GetSecurityToken / SetSecurityToken /
//     UseDefaultSecurityToken). An access-check callback is intentionally
//     absent -- it must not be invented.
//   - Value identity (GetHash) and the object template's call-as-function
//     handler and immutable-proto switches.
//
// Ownership rules match the rest of the callback surface: handlers register
// under integer handles in the shared callback registry (no Go pointer ever
// crosses into the engine), dispatch is the single shared trampoline family
// in the shim, and a panic inside any handler is recovered and translated
// into the documented fail-fast process abort. Handler data travels as a
// scope-local Value and must belong to the template's isolate.

// PropertyHandlerFlags mirrors v8::PropertyHandlerFlags (the public bits;
// the engine's internal new-signature bit is managed by the engine bindings
// and never crosses this API).
type PropertyHandlerFlags uint8

const (
	// HandlerFlagNone is the default: every key reaches the handler.
	HandlerFlagNone PropertyHandlerFlags = 0
	// HandlerFlagNonMasking lets an existing own data property win over the
	// getter; absent properties are still intercepted.
	HandlerFlagNonMasking PropertyHandlerFlags = 1
	// HandlerFlagOnlyInterceptStrings bypasses the handler for symbol keys.
	HandlerFlagOnlyInterceptStrings PropertyHandlerFlags = 1 << 1
	// HandlerFlagHasNoSideEffect marks getter/query/enumerator as
	// side-effect-free (only observable under debug-evaluate).
	HandlerFlagHasNoSideEffect PropertyHandlerFlags = 1 << 2
)

func (f PropertyHandlerFlags) valid() bool {
	return f <= HandlerFlagNonMasking|HandlerFlagOnlyInterceptStrings|HandlerFlagHasNoSideEffect
}

// Intercepted mirrors v8::Intercepted. The callback returns InterceptedYes
// when it handled the request (the engine stops the lookup) and
// InterceptedNo to fall through to normal property resolution. The numeric
// values are the engine's own (kYes = 0, kNo = 1).
type Intercepted uint32

const (
	InterceptedYes Intercepted = 0
	InterceptedNo  Intercepted = 1
)

// Named property handler callbacks. key is the property Name; args carries
// holder/this/data/should-throw; rv receives the handler's result where the
// engine expects one (getter value, query attributes, deleter/definer
// boolean, descriptor object).
type (
	NamedPropertyGetterCallback       func(cs *CallbackScope, key Value, args PropertyCallbackArguments, rv ReturnValue) Intercepted
	NamedPropertySetterCallback       func(cs *CallbackScope, key, value Value, args PropertyCallbackArguments, rv ReturnValue) Intercepted
	NamedPropertyQueryCallback        func(cs *CallbackScope, key Value, args PropertyCallbackArguments, rv ReturnValue) Intercepted
	NamedPropertyDeleterCallback      func(cs *CallbackScope, key Value, args PropertyCallbackArguments, rv ReturnValue) Intercepted
	NamedPropertyEnumeratorCallback   func(cs *CallbackScope, args PropertyCallbackArguments, rv ReturnValue)
	NamedPropertyDefinerCallback      func(cs *CallbackScope, key Value, desc CallbackPropertyDescriptor, args PropertyCallbackArguments, rv ReturnValue) Intercepted
	NamedPropertyDescriptorCallback   func(cs *CallbackScope, key Value, args PropertyCallbackArguments, rv ReturnValue) Intercepted
	IndexedPropertyGetterCallback     func(cs *CallbackScope, index uint32, args PropertyCallbackArguments, rv ReturnValue) Intercepted
	IndexedPropertySetterCallback     func(cs *CallbackScope, index uint32, value Value, args PropertyCallbackArguments, rv ReturnValue) Intercepted
	IndexedPropertyQueryCallback      func(cs *CallbackScope, index uint32, args PropertyCallbackArguments, rv ReturnValue) Intercepted
	IndexedPropertyDeleterCallback    func(cs *CallbackScope, index uint32, args PropertyCallbackArguments, rv ReturnValue) Intercepted
	IndexedPropertyEnumeratorCallback func(cs *CallbackScope, args PropertyCallbackArguments, rv ReturnValue)
	IndexedPropertyDefinerCallback    func(cs *CallbackScope, index uint32, desc CallbackPropertyDescriptor, args PropertyCallbackArguments, rv ReturnValue) Intercepted
	IndexedPropertyDescriptorCallback func(cs *CallbackScope, index uint32, args PropertyCallbackArguments, rv ReturnValue) Intercepted
)

// NamedPropertyHandlerConfig mirrors the crate's
// NamedPropertyHandlerConfiguration builder. Data is the handler's callback
// data observed via args.Data(); zero Value means none.
type NamedPropertyHandlerConfig struct {
	Getter     NamedPropertyGetterCallback
	Setter     NamedPropertySetterCallback
	Query      NamedPropertyQueryCallback
	Deleter    NamedPropertyDeleterCallback
	Enumerator NamedPropertyEnumeratorCallback
	Definer    NamedPropertyDefinerCallback
	Descriptor NamedPropertyDescriptorCallback
	Data       Value
	Flags      PropertyHandlerFlags
}

// empty mirrors the crate's configuration.is_some() assert: a configuration
// with neither callbacks nor flags is rejected by this wrapper (as an error)
// instead of reaching the engine.
func (c *NamedPropertyHandlerConfig) empty() bool {
	return c.Getter == nil && c.Setter == nil && c.Query == nil &&
		c.Deleter == nil && c.Enumerator == nil && c.Definer == nil &&
		c.Descriptor == nil && c.Flags == HandlerFlagNone
}

func (c *NamedPropertyHandlerConfig) mask() int32 {
	m := int32(0)
	if c.Getter != nil {
		m |= handlerBitGetter
	}
	if c.Setter != nil {
		m |= handlerBitSetter
	}
	if c.Query != nil {
		m |= handlerBitQuery
	}
	if c.Deleter != nil {
		m |= handlerBitDeleter
	}
	if c.Enumerator != nil {
		m |= handlerBitEnumerator
	}
	if c.Definer != nil {
		m |= handlerBitDefiner
	}
	if c.Descriptor != nil {
		m |= handlerBitDescriptor
	}
	return m
}

// IndexedPropertyHandlerConfig mirrors the crate's
// IndexedPropertyHandlerConfiguration builder.
type IndexedPropertyHandlerConfig struct {
	Getter     IndexedPropertyGetterCallback
	Setter     IndexedPropertySetterCallback
	Query      IndexedPropertyQueryCallback
	Deleter    IndexedPropertyDeleterCallback
	Enumerator IndexedPropertyEnumeratorCallback
	Definer    IndexedPropertyDefinerCallback
	Descriptor IndexedPropertyDescriptorCallback
	Data       Value
	Flags      PropertyHandlerFlags
}

func (c *IndexedPropertyHandlerConfig) empty() bool {
	return c.Getter == nil && c.Setter == nil && c.Query == nil &&
		c.Deleter == nil && c.Enumerator == nil && c.Definer == nil &&
		c.Descriptor == nil && c.Flags == HandlerFlagNone
}

func (c *IndexedPropertyHandlerConfig) mask() int32 {
	m := int32(0)
	if c.Getter != nil {
		m |= handlerBitGetter
	}
	if c.Setter != nil {
		m |= handlerBitSetter
	}
	if c.Query != nil {
		m |= handlerBitQuery
	}
	if c.Deleter != nil {
		m |= handlerBitDeleter
	}
	if c.Enumerator != nil {
		m |= handlerBitEnumerator
	}
	if c.Definer != nil {
		m |= handlerBitDefiner
	}
	if c.Descriptor != nil {
		m |= handlerBitDescriptor
	}
	return m
}

// Callback presence masks shared with the shim installers.
const (
	handlerBitGetter     = 1
	handlerBitSetter     = 2
	handlerBitQuery      = 4
	handlerBitDeleter    = 8
	handlerBitEnumerator = 16
	handlerBitDefiner    = 32
	handlerBitDescriptor = 64
)

// SetNamedPropertyHandler installs the named property interceptor family on
// the template (v8 ObjectTemplate::SetHandler with a
// NamedPropertyHandlerConfiguration).
func (t *ObjectTemplate) SetNamedPropertyHandler(cfg NamedPropertyHandlerConfig) error {
	if err := t.check(); err != nil {
		return err
	}
	if cfg.empty() {
		return fmt.Errorf("gov8: property handler configuration requires a callback or flags")
	}
	if !cfg.Flags.valid() {
		return fmt.Errorf("gov8: property handler flags %d out of range", cfg.Flags)
	}
	handle, err := registerHostEntry(t.iso, &hostCallbackEntry{
		nget: cfg.Getter, nset: cfg.Setter, nquery: cfg.Query,
		ndel: cfg.Deleter, nenum: cfg.Enumerator, ndefine: cfg.Definer,
		ndesc: cfg.Descriptor,
	}, cfg.Data)
	if err != nil {
		return err
	}
	entry := lookupHostCallback(handle)
	if entry == nil {
		return fmt.Errorf("gov8: handler registration lost")
	}
	err = callErr("ObjectTemplate.SetNamedPropertyHandler",
		proc("gov8_object_template_set_named_property_handler"),
		t.iso.handle, t.sc.handle, t.h, entry.ctx,
		uintptr(cfg.mask()), uintptr(cfg.Flags))
	if err != nil {
		dropHostCallback(handle)
	}
	return err
}

// SetIndexedPropertyHandler installs the indexed property interceptor family
// on the template.
func (t *ObjectTemplate) SetIndexedPropertyHandler(cfg IndexedPropertyHandlerConfig) error {
	if err := t.check(); err != nil {
		return err
	}
	if cfg.empty() {
		return fmt.Errorf("gov8: property handler configuration requires a callback or flags")
	}
	if !cfg.Flags.valid() {
		return fmt.Errorf("gov8: property handler flags %d out of range", cfg.Flags)
	}
	handle, err := registerHostEntry(t.iso, &hostCallbackEntry{
		iget: cfg.Getter, iset: cfg.Setter, iquery: cfg.Query,
		idel: cfg.Deleter, ienum: cfg.Enumerator, idefine: cfg.Definer,
		idesc: cfg.Descriptor,
	}, cfg.Data)
	if err != nil {
		return err
	}
	entry := lookupHostCallback(handle)
	if entry == nil {
		return fmt.Errorf("gov8: handler registration lost")
	}
	err = callErr("ObjectTemplate.SetIndexedPropertyHandler",
		proc("gov8_object_template_set_indexed_property_handler"),
		t.iso.handle, t.sc.handle, t.h, entry.ctx,
		uintptr(cfg.mask()), uintptr(cfg.Flags))
	if err != nil {
		dropHostCallback(handle)
	}
	return err
}

// CallbackPropertyDescriptor is the Go view of the v8::PropertyDescriptor
// snapshot a definer callback receives. (runtime_values.go already owns the
// name PropertyDescriptor for the engine-handle wrapper used by
// Object.defineProperty; this read-only view is the callback-side shape.)
// Presence flags mirror the C++ has_* accessors; the value fields are only
// meaningful when the corresponding has bit is set. It is bound to the
// running callback and must not outlive it.
type CallbackPropertyDescriptor struct {
	flags          int32
	getter, setter Value
	hasValue       bool
	value          Value
	writable       bool
	enumerable     bool
	configurable   bool
}

// HasValue reports whether the descriptor carries a value.
func (d CallbackPropertyDescriptor) HasValue() bool { return d.flags&pdFlagHasValue != 0 }

// Value returns the descriptor's value; ok is false when absent.
func (d CallbackPropertyDescriptor) Value() (Value, bool, error) {
	if !d.HasValue() {
		return Value{}, false, nil
	}
	if err := d.value.check(); err != nil {
		return Value{}, false, err
	}
	return d.value, true, nil
}

// HasWritable reports whether the descriptor carries a writable flag.
func (d CallbackPropertyDescriptor) HasWritable() bool { return d.flags&pdFlagHasWritable != 0 }

// Writable returns the descriptor's writable flag.
func (d CallbackPropertyDescriptor) Writable() bool { return d.writable }

// HasEnumerable reports whether the descriptor carries an enumerable flag.
func (d CallbackPropertyDescriptor) HasEnumerable() bool { return d.flags&pdFlagHasEnumerable != 0 }

// Enumerable returns the descriptor's enumerable flag.
func (d CallbackPropertyDescriptor) Enumerable() bool { return d.enumerable }

// HasConfigurable reports whether the descriptor carries a configurable flag.
func (d CallbackPropertyDescriptor) HasConfigurable() bool { return d.flags&pdFlagHasConfigurable != 0 }

// Configurable returns the descriptor's configurable flag.
func (d CallbackPropertyDescriptor) Configurable() bool { return d.configurable }

// --- accessor-shaped properties -------------------------------------------------

// SetAccessorProperty installs an accessor-SHAPED property on the template
// (v8 ObjectTemplate::SetAccessorProperty): instances expose function-valued
// get/set in their property descriptor, unlike the native data property of
// SetAccessorWithSetter. Exactly one of getter/setter must be non-nil.
func (t *ObjectTemplate) SetAccessorProperty(key string, getter, setter *FunctionTemplate, attr PropertyAttribute) error {
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
	return callErr("ObjectTemplate.SetAccessorProperty",
		proc("gov8_object_template_set_accessor_property"),
		t.iso.handle, t.sc.handle, t.h, k.h, getterWire, setterWire,
		uintptr(attr))
}

// --- intrinsic data properties ----------------------------------------------------

// Intrinsic mirrors v8::Intrinsic: context-owned intrinsic objects that can
// be bound as data properties at template instantiation. Values match the
// pinned header's enum order.
type Intrinsic uint8

const (
	IntrinsicArrayProtoEntries      Intrinsic = 0
	IntrinsicArrayProtoForEach      Intrinsic = 1
	IntrinsicArrayProtoKeys         Intrinsic = 2
	IntrinsicArrayProtoValues       Intrinsic = 3
	IntrinsicArrayPrototype         Intrinsic = 4
	IntrinsicAsyncIteratorPrototype Intrinsic = 5
	IntrinsicErrorPrototype         Intrinsic = 6
	IntrinsicIteratorPrototype      Intrinsic = 7
	IntrinsicMapIteratorPrototype   Intrinsic = 8
	IntrinsicObjProtoValueOf        Intrinsic = 9
	IntrinsicSetIteratorPrototype   Intrinsic = 10
)

// SetIntrinsicDataProperty binds one of the context's real intrinsic objects
// (e.g. Array.prototype) as a data property on every instance created from
// this template (v8 Template::SetIntrinsicDataProperty), with attr applied.
func (t *ObjectTemplate) SetIntrinsicDataProperty(key string, intrinsic Intrinsic, attr PropertyAttribute) error {
	return t.setIntrinsicDataProperty(key, intrinsic, attr)
}

// SetIntrinsicDataProperty is the FunctionTemplate flavor of the Template
// base method: instances created from the template's constructor receive the
// property.
func (t *FunctionTemplate) SetIntrinsicDataProperty(key string, intrinsic Intrinsic, attr PropertyAttribute) error {
	return (&ObjectTemplate{iso: t.iso, sc: t.sc, h: t.h}).setIntrinsicDataProperty(key, intrinsic, attr)
}

func (t *ObjectTemplate) setIntrinsicDataProperty(key string, intrinsic Intrinsic, attr PropertyAttribute) error {
	if err := t.check(); err != nil {
		return err
	}
	if intrinsic > IntrinsicSetIteratorPrototype {
		return fmt.Errorf("gov8: intrinsic %d out of range", intrinsic)
	}
	k, err := t.sc.NewString(key)
	if err != nil {
		return err
	}
	return callErr("Template.SetIntrinsicDataProperty",
		proc("gov8_template_set_intrinsic_data_property"),
		t.iso.handle, t.sc.handle, t.h, k.h, uintptr(uint8(intrinsic)),
		uintptr(attr))
}

// --- object template behavior switches ---------------------------------------------

// SetImmutableProto makes every instance created from this template an
// immutable-prototype exotic object: setPrototypeOf (and __proto__
// assignment) THROWS instead of silently failing (v8
// ObjectTemplate::SetImmutableProto).
func (t *ObjectTemplate) SetImmutableProto() error {
	if err := t.check(); err != nil {
		return err
	}
	return callErr("ObjectTemplate.SetImmutableProto",
		proc("gov8_object_template_set_immutable_proto"),
		t.iso.handle, t.sc.handle, t.h)
}

// SetCallAsFunctionHandler makes every instance created from this template
// callable (v8 ObjectTemplate::SetCallAsFunctionHandler): plain calls and
// construct calls both dispatch to cb (IsConstructCall distinguishes them)
// and even primitive return values are delivered as construct results. data
// is observed via args.Data(); zero Value means none.
func (t *ObjectTemplate) SetCallAsFunctionHandler(cb FunctionCallback, data Value) error {
	if err := t.check(); err != nil {
		return err
	}
	if cb == nil {
		return fmt.Errorf("gov8: nil callback")
	}
	handle, err := registerFunctionCallback(t.iso, cb, data)
	if err != nil {
		return err
	}
	entry := lookupHostCallback(handle)
	if entry == nil {
		return fmt.Errorf("gov8: callback registration lost")
	}
	err = callErr("ObjectTemplate.SetCallAsFunctionHandler",
		proc("gov8_object_template_set_call_as_function_handler"),
		t.iso.handle, t.sc.handle, t.h, entry.ctx)
	if err != nil {
		dropHostCallback(handle)
	}
	return err
}

// SetData installs a template-derived value (a FunctionTemplate) as a plain
// property on this object template -- Template::Set where the Data value is
// another template (e.g. putting a method template on a prototype template).
func (t *ObjectTemplate) SetData(key string, ft *FunctionTemplate) error {
	if err := t.check(); err != nil {
		return err
	}
	if err := ft.check(); err != nil {
		return err
	}
	if ft.iso != t.iso {
		return foreignIsolate("template value")
	}
	k, err := t.sc.NewString(key)
	if err != nil {
		return err
	}
	return callErr("ObjectTemplate.SetData", proc("gov8_template_set"),
		t.iso.handle, t.sc.handle, t.h, k.h, ft.h, uintptr(AttrNone))
}

// --- value identity ------------------------------------------------------------------

// GetHash returns the value's identity hash (v8 Value::GetHash): stable per
// object/name within one isolate, not across isolates or processes. Hash
// equality is a proxy for object identity, mirroring the oracle's
// get_hash()-based observations.
func (v Value) GetHash() (uint32, error) {
	if err := v.check(); err != nil {
		return 0, err
	}
	if err := requireInitialized(); err != nil {
		return 0, err
	}
	r1, _, _ := proc("gov8_value_get_hash").Call(
		v.iso.handleAssumingCheck(), v.h)
	if int64(r1) < 0 {
		return 0, shimError("GetHash", r1)
	}
	return uint32(r1), nil
}

// --- context security tokens -----------------------------------------------------------
//
// The pinned crate binds no access-check callback; the whole observable
// access-check surface is the security-token API. Each context's default
// token is its own global object, so fresh contexts mutually distrust each
// other: touching another context's global proxy throws
// "TypeError: no access" while bridged plain objects stay readable across
// tokens. Sharing one token value re-enables access;
// UseDefaultSecurityToken restores the own-global token.

// GetSecurityToken returns the context's current security token (v8
// Context::GetSecurityToken).
func (c *Context) GetSecurityToken(s *Scope) (Value, error) {
	if err := c.check(); err != nil {
		return Value{}, err
	}
	if s.iso != c.iso {
		return Value{}, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, err
	}
	h, err := callHandle("Context.GetSecurityToken",
		proc("gov8_context_get_security_token"),
		c.iso.handleAssumingCheck(), c.handle, sh)
	if err != nil {
		return Value{}, err
	}
	return Value{iso: c.iso, sc: s, h: h}, nil
}

// SetSecurityToken shares another context's token with this context (v8
// Context::SetSecurityToken), re-enabling cross-context global access.
func (c *Context) SetSecurityToken(s *Scope, token Value) error {
	if err := c.check(); err != nil {
		return err
	}
	if s.iso != c.iso {
		return foreignIsolate("scope")
	}
	if err := token.check(); err != nil {
		return err
	}
	if token.iso != c.iso {
		return foreignIsolate("token")
	}
	return callErr("Context.SetSecurityToken",
		proc("gov8_context_set_security_token"),
		c.iso.handleAssumingCheck(), c.handle, token.h)
}

// UseDefaultSecurityToken restores the context's own global object as its
// security token (v8 Context::UseDefaultSecurityToken).
func (c *Context) UseDefaultSecurityToken() error {
	if err := c.check(); err != nil {
		return err
	}
	return callErr("Context.UseDefaultSecurityToken",
		proc("gov8_context_use_default_security_token"),
		c.iso.handleAssumingCheck(), c.handle)
}

// MarkAsUndetectable gives instances the HTMLDDA operator semantics used by document.all.
func (t *ObjectTemplate) MarkAsUndetectable() error {
	if err := t.check(); err != nil {
		return err
	}
	return callErr("ObjectTemplate.MarkAsUndetectable", proc("gov8_object_template_mark_as_undetectable"), t.iso.handle, t.sc.handle, t.h)
}

func (d CallbackPropertyDescriptor) Get() (Value, bool, error) {
	if d.flags&16 == 0 {
		return Value{}, false, nil
	}
	return d.getter, true, d.getter.check()
}
func (d CallbackPropertyDescriptor) Set() (Value, bool, error) {
	if d.flags&32 == 0 {
		return Value{}, false, nil
	}
	return d.setter, true, d.setter.check()
}
