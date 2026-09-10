//go:build windows && amd64

package gov8

import (
	"fmt"
	"math"
	"unsafe"
)

// Runtime values: the JavaScript built-ins reachable through native APIs,
// mirroring the observable contract the pinned Rust oracle characterizes
// (rust-oracle/src/bin/conformance-runtime-values.rs, crate v8 =152.2.0):
//
//   - Date: NewDate / (*Date).ValueOf, JS interop, invalid-time boundaries.
//   - RegExp: NewRegExp / Exec / GetSource with RegExpFlags; the
//     invalid-pattern SyntaxError is reported through the caller's TryCatch
//     (the tc argument, matching Context.Compile/Script.Run) and Exec keeps
//     the pinned Option/null shape: a miss is a non-nil result wrapping the
//     null value, only a thrown exception yields an error.
//   - JSON: JSONParse / JSONStringify with TryCatch-routed failures.
//   - Array: NewArray (negative lengths forwarded verbatim — the engine
//     collapses them to an empty array, unlike the JS constructor),
//     NewArrayWithElements, Length and the index property family.
//   - Map / Set: native collections with engine SameValueZero keys;
//     insertions return the collection so callers can observe identity.
//   - Proxy: NewProxy / GetTarget / GetHandler / IsRevoked / Revoke; a
//     revoked proxy's target reads as the JavaScript null value.
//   - Symbol and Private: construction, registries (ForKey = Symbol.for,
//     SymbolForApi = the embedder-only registry), well-known getters,
//     Description, and the Object private-key family invisible to JS.
//   - Primitive wrapper predicates (Number/Boolean/String/BigInt objects)
//     plus IsTrue and IsName.
//   - Property attributes (PropertyAttribute), integrity levels,
//     PropertyDescriptor, GetOwnPropertyDescriptor and GetPropertyNames
//     filters.
//
// Ownership and lifetime rules:
//
//   - Every value produced here is a scope-local Value bound to the Scope
//     passed to the call; it must not outlive that Scope.
//   - Typed wrappers (Date, RegExp, ...) are constructed either natively or
//     through an As* cast that prevalidates the engine kind BEFORE any typed
//     call is made; a mis-typed wrapper is refused with an error instead of
//     entering a typed engine binding (which would be undefined behavior).
//     The shim re-checks the kind at the ABI boundary as defense in depth.
//   - PropertyDescriptor holds scope-local handle slots: it must be Closed
//     before the Scope it was created in closes, and its stored values are
//     only valid while that Scope is open (enforced by every accessor).
//   - Everything is thread-affine like the rest of the module: all calls
//     must run on the owning isolate thread (enforced by Isolate.check).
//
// Intentional API-shape differences from the pinned crate (semantics
// preserved):
//
//   - Option<Local<T>> maps to (T, error): an empty maybe (a pending
//     exception) is an error; misses that the engine reports as real values
//     (null match object, undefined descriptor) are returned as values.
//   - Maybe<bool> maps to (bool, error): Nothing is an error, Just(b) is
//     (b, nil). The core's legacy GetByName/SetByName fold the Maybe into
//     ok for historical reasons; this slice preserves the three states.
//   - Missing-key GetPropertyAttributes is (None, true, nil): the engine
//     reports Just(NONE) for a missing property, not an empty maybe.
//   - Map.Set/Set.Add return the collection (the pinned crate returns the
//     same handle; Go callers compare with Same to observe identity).

// --- additional value predicates ------------------------------------------------

// IsDate reports whether the value is a Date object.
func (v Value) IsDate() (bool, error) { return v.predicate("gov8_rv_is_date") }

// IsRegExp reports whether the value is a RegExp object.
func (v Value) IsRegExp() (bool, error) { return v.predicate("gov8_rv_is_reg_exp") }

// IsMap reports whether the value is a Map object.
func (v Value) IsMap() (bool, error) { return v.predicate("gov8_rv_is_map") }

// IsSet reports whether the value is a Set object.
func (v Value) IsSet() (bool, error) { return v.predicate("gov8_rv_is_set") }

// IsProxy reports whether the value is a Proxy exotic object.
func (v Value) IsProxy() (bool, error) { return v.predicate("gov8_rv_is_proxy") }

// IsSymbol reports whether the value is a symbol primitive.
func (v Value) IsSymbol() (bool, error) { return v.predicate("gov8_rv_is_symbol") }

// IsTrue reports whether the value is exactly the primitive true. A Boolean
// wrapper object is not true by this predicate even when ToBoolean of it
// would be (BooleanValue).
func (v Value) IsTrue() (bool, error) { return v.predicate("gov8_rv_is_true") }

// IsName reports whether the value can be used as a property name (a string
// or a symbol primitive).
func (v Value) IsName() (bool, error) { return v.predicate("gov8_rv_is_name") }

// IsBigInt reports whether the value is a BigInt primitive.
func (v Value) IsBigInt() (bool, error) { return v.predicate("gov8_rv_is_big_int") }

// IsNumberObject reports whether the value is a Number wrapper object.
func (v Value) IsNumberObject() (bool, error) {
	return v.predicate("gov8_rv_is_number_object")
}

// IsBooleanObject reports whether the value is a Boolean wrapper object.
func (v Value) IsBooleanObject() (bool, error) {
	return v.predicate("gov8_rv_is_boolean_object")
}

// IsStringObject reports whether the value is a String wrapper object.
func (v Value) IsStringObject() (bool, error) {
	return v.predicate("gov8_rv_is_string_object")
}

// IsBigIntObject reports whether the value is a BigInt wrapper object.
func (v Value) IsBigIntObject() (bool, error) {
	return v.predicate("gov8_rv_is_big_int_object")
}

// TypeOf returns the "typeof" string of the value (a scope-local string
// value). The scope must belong to the value's isolate.
func (v Value) TypeOf(s *Scope) (Value, error) {
	if err := v.check(); err != nil {
		return Value{}, err
	}
	if s.iso != v.iso {
		return Value{}, foreignIsolate("scope")
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_type_of").Call(v.iso.handle, sh, v.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("TypeOf", r1)
	}
	return Value{iso: s.iso, sc: s, h: out}, nil
}

// ToStringTC is the TryCatch-routed ToString: unlike Value.ToString (whose
// shim path installs an internal TryCatch and swallows conversion
// failures), a throwing conversion here is delivered to tc (HasCaught and
// MessageText observe it) and reported as an error. Use it when the value
// may not convert (symbols); tc follows the Compile/Run convention.
func (v Value) ToStringTC(s *Scope, c *Context, tc *TryCatch) (string, error) {
	if err := v.ctxHandle(c); err != nil {
		return "", err
	}
	sh, err := v.sc.checkedHandle()
	if err != nil {
		return "", err
	}
	tcv, err := tcArg(v.iso, tc)
	if err != nil {
		return "", err
	}
	return callTextFn("ToStringTC", func(buf *byte, cap int, outLen *int64) uintptr {
		r, _, _ := proc("gov8_rv_value_to_string_utf8").Call(
			v.iso.handle, c.handle, sh, tcv, v.h,
			uintptr(unsafe.Pointer(buf)), uintptr(cap), uintptr(unsafe.Pointer(outLen)))
		return r
	})
}

// --- shared validation helpers ----------------------------------------------------

// typedCast prevalidates the engine kind and returns v unchanged. castName
// appears in the error message.
func typedCast(v Value, is func() (bool, error), kind string) (Value, error) {
	if err := v.check(); err != nil {
		return Value{}, err
	}
	ok, err := is()
	if err != nil {
		return Value{}, err
	}
	if !ok {
		return Value{}, fmt.Errorf("gov8: value is not a %s", kind)
	}
	return v, nil
}

// requireString validates that v is a non-empty handle holding a JS string
// of its own isolate. Guards every shim path that dereferences a wire as
// const String&.
func (v Value) requireString() error {
	if _, err := typedCast(v, v.IsString, "String"); err != nil {
		return err
	}
	return nil
}

// requireName validates that v is a non-empty handle holding a property
// name (string or symbol). Guards the Name-keyed object operations.
func (v Value) requireName() error {
	if _, err := typedCast(v, v.IsName, "Name"); err != nil {
		return err
	}
	return nil
}

// optionalString validates description: an absent (zero) value is allowed
// and reported as false; a present value must be a string of v's isolate.
func (v Value) optionalString(s *Scope) (present bool, err error) {
	if v.h == 0 {
		return false, nil
	}
	if err := v.requireString(); err != nil {
		return false, err
	}
	if v.iso != s.iso {
		return false, foreignIsolate("description")
	}
	return true, nil
}

// scopeAndValue validates the scope handle for a call on v's isolate.
func (v Value) scopeArg(s *Scope) (uintptr, error) {
	if err := v.check(); err != nil {
		return 0, err
	}
	if s.iso != v.iso {
		return 0, foreignIsolate("scope")
	}
	return s.checkedHandle()
}

// boolResult converts a Maybe<bool>-shaped shim call result.
func boolResult(op string, r1, okv uintptr) (bool, error) {
	if int64(r1) < 0 {
		return false, shimError(op, r1)
	}
	return okv == 1, nil
}

// tcArg validates an optional TryCatch argument and returns its shim handle
// (0 when absent).
func tcArg(iso *Isolate, tc *TryCatch) (uintptr, error) {
	if tc == nil {
		return 0, nil
	}
	if tc.iso != iso {
		return 0, foreignIsolate("trycatch")
	}
	if err := tc.check(); err != nil {
		return 0, err
	}
	return tc.handle, nil
}

// --- Date -----------------------------------------------------------------------

// Date is a JS Date object.
type Date struct{ Value }

// NewDate creates a Date holding the given time value (milliseconds since
// the epoch; any double is accepted, including NaN for an invalid date).
// The context and scope must belong to the same isolate.
func (s *Scope) NewDate(c *Context, t float64) (*Date, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if c == nil || c.iso != s.iso {
		return nil, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_date_new").Call(
		s.iso.handle, c.handle, uintptr(math.Float64bits(t)), s.handle,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewDate", r1)
	}
	return &Date{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// ValueOf returns the stored time value (NaN for an invalid date).
func (d *Date) ValueOf() (float64, error) {
	if err := d.check(); err != nil {
		return 0, err
	}
	ih, err := d.iso.handleChecked()
	if err != nil {
		return 0, err
	}
	var out float64
	r1, _, _ := proc("gov8_rv_date_value_of").Call(ih, d.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("Date.ValueOf", r1)
	}
	return out, nil
}

// AsDate casts a value to a Date after prevalidating the engine kind.
func AsDate(v Value) (*Date, error) {
	vv, err := typedCast(v, v.IsDate, "Date")
	if err != nil {
		return nil, err
	}
	return &Date{Value: vv}, nil
}

// --- RegExp -----------------------------------------------------------------------

// RegExpFlags mirrors v8::RegExp::Flags / the crate's RegExpCreationFlags.
type RegExpFlags uint32

const (
	RegExpGlobal      RegExpFlags = 1 << iota // g
	RegExpIgnoreCase                          // i
	RegExpMultiline                           // m
	RegExpSticky                              // y
	RegExpUnicode                             // u
	RegExpDotAll                              // s
	RegExpLinear                              // l (experimental engine)
	RegExpHasIndices                          // d
	RegExpUnicodeSets                         // v
)

// RegExp is a JS RegExp object.
type RegExp struct{ Value }

// NewRegExp compiles pattern with the given flags. A syntax error is
// reported through tc when given (the engine's SyntaxError with the exact
// "Uncaught " message text) and an error is returned; with tc nil a shim-
// internal TryCatch observes the failure and only the error is returned.
// pattern must be a JS string.
func (s *Scope) NewRegExp(c *Context, pattern Value, flags RegExpFlags, tc *TryCatch) (*RegExp, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if c == nil || c.iso != s.iso {
		return nil, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	if err := pattern.requireString(); err != nil {
		return nil, err
	}
	if pattern.iso != s.iso {
		return nil, foreignIsolate("pattern")
	}
	tcv, err := tcArg(s.iso, tc)
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_regexp_new").Call(
		s.iso.handle, c.handle, s.handle, tcv, pattern.h,
		uintptr(uint32(flags)), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewRegExp", r1)
	}
	return &RegExp{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// Exec runs the regexp against subject, honoring and updating lastIndex for
// global/sticky patterns. A miss is a non-nil result whose Value is the
// null value (the pinned Some(null) shape); a thrown exception returns a
// nil result and an error (the exception stays observable through any
// active TryCatch). subject must be a JS string.
func (re *RegExp) Exec(s *Scope, c *Context, subject Value) (*Object, error) {
	if err := re.check(); err != nil {
		return nil, err
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	if c.iso != re.iso {
		return nil, foreignIsolate("context")
	}
	sh, err := re.scopeArg(s)
	if err != nil {
		return nil, err
	}
	if err := subject.requireString(); err != nil {
		return nil, err
	}
	if subject.iso != re.iso {
		return nil, foreignIsolate("subject")
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_regexp_exec").Call(
		re.iso.handle, c.handle, sh, re.h, subject.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("RegExp.Exec", r1)
	}
	return &Object{Value: Value{iso: re.iso, sc: s, h: out}}, nil
}

// GetSource returns the pattern source verbatim (a scope-local string).
func (re *RegExp) GetSource(s *Scope) (Value, error) {
	sh, err := re.scopeArg(s)
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_regexp_get_source").Call(
		re.iso.handle, sh, re.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("RegExp.GetSource", r1)
	}
	return Value{iso: re.iso, sc: s, h: out}, nil
}

// AsRegExp casts a value to a RegExp after prevalidating the engine kind.
func AsRegExp(v Value) (*RegExp, error) {
	vv, err := typedCast(v, v.IsRegExp, "RegExp")
	if err != nil {
		return nil, err
	}
	return &RegExp{Value: vv}, nil
}

// --- JSON ------------------------------------------------------------------------

// JSONParse parses text as JSON. A malformed input throws a SyntaxError
// which is reported through tc when given (HasCaught and MessageText carry
// the pinned message) and an error is returned; with tc nil the exception
// is observed by a shim-internal TryCatch and only the error is returned.
// text must be a JS string.
func JSONParse(c *Context, s *Scope, text Value, tc *TryCatch) (Value, error) {
	if err := c.check(); err != nil {
		return Value{}, err
	}
	if err := s.check(); err != nil {
		return Value{}, err
	}
	if s.iso != c.iso {
		return Value{}, foreignIsolate("scope")
	}
	if err := text.requireString(); err != nil {
		return Value{}, err
	}
	if text.iso != s.iso {
		return Value{}, foreignIsolate("text")
	}
	tcv, err := tcArg(s.iso, tc)
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_json_parse").Call(
		c.iso.handle, c.handle, s.handle, tcv, text.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("JSONParse", r1)
	}
	return Value{iso: s.iso, sc: s, h: out}, nil
}

// JSONStringify renders value as JSON text. Circular structures and toJSON
// failures throw a TypeError which is reported through tc when given (and
// an error returned); the top-level undefined/function/symbol boundaries
// are NOT errors — the engine renders them as the literal string
// "undefined" (the pinned C++ quirk). Any value kind is accepted.
func JSONStringify(c *Context, s *Scope, value Value, tc *TryCatch) (Value, error) {
	if err := c.check(); err != nil {
		return Value{}, err
	}
	if err := s.check(); err != nil {
		return Value{}, err
	}
	if s.iso != c.iso {
		return Value{}, foreignIsolate("scope")
	}
	if err := value.check(); err != nil {
		return Value{}, err
	}
	if value.iso != s.iso {
		return Value{}, foreignIsolate("value")
	}
	tcv, err := tcArg(s.iso, tc)
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_json_stringify").Call(
		c.iso.handle, c.handle, s.handle, tcv, value.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("JSONStringify", r1)
	}
	return Value{iso: s.iso, sc: s, h: out}, nil
}

// --- Array -----------------------------------------------------------------------

// Array is a JS Array object.
type Array struct{ Value }

// NewArray creates an array of the given length. Negative lengths are
// forwarded verbatim and collapse to an empty array (the pinned native-API
// boundary; the JS constructor throws a RangeError instead). The context
// and scope must belong to the same isolate.
func (s *Scope) NewArray(c *Context, length int32) (*Array, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if c == nil || c.iso != s.iso {
		return nil, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_array_new").Call(
		s.iso.handle, c.handle, sh, uintptr(int32(length)), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewArray", r1)
	}
	return &Array{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// NewArrayWithElements creates an array holding the given elements
// verbatim. All elements must belong to the scope's isolate.
func (s *Scope) NewArrayWithElements(c *Context, elements []Value) (*Array, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if c == nil || c.iso != s.iso {
		return nil, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, err
	}
	for _, e := range elements {
		if err := e.check(); err != nil {
			return nil, err
		}
		if e.iso != s.iso {
			return nil, foreignIsolate("element")
		}
	}
	// The wire array is passed directly to the artifact binding; keep a
	// valid non-nil pointer for the empty case.
	var dummy [1]uintptr
	wires := dummy[:]
	if len(elements) > 0 {
		wires = make([]uintptr, len(elements))
		for i, e := range elements {
			wires[i] = e.h
		}
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_array_new_with_elements").Call(
		s.iso.handle, c.handle, sh, uintptr(unsafe.Pointer(&wires[0])),
		uintptr(len(elements)), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewArrayWithElements", r1)
	}
	return &Array{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// Length returns the array length (the full uint32 range is preserved).
func (a *Array) Length() (int64, error) {
	if err := a.check(); err != nil {
		return 0, err
	}
	ih, err := a.iso.handleChecked()
	if err != nil {
		return 0, err
	}
	var out uint32
	r1, _, _ := proc("gov8_rv_array_length").Call(ih, a.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("Array.Length", r1)
	}
	return int64(out), nil
}

// GetIndex reads an index property. A missing index reads as the undefined
// value; only a thrown exception returns an error.
func (a *Array) GetIndex(s *Scope, c *Context, index uint32) (Value, error) {
	if err := a.ctxHandle(c); err != nil {
		return Value{}, err
	}
	sh, err := a.scopeArg(s)
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_array_get_index").Call(
		a.iso.handle, c.handle, sh, a.h, uintptr(index), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("Array.GetIndex", r1)
	}
	return Value{iso: a.iso, sc: s, h: out}, nil
}

// SetIndex writes an index property (growing length as the JS semantics
// require). ok is Just(false) when the write was ignored; an error means
// the operation threw.
func (a *Array) SetIndex(s *Scope, c *Context, index uint32, v Value) (bool, error) {
	if err := a.ctxHandle(c); err != nil {
		return false, err
	}
	sh, err := a.scopeArg(s)
	if err != nil {
		return false, err
	}
	if err := v.check(); err != nil {
		return false, err
	}
	if v.iso != a.iso {
		return false, foreignIsolate("value")
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_array_set_index").Call(
		a.iso.handle, c.handle, sh, a.h, uintptr(index), v.h,
		uintptr(unsafe.Pointer(&okv)))
	return boolResult("Array.SetIndex", r1, uintptr(okv))
}

// HasIndex reports whether the index property exists. An error means the
// operation threw (the Maybe is Nothing); otherwise ok is Just(b).
func (a *Array) HasIndex(s *Scope, c *Context, index uint32) (bool, error) {
	if err := a.ctxHandle(c); err != nil {
		return false, err
	}
	sh, err := a.scopeArg(s)
	if err != nil {
		return false, err
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_array_has_index").Call(
		a.iso.handle, c.handle, sh, a.h, uintptr(index), uintptr(unsafe.Pointer(&okv)))
	return boolResult("Array.HasIndex", r1, uintptr(okv))
}

// AsArray casts a value to an Array after prevalidating the engine kind.
func AsArray(v Value) (*Array, error) {
	vv, err := typedCast(v, v.IsArray, "Array")
	if err != nil {
		return nil, err
	}
	return &Array{Value: vv}, nil
}

// --- Map -----------------------------------------------------------------------

// Map is a JS Map object (engine SameValueZero keys: NaN keys work and +0/-0
// are the same key).
type Map struct{ Value }

// NewMap creates an empty Map. The context and scope must belong to the
// same isolate.
func (s *Scope) NewMap(c *Context) (*Map, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if c == nil || c.iso != s.iso {
		return nil, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_map_new").Call(s.iso.handle, c.handle, sh, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewMap", r1)
	}
	return &Map{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// Set inserts or overwrites the mapping and returns the collection itself
// (a fresh wrapper over the engine-returned handle; compare with Same to
// observe identity, matching the pinned returned-handle check).
func (m *Map) Set(s *Scope, c *Context, key, value Value) (*Map, error) {
	sh, err := m.mapCallArgs(s, c, &key, &value)
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_map_set").Call(
		m.iso.handle, c.handle, sh, m.h, key.h, value.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("Map.Set", r1)
	}
	return &Map{Value: Value{iso: m.iso, sc: s, h: out}}, nil
}

// Get returns the value stored for key (the undefined value when absent).
func (m *Map) Get(s *Scope, c *Context, key Value) (Value, error) {
	sh, err := m.mapCallArgs(s, c, &key, nil)
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_map_get").Call(
		m.iso.handle, c.handle, sh, m.h, key.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("Map.Get", r1)
	}
	return Value{iso: m.iso, sc: s, h: out}, nil
}

// Has reports key membership.
func (m *Map) Has(s *Scope, c *Context, key Value) (bool, error) {
	sh, err := m.mapCallArgs(s, c, &key, nil)
	if err != nil {
		return false, err
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_map_has").Call(
		m.iso.handle, c.handle, sh, m.h, key.h, uintptr(unsafe.Pointer(&okv)))
	return boolResult("Map.Has", r1, uintptr(okv))
}

// Delete removes key; ok reports whether it was present.
func (m *Map) Delete(s *Scope, c *Context, key Value) (bool, error) {
	sh, err := m.mapCallArgs(s, c, &key, nil)
	if err != nil {
		return false, err
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_map_delete").Call(
		m.iso.handle, c.handle, sh, m.h, key.h, uintptr(unsafe.Pointer(&okv)))
	return boolResult("Map.Delete", r1, uintptr(okv))
}

// Size returns the number of entries.
func (m *Map) Size() (int64, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	ih, err := m.iso.handleChecked()
	if err != nil {
		return 0, err
	}
	var out int64
	r1, _, _ := proc("gov8_rv_map_size").Call(ih, m.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("Map.Size", r1)
	}
	return out, nil
}

// Clear removes every entry.
func (m *Map) Clear() error {
	if err := m.check(); err != nil {
		return err
	}
	ih, err := m.iso.handleChecked()
	if err != nil {
		return err
	}
	r1, _, _ := proc("gov8_rv_map_clear").Call(ih, m.h)
	if int64(r1) < 0 {
		return shimError("Map.Clear", r1)
	}
	return nil
}

// AsArray renders the map as [[k0, v0], [k1, v1], ...] in insertion order.
func (m *Map) AsArray(s *Scope, c *Context) (*Array, error) {
	if err := m.ctxCheckArg(c); err != nil {
		return nil, err
	}
	sh, err := m.scopeArg(s)
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_map_as_array").Call(
		m.iso.handle, c.handle, sh, m.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("Map.AsArray", r1)
	}
	return &Array{Value: Value{iso: m.iso, sc: s, h: out}}, nil
}

// mapCallArgs validates the receiver, scope, context and key (and optional
// value) for a Map method and returns the checked scope handle.
func (m *Map) mapCallArgs(s *Scope, c *Context, key *Value, value *Value) (uintptr, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	if err := c.check(); err != nil {
		return 0, err
	}
	if c.iso != m.iso {
		return 0, foreignIsolate("context")
	}
	sh, err := m.scopeArg(s)
	if err != nil {
		return 0, err
	}
	if err := key.check(); err != nil {
		return 0, err
	}
	if key.iso != m.iso {
		return 0, foreignIsolate("key")
	}
	if value != nil {
		if err := value.check(); err != nil {
			return 0, err
		}
		if value.iso != m.iso {
			return 0, foreignIsolate("value")
		}
	}
	return sh, nil
}

// ctxCheckArg validates the context belongs to the receiver's isolate.
func (m *Map) ctxCheckArg(c *Context) error {
	if err := m.check(); err != nil {
		return err
	}
	if err := c.check(); err != nil {
		return err
	}
	if c.iso != m.iso {
		return foreignIsolate("context")
	}
	return nil
}

// AsMap casts a value to a Map after prevalidating the engine kind.
func AsMap(v Value) (*Map, error) {
	vv, err := typedCast(v, v.IsMap, "Map")
	if err != nil {
		return nil, err
	}
	return &Map{Value: vv}, nil
}

// --- Set -----------------------------------------------------------------------

// Set is a JS Set object (SameValueZero dedup: NaN deduplicates, +0 and -0
// are the same element).
type Set struct{ Value }

// NewSet creates an empty Set. The context and scope must belong to the
// same isolate.
func (s *Scope) NewSet(c *Context) (*Set, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if c == nil || c.iso != s.iso {
		return nil, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_set_new").Call(s.iso.handle, c.handle, sh, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewSet", r1)
	}
	return &Set{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// Add inserts key and returns the collection itself (compare with Same to
// observe identity).
func (st *Set) Add(s *Scope, c *Context, key Value) (*Set, error) {
	sh, err := st.setCallArgs(s, c, key)
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_set_add").Call(
		st.iso.handle, c.handle, sh, st.h, key.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("Set.Add", r1)
	}
	return &Set{Value: Value{iso: st.iso, sc: s, h: out}}, nil
}

// Has reports membership.
func (st *Set) Has(s *Scope, c *Context, key Value) (bool, error) {
	sh, err := st.setCallArgs(s, c, key)
	if err != nil {
		return false, err
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_set_has").Call(
		st.iso.handle, c.handle, sh, st.h, key.h, uintptr(unsafe.Pointer(&okv)))
	return boolResult("Set.Has", r1, uintptr(okv))
}

// Delete removes key; ok reports whether it was present.
func (st *Set) Delete(s *Scope, c *Context, key Value) (bool, error) {
	sh, err := st.setCallArgs(s, c, key)
	if err != nil {
		return false, err
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_set_delete").Call(
		st.iso.handle, c.handle, sh, st.h, key.h, uintptr(unsafe.Pointer(&okv)))
	return boolResult("Set.Delete", r1, uintptr(okv))
}

// Size returns the number of elements.
func (st *Set) Size() (int64, error) {
	if err := st.check(); err != nil {
		return 0, err
	}
	ih, err := st.iso.handleChecked()
	if err != nil {
		return 0, err
	}
	var out int64
	r1, _, _ := proc("gov8_rv_set_size").Call(ih, st.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("Set.Size", r1)
	}
	return out, nil
}

// Clear removes every element.
func (st *Set) Clear() error {
	if err := st.check(); err != nil {
		return err
	}
	ih, err := st.iso.handleChecked()
	if err != nil {
		return err
	}
	r1, _, _ := proc("gov8_rv_set_clear").Call(ih, st.h)
	if int64(r1) < 0 {
		return shimError("Set.Clear", r1)
	}
	return nil
}

// AsArray renders the set's elements in insertion order.
func (st *Set) AsArray(s *Scope, c *Context) (*Array, error) {
	if err := st.ctxCheckArg(c); err != nil {
		return nil, err
	}
	sh, err := st.scopeArg(s)
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_set_as_array").Call(
		st.iso.handle, c.handle, sh, st.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("Set.AsArray", r1)
	}
	return &Array{Value: Value{iso: st.iso, sc: s, h: out}}, nil
}

func (st *Set) setCallArgs(s *Scope, c *Context, key Value) (uintptr, error) {
	if err := st.check(); err != nil {
		return 0, err
	}
	if err := c.check(); err != nil {
		return 0, err
	}
	if c.iso != st.iso {
		return 0, foreignIsolate("context")
	}
	sh, err := st.scopeArg(s)
	if err != nil {
		return 0, err
	}
	if err := key.check(); err != nil {
		return 0, err
	}
	if key.iso != st.iso {
		return 0, foreignIsolate("key")
	}
	return sh, nil
}

// ctxCheckArg validates the context belongs to the receiver's isolate.
func (st *Set) ctxCheckArg(c *Context) error {
	if err := st.check(); err != nil {
		return err
	}
	if err := c.check(); err != nil {
		return err
	}
	if c.iso != st.iso {
		return foreignIsolate("context")
	}
	return nil
}

// AsSet casts a value to a Set after prevalidating the engine kind.
func AsSet(v Value) (*Set, error) {
	vv, err := typedCast(v, v.IsSet, "Set")
	if err != nil {
		return nil, err
	}
	return &Set{Value: vv}, nil
}

// --- Proxy -----------------------------------------------------------------------

// Proxy is a JS Proxy exotic object.
type Proxy struct{ Value }

// NewProxy creates a proxy over target with the given handler.
func (s *Scope) NewProxy(c *Context, target, handler *Object) (*Proxy, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if c == nil || c.iso != s.iso {
		return nil, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	if target == nil || handler == nil {
		return nil, fmt.Errorf("gov8: proxy target and handler are required")
	}
	if target.iso != s.iso || handler.iso != s.iso {
		return nil, foreignIsolate("object")
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_proxy_new").Call(
		s.iso.handle, c.handle, s.handle, target.h, handler.h,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewProxy", r1)
	}
	return &Proxy{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// GetTarget returns the proxy target. After Revoke the engine clears the
// internal target, so this resolves to the JavaScript null value.
func (p *Proxy) GetTarget(s *Scope) (Value, error) {
	sh, err := p.scopeArg(s)
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_proxy_get_target").Call(
		p.iso.handle, sh, p.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("Proxy.GetTarget", r1)
	}
	return Value{iso: p.iso, sc: s, h: out}, nil
}

// GetHandler returns the proxy handler object.
func (p *Proxy) GetHandler(s *Scope) (Value, error) {
	sh, err := p.scopeArg(s)
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_proxy_get_handler").Call(
		p.iso.handle, sh, p.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("Proxy.GetHandler", r1)
	}
	return Value{iso: p.iso, sc: s, h: out}, nil
}

// IsRevoked reports whether the proxy has been revoked.
func (p *Proxy) IsRevoked() (bool, error) {
	if err := p.check(); err != nil {
		return false, err
	}
	ih, err := p.iso.handleChecked()
	if err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_rv_proxy_is_revoked").Call(ih, p.h)
	if int64(r1) < 0 {
		return false, shimError("Proxy.IsRevoked", r1)
	}
	return r1 == 1, nil
}

// Revoke revokes the proxy. Property operations on it throw afterwards
// (observable natively through failed property calls and any active
// TryCatch).
func (p *Proxy) Revoke() error {
	if err := p.check(); err != nil {
		return err
	}
	ih, err := p.iso.handleChecked()
	if err != nil {
		return err
	}
	r1, _, _ := proc("gov8_rv_proxy_revoke").Call(ih, p.h)
	if int64(r1) < 0 {
		return shimError("Proxy.Revoke", r1)
	}
	return nil
}

// AsProxy casts a value to a Proxy after prevalidating the engine kind.
func AsProxy(v Value) (*Proxy, error) {
	vv, err := typedCast(v, v.IsProxy, "Proxy")
	if err != nil {
		return nil, err
	}
	return &Proxy{Value: vv}, nil
}

// --- Symbol -----------------------------------------------------------------------

// Symbol is a JS symbol primitive.
type Symbol struct{ Value }

// NewSymbol creates a fresh symbol. A zero Value description creates an
// anonymous symbol; otherwise the description must be a JS string.
func (s *Scope) NewSymbol(description Value) (*Symbol, error) {
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, err
	}
	present, err := description.optionalString(s)
	if err != nil {
		return nil, err
	}
	var dh uintptr
	if present {
		dh = description.h
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_symbol_new").Call(
		s.iso.handle, sh, dh, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewSymbol", r1)
	}
	return &Symbol{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// SymbolForKey returns the symbol registered in the global (JS-visible)
// registry for description — the Symbol.for equivalent. description must be
// a JS string.
func (s *Scope) SymbolForKey(description Value) (*Symbol, error) {
	return s.symbolFor(description, "gov8_rv_symbol_for", "SymbolForKey")
}

// SymbolForApi returns the symbol registered in the embedder-only registry
// for description (a separate registry from Symbol.for). description must
// be a JS string.
func (s *Scope) SymbolForApi(description Value) (*Symbol, error) {
	return s.symbolFor(description, "gov8_rv_symbol_for_api", "SymbolForApi")
}

func (s *Scope) symbolFor(description Value, op, export string) (*Symbol, error) {
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, err
	}
	if err := description.requireString(); err != nil {
		return nil, err
	}
	if description.iso != s.iso {
		return nil, foreignIsolate("description")
	}
	var out uintptr
	r1, _, _ := proc(op).Call(s.iso.handle, sh, description.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError(export, r1)
	}
	return &Symbol{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// GetToStringTagSymbol returns the well-known Symbol.toStringTag.
func (s *Scope) GetToStringTagSymbol() (*Symbol, error) {
	return s.wellKnown("gov8_rv_symbol_get_to_string_tag", "GetToStringTagSymbol")
}

// GetIteratorSymbol returns the well-known Symbol.iterator.
func (s *Scope) GetIteratorSymbol() (*Symbol, error) {
	return s.wellKnown("gov8_rv_symbol_get_iterator", "GetIteratorSymbol")
}

// GetHasInstanceSymbol returns the well-known Symbol.hasInstance.
func (s *Scope) GetHasInstanceSymbol() (*Symbol, error) {
	return s.wellKnown("gov8_rv_symbol_get_has_instance", "GetHasInstanceSymbol")
}

// GetAsyncIteratorSymbol returns the well-known Symbol.asyncIterator.
func (s *Scope) GetAsyncIteratorSymbol() (*Symbol, error) {
	return s.wellKnown("gov8_rvr_symbol_get_async_iterator", "GetAsyncIteratorSymbol")
}

// GetIsConcatSpreadableSymbol returns the well-known Symbol.isConcatSpreadable.
func (s *Scope) GetIsConcatSpreadableSymbol() (*Symbol, error) {
	return s.wellKnown("gov8_rvr_symbol_get_is_concat_spreadable", "GetIsConcatSpreadableSymbol")
}

// GetMatchSymbol returns the well-known Symbol.match.
func (s *Scope) GetMatchSymbol() (*Symbol, error) {
	return s.wellKnown("gov8_rvr_symbol_get_match", "GetMatchSymbol")
}

// GetReplaceSymbol returns the well-known Symbol.replace.
func (s *Scope) GetReplaceSymbol() (*Symbol, error) {
	return s.wellKnown("gov8_rvr_symbol_get_replace", "GetReplaceSymbol")
}

// GetSearchSymbol returns the well-known Symbol.search.
func (s *Scope) GetSearchSymbol() (*Symbol, error) {
	return s.wellKnown("gov8_rvr_symbol_get_search", "GetSearchSymbol")
}

// GetSplitSymbol returns the well-known Symbol.split.
func (s *Scope) GetSplitSymbol() (*Symbol, error) {
	return s.wellKnown("gov8_rvr_symbol_get_split", "GetSplitSymbol")
}

// GetToPrimitiveSymbol returns the well-known Symbol.toPrimitive.
func (s *Scope) GetToPrimitiveSymbol() (*Symbol, error) {
	return s.wellKnown("gov8_rvr_symbol_get_to_primitive", "GetToPrimitiveSymbol")
}

// GetUnscopablesSymbol returns the well-known Symbol.unscopables.
func (s *Scope) GetUnscopablesSymbol() (*Symbol, error) {
	return s.wellKnown("gov8_rvr_symbol_get_unscopables", "GetUnscopablesSymbol")
}

func (s *Scope) wellKnown(op, export string) (*Symbol, error) {
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc(op).Call(s.iso.handle, sh, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError(export, r1)
	}
	return &Symbol{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// Description returns the symbol description (the undefined value for an
// anonymous symbol).
func (sym *Symbol) Description(s *Scope) (Value, error) {
	sh, err := sym.scopeArg(s)
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_symbol_description").Call(
		sym.iso.handle, sh, sym.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("Symbol.Description", r1)
	}
	return Value{iso: sym.iso, sc: s, h: out}, nil
}

// AsSymbol casts a value to a Symbol after prevalidating the engine kind.
func AsSymbol(v Value) (*Symbol, error) {
	vv, err := typedCast(v, v.IsSymbol, "Symbol")
	if err != nil {
		return nil, err
	}
	return &Symbol{Value: vv}, nil
}

// --- Private -----------------------------------------------------------------------

// Private is a private symbol (v8::Private). It is a Data, not a Value:
// the embedded Value exists only for handle plumbing and must not be used
// with value predicates. Private properties are completely invisible to JS
// property machinery.
type Private struct{ Value }

// NewPrivate creates a fresh private symbol. A zero Value name creates an
// anonymous private; otherwise the name must be a JS string.
func (s *Scope) NewPrivate(name Value) (*Private, error) {
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, err
	}
	present, err := name.optionalString(s)
	if err != nil {
		return nil, err
	}
	var nh uintptr
	if present {
		nh = name.h
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_private_new").Call(
		s.iso.handle, sh, nh, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewPrivate", r1)
	}
	return &Private{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// PrivateForApi returns the private symbol registered per isolate for name
// (Private.for_api — repeated calls with the same name return the same
// private). name must be a JS string. Although the pinned Rust signature
// accepts None, that input access-violates in this build; Go rejects a zero
// Value before FFI.
func (s *Scope) PrivateForApi(name Value) (*Private, error) {
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, err
	}
	if err := name.requireString(); err != nil {
		return nil, err
	}
	if name.iso != s.iso {
		return nil, foreignIsolate("name")
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_private_for_api").Call(
		s.iso.handle, sh, name.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("PrivateForApi", r1)
	}
	return &Private{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// Name returns the private's name value (the undefined value for an
// anonymous private).
func (p *Private) Name(s *Scope) (Value, error) {
	sh, err := p.scopeArg(s)
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_private_name").Call(
		p.iso.handle, sh, p.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("Private.Name", r1)
	}
	return Value{iso: p.iso, sc: s, h: out}, nil
}

// --- Object property surface ---------------------------------------------------------

// NewObject creates a fresh plain JS object (v8::Object::new). The context
// and scope must belong to the same isolate.
func (s *Scope) NewObject(c *Context) (*Object, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if c == nil || c.iso != s.iso {
		return nil, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, err
	}
	out, err := callHandle("NewObject", proc("gov8_rv_object_new"), s.iso.handle, c.handle, sh)
	if err != nil {
		return nil, err
	}
	return &Object{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// GetByKey reads the property held under an arbitrary key value (string or
// symbol). A missing key reads as the undefined value; an error means the
// getter threw.
func (o *Object) GetByKey(s *Scope, c *Context, key Value) (Value, error) {
	if err := o.ctxHandle(c); err != nil {
		return Value{}, err
	}
	sh, err := o.scopeArg(s)
	if err != nil {
		return Value{}, err
	}
	if err := key.check(); err != nil {
		return Value{}, err
	}
	if key.iso != o.iso {
		return Value{}, foreignIsolate("key")
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_object_get").Call(
		o.iso.handle, c.handle, sh, o.h, key.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("GetByKey", r1)
	}
	return Value{iso: o.iso, sc: s, h: out}, nil
}

// SetByKey writes the property held under an arbitrary key value. ok is
// Just(false) when the write was ignored (e.g. a non-writable inherited
// property in non-strict mode); an error means the setter threw.
func (o *Object) SetByKey(s *Scope, c *Context, key, value Value) (bool, error) {
	if err := o.ctxHandle(c); err != nil {
		return false, err
	}
	sh, err := o.scopeArg(s)
	if err != nil {
		return false, err
	}
	if err := key.check(); err != nil {
		return false, err
	}
	if key.iso != o.iso {
		return false, foreignIsolate("key")
	}
	if err := value.check(); err != nil {
		return false, err
	}
	if value.iso != o.iso {
		return false, foreignIsolate("value")
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_object_set").Call(
		o.iso.handle, c.handle, sh, o.h, key.h, value.h,
		uintptr(unsafe.Pointer(&okv)))
	return boolResult("SetByKey", r1, uintptr(okv))
}

// CreateDataProperty creates (or redefines) key as a plain enumerable,
// writable, configurable data property. key must be a Name (string or
// symbol).
func (o *Object) CreateDataProperty(s *Scope, c *Context, key, value Value) (bool, error) {
	if err := o.ctxHandle(c); err != nil {
		return false, err
	}
	sh, err := o.scopeArg(s)
	if err != nil {
		return false, err
	}
	if err := key.requireName(); err != nil {
		return false, err
	}
	if key.iso != o.iso {
		return false, foreignIsolate("key")
	}
	if err := value.check(); err != nil {
		return false, err
	}
	if value.iso != o.iso {
		return false, foreignIsolate("value")
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_object_create_data_property").Call(
		o.iso.handle, c.handle, sh, o.h, key.h, value.h,
		uintptr(unsafe.Pointer(&okv)))
	return boolResult("CreateDataProperty", r1, uintptr(okv))
}

// DefineOwnProperty defines key with value and the given attribute bits
// (like Object.defineProperty with a partial data descriptor).
func (o *Object) DefineOwnProperty(s *Scope, c *Context, key, value Value, attr PropertyAttribute) (bool, error) {
	if err := o.ctxHandle(c); err != nil {
		return false, err
	}
	sh, err := o.scopeArg(s)
	if err != nil {
		return false, err
	}
	if err := key.requireName(); err != nil {
		return false, err
	}
	if key.iso != o.iso {
		return false, foreignIsolate("key")
	}
	if err := value.check(); err != nil {
		return false, err
	}
	if value.iso != o.iso {
		return false, foreignIsolate("value")
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_object_define_own_property").Call(
		o.iso.handle, c.handle, sh, o.h, key.h, value.h, uintptr(uint8(attr)),
		uintptr(unsafe.Pointer(&okv)))
	return boolResult("DefineOwnProperty", r1, uintptr(okv))
}

// GetPropertyAttributes returns the PropertyAttribute bits of key. The
// second result mirrors the engine's Maybe: a MISSING property is Just(NONE)
// (present=true, attr=PropertyAttributeNone) — the pinned nuance; present is
// only false when the call threw (err is non-nil in that case).
func (o *Object) GetPropertyAttributes(s *Scope, c *Context, key Value) (attr PropertyAttribute, present bool, err error) {
	if err = o.ctxHandle(c); err != nil {
		return 0, false, err
	}
	sh, err := o.scopeArg(s)
	if err != nil {
		return 0, false, err
	}
	if err = key.check(); err != nil {
		return 0, false, err
	}
	if key.iso != o.iso {
		return 0, false, foreignIsolate("key")
	}
	var raw, isJust int32
	r1, _, _ := proc("gov8_rv_object_get_property_attributes").Call(
		o.iso.handle, c.handle, sh, o.h, key.h,
		uintptr(unsafe.Pointer(&raw)), uintptr(unsafe.Pointer(&isJust)))
	if int64(r1) < 0 {
		return 0, false, shimError("GetPropertyAttributes", r1)
	}
	return PropertyAttribute(uint8(raw)), isJust == 1, nil
}

// SetIntegrityLevel seals (no deletions, no additions) or freezes (additionally
// read-only existing data properties) the object.
func (o *Object) SetIntegrityLevel(s *Scope, c *Context, level IntegrityLevel) (bool, error) {
	if err := o.ctxHandle(c); err != nil {
		return false, err
	}
	sh, err := o.scopeArg(s)
	if err != nil {
		return false, err
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_object_set_integrity_level").Call(
		o.iso.handle, c.handle, sh, o.h, uintptr(uint8(level)),
		uintptr(unsafe.Pointer(&okv)))
	return boolResult("SetIntegrityLevel", r1, uintptr(okv))
}

// DefineProperty defines key according to the descriptor (the general
// Object.defineProperty mechanism). key must be a Name.
func (o *Object) DefineProperty(s *Scope, c *Context, key Value, desc *PropertyDescriptor) (bool, error) {
	if err := o.ctxHandle(c); err != nil {
		return false, err
	}
	sh, err := o.scopeArg(s)
	if err != nil {
		return false, err
	}
	if err := key.requireName(); err != nil {
		return false, err
	}
	if key.iso != o.iso {
		return false, foreignIsolate("key")
	}
	if desc == nil {
		return false, fmt.Errorf("gov8: descriptor is required")
	}
	// The descriptor's stored locals live in its creating scope; that scope
	// must still be open (checked inside desc.check()).
	if desc.iso != o.iso {
		return false, foreignIsolate("descriptor")
	}
	if err := desc.check(); err != nil {
		return false, err
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_object_define_property").Call(
		o.iso.handle, c.handle, sh, o.h, key.h, desc.handle,
		uintptr(unsafe.Pointer(&okv)))
	return boolResult("DefineProperty", r1, uintptr(okv))
}

// GetOwnPropertyDescriptor returns the property's descriptor object
// (JSON/stringify-able, mirroring Object.getOwnPropertyDescriptor). A
// missing key reads as the undefined VALUE (the pinned nuance); an error
// means the call threw.
func (o *Object) GetOwnPropertyDescriptor(s *Scope, c *Context, key Value) (Value, error) {
	if err := o.ctxHandle(c); err != nil {
		return Value{}, err
	}
	sh, err := o.scopeArg(s)
	if err != nil {
		return Value{}, err
	}
	if err := key.requireName(); err != nil {
		return Value{}, err
	}
	if key.iso != o.iso {
		return Value{}, foreignIsolate("key")
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_object_get_own_property_descriptor").Call(
		o.iso.handle, c.handle, sh, o.h, key.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("GetOwnPropertyDescriptor", r1)
	}
	return Value{iso: o.iso, sc: s, h: out}, nil
}

// GetPropertyNames collects the object's property names according to the
// four-way filter (collection mode, property filter, index filter and key
// conversion), mirroring the crate's GetPropertyNamesArgs.
func (o *Object) GetPropertyNames(s *Scope, c *Context, mode KeyCollectionMode, propertyFilter PropertyFilter, indexFilter IndexFilter, conversion KeyConversionMode) (*Array, error) {
	if err := o.ctxHandle(c); err != nil {
		return nil, err
	}
	sh, err := o.scopeArg(s)
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_object_get_property_names").Call(
		o.iso.handle, c.handle, sh, o.h, uintptr(uint8(mode)),
		uintptr(uint8(propertyFilter)), uintptr(uint8(indexFilter)),
		uintptr(uint8(conversion)), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("GetPropertyNames", r1)
	}
	return &Array{Value: Value{iso: o.iso, sc: s, h: out}}, nil
}

// SetPrivate stores value under the private key (invisible to JS).
func (o *Object) SetPrivate(s *Scope, c *Context, key *Private, value Value) (bool, error) {
	sh, err := o.privateCallArgs(s, c, key)
	if err != nil {
		return false, err
	}
	if err := value.check(); err != nil {
		return false, err
	}
	if value.iso != o.iso {
		return false, foreignIsolate("value")
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_object_set_private").Call(
		o.iso.handle, c.handle, sh, o.h, key.h, value.h,
		uintptr(unsafe.Pointer(&okv)))
	return boolResult("SetPrivate", r1, uintptr(okv))
}

// GetPrivate reads the value stored under the private key (the undefined
// value when absent).
func (o *Object) GetPrivate(s *Scope, c *Context, key *Private) (Value, error) {
	sh, err := o.privateCallArgs(s, c, key)
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_rv_object_get_private").Call(
		o.iso.handle, c.handle, sh, o.h, key.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("GetPrivate", r1)
	}
	return Value{iso: o.iso, sc: s, h: out}, nil
}

// HasPrivate reports whether the private key is present.
func (o *Object) HasPrivate(s *Scope, c *Context, key *Private) (bool, error) {
	sh, err := o.privateCallArgs(s, c, key)
	if err != nil {
		return false, err
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_object_has_private").Call(
		o.iso.handle, c.handle, sh, o.h, key.h, uintptr(unsafe.Pointer(&okv)))
	return boolResult("HasPrivate", r1, uintptr(okv))
}

// DeletePrivate removes the private key; ok reports whether it was present.
func (o *Object) DeletePrivate(s *Scope, c *Context, key *Private) (bool, error) {
	sh, err := o.privateCallArgs(s, c, key)
	if err != nil {
		return false, err
	}
	var okv int32
	r1, _, _ := proc("gov8_rv_object_delete_private").Call(
		o.iso.handle, c.handle, sh, o.h, key.h, uintptr(unsafe.Pointer(&okv)))
	return boolResult("DeletePrivate", r1, uintptr(okv))
}

func (o *Object) privateCallArgs(s *Scope, c *Context, key *Private) (uintptr, error) {
	if err := o.ctxHandle(c); err != nil {
		return 0, err
	}
	sh, err := o.scopeArg(s)
	if err != nil {
		return 0, err
	}
	if key == nil {
		return 0, fmt.Errorf("gov8: private key is required")
	}
	if err := key.check(); err != nil {
		return 0, err
	}
	if key.iso != o.iso {
		return 0, foreignIsolate("key")
	}
	return sh, nil
}

// --- property attribute / integrity / name-filter types -------------------------

// (PropertyAttribute and its Attr* constants live in template.go; this
// slice reuses them for DefineOwnProperty and GetPropertyAttributes.)

// IntegrityLevel mirrors v8::IntegrityLevel (kFrozen = 0, kSealed = 1 in the
// engine's encoding).
type IntegrityLevel uint8

const (
	IntegrityFrozen IntegrityLevel = 0
	IntegritySealed IntegrityLevel = 1
)

// KeyCollectionMode mirrors v8::KeyCollectionMode.
type KeyCollectionMode uint8

const (
	KeyCollectionOwnOnly           KeyCollectionMode = 0
	KeyCollectionIncludePrototypes KeyCollectionMode = 1
)

// PropertyFilter mirrors v8::PropertyFilter (a bitmask).
type PropertyFilter uint8

const (
	PropertyFilterAllProperties    PropertyFilter = 0
	PropertyFilterOnlyWritable     PropertyFilter = 1 << 0
	PropertyFilterOnlyEnumerable   PropertyFilter = 1 << 1
	PropertyFilterOnlyConfigurable PropertyFilter = 1 << 2
	PropertyFilterSkipStrings      PropertyFilter = 1 << 3
	PropertyFilterSkipSymbols      PropertyFilter = 1 << 4
)

// IndexFilter mirrors v8::IndexFilter.
type IndexFilter uint8

const (
	IndexFilterIncludeIndices IndexFilter = 0
	IndexFilterSkipIndices    IndexFilter = 1
)

// KeyConversionMode mirrors v8::KeyConversionMode.
type KeyConversionMode uint8

const (
	KeyConversionConvertToString KeyConversionMode = 0
	KeyConversionKeepNumbers     KeyConversionMode = 1
	KeyConversionNoNumbers       KeyConversionMode = 2
)

// --- PropertyDescriptor ------------------------------------------------------------

// PropertyDescriptor mirrors v8::PropertyDescriptor: a partial property
// description with presence flags for each field. It holds scope-local
// handle slots, so it is bound to the Scope it was created in: Close it
// before that Scope closes and do not use it afterwards (every accessor
// enforces both). It must only be used on the owning isolate thread.
type PropertyDescriptor struct {
	iso    *Isolate
	sc     *Scope
	handle uintptr
	closed bool
}

func (pd *PropertyDescriptor) check() error {
	if pd == nil {
		return fmt.Errorf("gov8: nil descriptor")
	}
	if pd.sc != nil {
		if err := pd.sc.check(); err != nil {
			return err
		}
	}
	if pd.closed {
		return fmt.Errorf("gov8: descriptor used after Close")
	}
	return nil
}

// NewPropertyDescriptor creates the default (empty) descriptor.
func (s *Scope) NewPropertyDescriptor() (*PropertyDescriptor, error) {
	return s.newPD(func(isoHandle uintptr) (uintptr, error) {
		return callHandle("NewPropertyDescriptor", proc("gov8_rv_pd_new"), isoHandle)
	})
}

// NewPropertyDescriptorFromValue creates a data descriptor with only the
// value present.
func (s *Scope) NewPropertyDescriptorFromValue(value Value) (*PropertyDescriptor, error) {
	if err := value.check(); err != nil {
		return nil, err
	}
	if value.iso != s.iso {
		return nil, foreignIsolate("value")
	}
	return s.newPD(func(isoHandle uintptr) (uintptr, error) {
		return callHandle("NewPropertyDescriptorFromValue",
			proc("gov8_rv_pd_new_value"), isoHandle, value.h)
	})
}

// NewPropertyDescriptorFromValueWritable creates a data descriptor with the
// value present and the writable flag specified.
func (s *Scope) NewPropertyDescriptorFromValueWritable(value Value, writable bool) (*PropertyDescriptor, error) {
	if err := value.check(); err != nil {
		return nil, err
	}
	if value.iso != s.iso {
		return nil, foreignIsolate("value")
	}
	w := uintptr(0)
	if writable {
		w = 1
	}
	return s.newPD(func(isoHandle uintptr) (uintptr, error) {
		return callHandle("NewPropertyDescriptorFromValueWritable",
			proc("gov8_rv_pd_new_value_writable"), isoHandle, value.h, w)
	})
}

// NewPropertyDescriptorFromGetSet creates an accessor descriptor from the
// given getter and setter (both must be callable values).
func (s *Scope) NewPropertyDescriptorFromGetSet(get, set Value) (*PropertyDescriptor, error) {
	if err := get.check(); err != nil {
		return nil, err
	}
	if get.iso != s.iso {
		return nil, foreignIsolate("getter")
	}
	if err := set.check(); err != nil {
		return nil, err
	}
	if set.iso != s.iso {
		return nil, foreignIsolate("setter")
	}
	return s.newPD(func(isoHandle uintptr) (uintptr, error) {
		return callHandle("NewPropertyDescriptorFromGetSet",
			proc("gov8_rv_pd_new_get_set"), isoHandle, get.h, set.h)
	})
}

func (s *Scope) newPD(newFn func(isoHandle uintptr) (uintptr, error)) (*PropertyDescriptor, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	ih, err := s.iso.handleChecked()
	if err != nil {
		return nil, err
	}
	h, err := newFn(ih)
	if err != nil {
		return nil, err
	}
	return &PropertyDescriptor{iso: s.iso, sc: s, handle: h}, nil
}

// Close releases the descriptor. It must not be used afterwards.
func (pd *PropertyDescriptor) Close() error {
	if pd == nil {
		return fmt.Errorf("gov8: nil descriptor")
	}
	if pd.closed {
		return fmt.Errorf("gov8: descriptor already closed")
	}
	if pd.sc != nil {
		if err := pd.sc.check(); err != nil {
			return err
		}
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	r1, _, _ := proc("gov8_rv_pd_dispose").Call(pd.handle)
	pd.closed = true
	if int64(r1) < 0 {
		return shimError("PropertyDescriptor.Close", r1)
	}
	return nil
}

// HasValue reports whether a data value is present.
func (pd *PropertyDescriptor) HasValue() (bool, error) {
	return pd.boolFlag("gov8_rv_pd_has_value", "HasValue")
}

// HasWritable reports whether the writable flag is specified.
func (pd *PropertyDescriptor) HasWritable() (bool, error) {
	return pd.boolFlag("gov8_rv_pd_has_writable", "HasWritable")
}

// HasEnumerable reports whether the enumerable flag is specified.
func (pd *PropertyDescriptor) HasEnumerable() (bool, error) {
	return pd.boolFlag("gov8_rv_pd_has_enumerable", "HasEnumerable")
}

// HasConfigurable reports whether the configurable flag is specified.
func (pd *PropertyDescriptor) HasConfigurable() (bool, error) {
	return pd.boolFlag("gov8_rv_pd_has_configurable", "HasConfigurable")
}

// HasGet reports whether a getter is present.
func (pd *PropertyDescriptor) HasGet() (bool, error) {
	return pd.boolFlag("gov8_rv_pd_has_get", "HasGet")
}

// HasSet reports whether a setter is present.
func (pd *PropertyDescriptor) HasSet() (bool, error) {
	return pd.boolFlag("gov8_rv_pd_has_set", "HasSet")
}

// Writable returns the writable flag (meaningful only when HasWritable).
func (pd *PropertyDescriptor) Writable() (bool, error) {
	return pd.boolFlag("gov8_rv_pd_writable", "Writable")
}

// Enumerable returns the enumerable flag (meaningful only when
// HasEnumerable).
func (pd *PropertyDescriptor) Enumerable() (bool, error) {
	return pd.boolFlag("gov8_rv_pd_enumerable", "Enumerable")
}

// Configurable returns the configurable flag (meaningful only when
// HasConfigurable).
func (pd *PropertyDescriptor) Configurable() (bool, error) {
	return pd.boolFlag("gov8_rv_pd_configurable", "Configurable")
}

func (pd *PropertyDescriptor) boolFlag(op, export string) (bool, error) {
	if err := pd.check(); err != nil {
		return false, err
	}
	r1, _, _ := proc(op).Call(pd.handle)
	if int64(r1) < 0 {
		return false, shimError("PropertyDescriptor."+export, r1)
	}
	return r1 == 1, nil
}

// Value returns the data value (valid only while the creating scope is
// open; call HasValue first).
func (pd *PropertyDescriptor) Value() (Value, error) {
	return pd.valueField("gov8_rv_pd_value", "Value")
}

// Get returns the getter (valid only while the creating scope is open).
func (pd *PropertyDescriptor) Get() (Value, error) {
	return pd.valueField("gov8_rv_pd_get", "Get")
}

// Set returns the setter (valid only while the creating scope is open).
func (pd *PropertyDescriptor) Set() (Value, error) {
	return pd.valueField("gov8_rv_pd_set", "Set")
}

func (pd *PropertyDescriptor) valueField(op, export string) (Value, error) {
	if err := pd.check(); err != nil {
		return Value{}, err
	}
	sh, err := pd.sc.checkedHandle()
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc(op).Call(pd.iso.handle, sh, pd.handle, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("PropertyDescriptor."+export, r1)
	}
	return Value{iso: pd.iso, sc: pd.sc, h: out}, nil
}

// SetEnumerable specifies the enumerable flag in place.
func (pd *PropertyDescriptor) SetEnumerable(enumerable bool) error {
	return pd.setFlag("gov8_rv_pd_set_enumerable", "SetEnumerable", enumerable)
}

// SetConfigurable specifies the configurable flag in place.
func (pd *PropertyDescriptor) SetConfigurable(configurable bool) error {
	return pd.setFlag("gov8_rv_pd_set_configurable", "SetConfigurable", configurable)
}

func (pd *PropertyDescriptor) setFlag(op, export string, v bool) error {
	if err := pd.check(); err != nil {
		return err
	}
	arg := uintptr(0)
	if v {
		arg = 1
	}
	r1, _, _ := proc(op).Call(pd.handle, arg)
	if int64(r1) < 0 {
		return shimError("PropertyDescriptor."+export, r1)
	}
	return nil
}
