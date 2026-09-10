//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// Core-advanced embedding surface, mirroring the pinned crate observably:
//
//   - EscapableHandleScope -> Scope.NewEscapableScope with Escape/Close. The
//     pinned "escape() called twice" guard is a Go error carrying the exact
//     pinned message text; the C++ release-mode Escape would silently
//     overwrite the escape slot, so the guard never lets a second escape
//     reach the engine.
//   - ContextScope enter/exit (Context::Enter/Exit under the crate's scope
//     wrappers), current/entered context observation with engine identity
//     comparison, SameValue, embedder data (values and aligned pointers,
//     with the pinned crate's two-slot internal offset) and host-side
//     context slots (the crate's Rc slots, keyed explicitly in Go).
//   - Raw isolate data slots with a Go-side bound check (upstream
//     Set/GetData with an out-of-range slot is an unchecked OOB).
//   - ScriptOrigin compiles (including the module flag whose classic-compile
//     rejection is engine-fatal upstream and characterized in subprocesses),
//     the ScriptCompiler surface (unbound compiles, CompileFunction, code
//     caches with a graceful CompatibilityCheck prevalidation before
//     ConsumeCodeCache), UnboundScript rebinding and cache production.
//   - GC prologue/epilogue callbacks with a GCType filter (integer registry,
//     no Go pointers cross the boundary), heap statistics, external-memory
//     accounting and heap-space count.
//
// Intentionally unsupported: v8::SealHandleScope. The pinned crate has no
// binding for it and nothing observable can be pinned; a Go API would
// invent behavior. Tracked as explicit unsupported (not a silent stub).
//
// THREAD-AFFINITY: everything here follows the module's isolate-affinity
// model except the shared-isolate surface in locker.go, which re-pins the
// affinity to the locking thread.

// ---------------------------------------------------------------------------
// Escapable handle scopes
// ---------------------------------------------------------------------------

// EscapableScope is a v8 EscapableHandleScope: one value can be "escaped"
// out of it into the scope it was created under, surviving this scope's
// closure. Escape may be called exactly once (the pinned crate's guard).
type EscapableScope struct {
	iso     *Isolate
	outer   *Scope
	handle  uintptr
	escaped bool
	closed  bool
}

// NewEscapableScope opens an escapable handle scope under s. Exactly one
// value can be escaped into s.
func (s *Scope) NewEscapableScope() (*EscapableScope, error) {
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, err
	}
	if !s.borrowed && !currentHandleScope(s.iso, s) {
		// The Go API names the ordinary destination Scope even for a chain
		// of nested EscapableScopes. V8 opens the new escapable scope under
		// the current escapable scope and Escape is then repeated outward.
		current, nested := currentHandleScopeToken(s.iso).(*EscapableScope)
		if !nested || current.outer != s {
			return nil, errors.New("gov8: scope is not the current escapable-scope destination")
		}
	}
	h, err := callHandle("EscapableScope.New", proc("gov8_ca_esc_scope_enter"), s.iso.handleAssumingCheck(), sh)
	if err != nil {
		return nil, err
	}
	escapable := &EscapableScope{iso: s.iso, outer: s, handle: h}
	pushHandleScope(s.iso, escapable)
	return escapable, nil
}

// check validates the escapable scope's own state after its outer scope.
func (e *EscapableScope) check() error {
	if err := e.outer.check(); err != nil {
		return err
	}
	if e.closed {
		return errors.New("gov8: escapable scope used after Close")
	}
	return nil
}

// Escape pushes v into the outer scope and returns the escaped handle bound
// to the outer scope (valid after this escapable scope closes). A second
// escape on the same scope returns the pinned crate's error verbatim; the
// engine is not touched (its release-mode Escape would silently corrupt the
// first escaped value by overwriting the escape slot).
func (e *EscapableScope) Escape(v Value) (Value, error) {
	if err := e.check(); err != nil {
		return Value{}, err
	}
	if e.escaped {
		return Value{}, errors.New("EscapableHandleScope::escape() called twice")
	}
	if err := v.check(); err != nil {
		return Value{}, err
	}
	if v.iso != e.iso {
		return Value{}, foreignIsolate("value")
	}
	sh, err := e.outer.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_ca_esc_scope_escape").Call(
		e.iso.handleAssumingCheck(), sh, e.handle, v.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("EscapableScope.Escape", r1)
	}
	e.escaped = true
	return Value{iso: e.iso, sc: e.outer, h: out}, nil
}

// Close closes the escapable scope. Values created inside it (other than
// the escaped one) become invalid.
func (e *EscapableScope) Close() error {
	if err := e.outer.check(); err != nil {
		return err
	}
	if e.closed {
		return errors.New("gov8: escapable scope already closed")
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	if !currentHandleScope(e.iso, e) {
		return errors.New("gov8: escapable scope is not the current innermost handle scope")
	}
	r1, _, _ := proc("gov8_ca_esc_scope_exit").Call(e.handle)
	e.closed = true
	if err := popHandleScope(e.iso, e); err != nil {
		return err
	}
	if int64(r1) < 0 {
		return shimError("EscapableScope.Close", r1)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Context enter/exit and observation
// ---------------------------------------------------------------------------

// ContextScope is an entered-context guard (the engine half of the crate's
// ContextScope). Contexts must be exited in reverse enter order; the Go
// stack below enforces that where the crate's type system does.
type ContextScope struct {
	iso *Isolate
	// Exactly one of ctx/ref is set. ctx is a persistent Go Context; ref is a
	// borrowed scope-local Context whose source Scope is guarded until Close.
	ctx    *Context
	ref    *ContextRef
	closed bool
}

// contextScopeStacks tracks per-isolate enter order so Close can reject
// out-of-order exits before they reach the engine.
var contextScopeStacks = struct {
	mu sync.Mutex
	m  map[*Isolate][]*ContextScope
}{m: make(map[*Isolate][]*ContextScope)}

// Enter enters the context (v8 Context::Enter). Exit with Close, obeying
// reverse-enter order across all contexts of the isolate.
func (c *Context) Enter() (*ContextScope, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	if err := callErr("Context.Enter", proc("gov8_ca_context_enter"),
		c.iso.handleAssumingCheck(), c.handle); err != nil {
		return nil, err
	}
	cs := &ContextScope{iso: c.iso, ctx: c}
	contextScopeStacks.mu.Lock()
	contextScopeStacks.m[c.iso] = append(contextScopeStacks.m[c.iso], cs)
	contextScopeStacks.mu.Unlock()
	return cs, nil
}

// Enter enters this scope-local Context. The source Scope need not be the
// current innermost HandleScope, but it must remain live until the returned
// ContextScope closes; Scope.Close enforces that borrowed lifetime.
func (r *ContextRef) Enter() (*ContextScope, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	if r.h == 0 {
		return nil, errors.New("gov8: empty context reference")
	}
	if r.sc.activeBorrowedContextScopes == ^uint32(0) {
		return nil, errors.New("gov8: too many active borrowed context scopes")
	}
	r.sc.activeBorrowedContextScopes++
	if err := callErr("ContextRef.Enter", proc("gov8_ca_context_enter_local"),
		r.iso.handleAssumingCheck(), r.sc.handle, r.h); err != nil {
		r.sc.activeBorrowedContextScopes--
		return nil, err
	}
	cs := &ContextScope{iso: r.iso, ref: r}
	contextScopeStacks.mu.Lock()
	contextScopeStacks.m[r.iso] = append(contextScopeStacks.m[r.iso], cs)
	contextScopeStacks.mu.Unlock()
	return cs, nil
}

// Close exits the context. The innermost entered context must be exited
// first (the engine's Enter/Exit pair is LIFO).
func (cs *ContextScope) Close() error {
	if cs.closed {
		return errors.New("gov8: context scope already closed")
	}
	if err := cs.iso.check(); err != nil {
		return err
	}
	if cs.ctx != nil {
		if err := cs.ctx.checkAssumingIsolate(); err != nil {
			return err
		}
	} else if cs.ref != nil {
		if err := cs.ref.check(); err != nil {
			return err
		}
	} else {
		return errors.New("gov8: invalid context scope")
	}
	contextScopeStacks.mu.Lock()
	stack := contextScopeStacks.m[cs.iso]
	if len(stack) == 0 || stack[len(stack)-1] != cs {
		contextScopeStacks.mu.Unlock()
		return errors.New("gov8: context scopes must be exited in reverse enter order")
	}
	contextScopeStacks.mu.Unlock()
	if cs.ctx != nil {
		if err := callErr("ContextScope.Close", proc("gov8_ca_context_exit"),
			cs.iso.handleAssumingCheck(), cs.ctx.handle); err != nil {
			return err
		}
	} else {
		if err := callErr("ContextScope.Close", proc("gov8_ca_context_exit_local"),
			cs.iso.handleAssumingCheck(), cs.ref.sc.handle, cs.ref.h); err != nil {
			return err
		}
	}
	contextScopeStacks.mu.Lock()
	stack = contextScopeStacks.m[cs.iso]
	contextScopeStacks.m[cs.iso] = stack[:len(stack)-1]
	contextScopeStacks.mu.Unlock()
	if cs.ref != nil {
		cs.ref.sc.activeBorrowedContextScopes--
	}
	cs.closed = true
	return nil
}

// ContextRef is an engine-observed scope-local Context. It preserves the
// Local<Context> lifetime of the pinned Rust API: the reference and values
// materialized from it are invalid once its Scope closes.
type ContextRef struct {
	iso *Isolate
	sc  *Scope
	h   uintptr
}

func (r *ContextRef) check() error {
	if r == nil || r.iso == nil || r.sc == nil {
		return errors.New("gov8: nil context reference")
	}
	if err := r.iso.check(); err != nil {
		return err
	}
	if r.sc.iso != r.iso {
		return foreignIsolate("context reference scope")
	}
	_, err := r.sc.checkedHandleAssumingIsolate()
	return err
}

// IsEmpty reports whether V8 returned no current/entered Context. The pinned
// Rust getters cannot safely represent this native state, while Go keeps it as
// an explicit, non-dereferenceable result.
func (r *ContextRef) IsEmpty() (bool, error) {
	if err := r.check(); err != nil {
		return false, err
	}
	return r.h == 0, nil
}

// GlobalObject returns this Context's global object in s. The reference's
// source Scope and the result Scope must both still be live and belong to the
// same isolate.
func (r *ContextRef) GlobalObject(s *Scope) (*Object, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	if r.h == 0 {
		return nil, errors.New("gov8: empty context reference")
	}
	if s == nil || s.iso != r.iso {
		return nil, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	if err := s.requireCurrent(); err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_ca_context_global_local").Call(
		r.iso.handleAssumingCheck(), sh, r.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("ContextRef.GlobalObject", r1)
	}
	if out == 0 {
		return nil, errors.New("gov8: ContextRef.GlobalObject returned an empty value")
	}
	return &Object{Value{iso: r.iso, sc: s, h: out}}, nil
}

// CurrentContext observes the isolate's current context (the innermost
// entered one) as a scope-local reference.
func (i *Isolate) CurrentContext(s *Scope) (*ContextRef, error) {
	if err := i.check(); err != nil {
		return nil, err
	}
	if s == nil || s.iso != i {
		return nil, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	if err := s.requireCurrent(); err != nil {
		return nil, err
	}
	var out uintptr
	if err := callErr("CurrentContext", proc("gov8_ca_context_current"),
		i.handleAssumingCheck(), sh, uintptr(unsafe.Pointer(&out))); err != nil {
		return nil, err
	}
	return &ContextRef{iso: i, sc: s, h: out}, nil
}

// EnteredOrMicrotaskContext observes the entered or microtask context.
func (i *Isolate) EnteredOrMicrotaskContext(s *Scope) (*ContextRef, error) {
	if err := i.check(); err != nil {
		return nil, err
	}
	if s == nil || s.iso != i {
		return nil, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	if err := s.requireCurrent(); err != nil {
		return nil, err
	}
	var out uintptr
	if err := callErr("EnteredOrMicrotaskContext", proc("gov8_ca_context_entered_or_microtask"),
		i.handleAssumingCheck(), sh, uintptr(unsafe.Pointer(&out))); err != nil {
		return nil, err
	}
	return &ContextRef{iso: i, sc: s, h: out}, nil
}

// SameAs reports whether the observed context is c (engine identity).
func (r *ContextRef) SameAs(c *Context) (bool, error) {
	if err := r.check(); err != nil {
		return false, err
	}
	if c == nil {
		return false, errors.New("gov8: nil context")
	}
	if c.iso != r.iso {
		return false, nil
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return false, err
	}
	var local uintptr
	if err := callErr("ContextRef.SameAs", proc("gov8_ca_context_local"),
		r.iso.handleAssumingCheck(), c.handle, r.sc.handle,
		uintptr(unsafe.Pointer(&local))); err != nil {
		return false, err
	}
	if r.h == 0 || local == 0 {
		return r.h == local, nil
	}
	r1, _, _ := proc("gov8_ca_context_eq").Call(r.h, local)
	if int64(r1) < 0 {
		return false, shimError("ContextRef.SameAs", r1)
	}
	return r1 == 1, nil
}

// SameAsRef reports engine identity equality with another scope-local Context.
// Empty references compare equal only to another empty reference.
func (r *ContextRef) SameAsRef(other *ContextRef) (bool, error) {
	if err := r.check(); err != nil {
		return false, err
	}
	if other == nil {
		return false, errors.New("gov8: nil context reference")
	}
	if other.iso != r.iso {
		return false, nil
	}
	if err := other.check(); err != nil {
		return false, err
	}
	if r.h == 0 || other.h == 0 {
		return r.h == other.h, nil
	}
	r1, _, _ := proc("gov8_ca_context_eq").Call(r.h, other.h)
	if int64(r1) < 0 {
		return false, shimError("ContextRef.SameAsRef", r1)
	}
	return r1 == 1, nil
}

// SameValue reports ECMAScript SameValue equality (identity for objects;
// the security-token comparison of the oracle checks).
func (v Value) SameValue(other Value) (bool, error) {
	if err := v.check(); err != nil {
		return false, err
	}
	if err := other.check(); err != nil {
		return false, err
	}
	if other.iso != v.iso {
		return false, foreignIsolate("value")
	}
	r1, _, _ := proc("gov8_ca_value_same_value").Call(
		v.iso.handleAssumingCheck(), v.h, other.h)
	if int64(r1) < 0 {
		return false, shimError("SameValue", r1)
	}
	return r1 == 1, nil
}

// ---------------------------------------------------------------------------
// Embedder data (values and aligned pointers)
// ---------------------------------------------------------------------------

// kCtxInternalSlotCount mirrors the pinned crate's Context::INTERNAL_SLOT_COUNT:
// the first two engine embedder-data slots are reserved, and user slots are
// offset by this amount exactly like the oracle.
const kCtxInternalSlotCount = 2

const (
	ctxSlotKindNone    = 0
	ctxSlotKindValue   = 1
	ctxSlotKindPointer = 2
)

// ctxSlotState remembers the flavor written to each embedder slot so a
// value/pointer mix-up is rejected before it corrupts slot contents (an
// upstream write of one flavor over the other CHECK-fails or worse).
type ctxSlotState struct {
	mu    sync.Mutex
	kinds map[int]uint8
}

var ctxSlotStates = struct {
	mu sync.Mutex
	m  map[*Context]*ctxSlotState
}{m: make(map[*Context]*ctxSlotState)}

func slotStateOf(c *Context) *ctxSlotState {
	ctxSlotStates.mu.Lock()
	defer ctxSlotStates.mu.Unlock()
	st, ok := ctxSlotStates.m[c]
	if !ok {
		st = &ctxSlotState{kinds: make(map[int]uint8)}
		ctxSlotStates.m[c] = st
	}
	return st
}

// GetEmbedderData reads the value in embedder slot slot (0-based). ok=false
// when the engine reports the slot absent. NOTE: the engine's default slot
// content is an internal oddball, not a JS value — only predicates are
// meaningful on it (the pinned oracle records exactly those).
func (c *Context) GetEmbedderData(s *Scope, slot int) (Value, bool, error) {
	if err := validateContextEmbedderDataSlot(slot); err != nil {
		return Value{}, false, err
	}
	if err := c.check(); err != nil {
		return Value{}, false, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, false, err
	}
	if s.iso != c.iso {
		return Value{}, false, foreignIsolate("scope")
	}
	var out uintptr
	var ok int32
	r1, _, _ := proc("gov8_ca_context_get_embedder_data").Call(
		c.iso.handleAssumingCheck(), c.handle, sh, uintptr(slot+kCtxInternalSlotCount),
		uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&ok)))
	if int64(r1) < 0 {
		return Value{}, false, shimError("Context.GetEmbedderData", r1)
	}
	if ok != 1 {
		return Value{}, false, nil
	}
	return Value{iso: c.iso, sc: s, h: out}, true, nil
}

// SetEmbedderData writes a JS value into embedder slot slot.
func (c *Context) SetEmbedderData(s *Scope, slot int, v Value) error {
	if err := validateContextEmbedderDataSlot(slot); err != nil {
		return err
	}
	if err := c.check(); err != nil {
		return err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return err
	}
	if s.iso != c.iso {
		return foreignIsolate("scope")
	}
	if err := v.check(); err != nil {
		return err
	}
	if v.iso != c.iso {
		return foreignIsolate("value")
	}
	st := slotStateOf(c)
	st.mu.Lock()
	if kind := st.kinds[slot]; kind == ctxSlotKindPointer {
		st.mu.Unlock()
		return fmt.Errorf("gov8: embedder slot %d holds an aligned pointer", slot)
	}
	st.kinds[slot] = ctxSlotKindValue
	st.mu.Unlock()
	return callErr("Context.SetEmbedderData", proc("gov8_ca_context_set_embedder_data"),
		c.iso.handleAssumingCheck(), c.handle, sh,
		uintptr(slot+kCtxInternalSlotCount), v.h)
}

// SetAlignedPointerInEmbedderData stores an aligned raw pointer in embedder
// slot slot. The pointer crosses as a raw word; it must not be a Go pointer
// (the engine never dereferences it). Alignment is validated Go-side: the
// upstream ApiCheck-fatals ("Pointer is not aligned") on unaligned values.
func (c *Context) SetAlignedPointerInEmbedderData(slot int, p uintptr) error {
	if err := validateContextEmbedderDataSlot(slot); err != nil {
		return err
	}
	if p%8 != 0 {
		return fmt.Errorf("gov8: embedder pointer is not aligned: %#x", p)
	}
	if err := c.check(); err != nil {
		return err
	}
	st := slotStateOf(c)
	st.mu.Lock()
	if kind := st.kinds[slot]; kind == ctxSlotKindValue {
		st.mu.Unlock()
		return fmt.Errorf("gov8: embedder slot %d holds a value", slot)
	}
	st.kinds[slot] = ctxSlotKindPointer
	st.mu.Unlock()
	return callErr("Context.SetAlignedPointerInEmbedderData",
		proc("gov8_ca_context_set_aligned_pointer"),
		c.iso.handleAssumingCheck(), c.handle, uintptr(slot+kCtxInternalSlotCount), p)
}

// GetAlignedPointerFromEmbedderData reads the aligned pointer from embedder
// slot slot.
func (c *Context) GetAlignedPointerFromEmbedderData(slot int) (uintptr, error) {
	if err := validateContextEmbedderDataSlot(slot); err != nil {
		return 0, err
	}
	if err := c.check(); err != nil {
		return 0, err
	}
	var out uintptr
	if err := callErr("Context.GetAlignedPointerFromEmbedderData",
		proc("gov8_ca_context_get_aligned_pointer"),
		c.iso.handleAssumingCheck(), c.handle, uintptr(slot+kCtxInternalSlotCount),
		uintptr(unsafe.Pointer(&out))); err != nil {
		return 0, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Host-side context slots (the crate's Rc slots)
// ---------------------------------------------------------------------------
//
// The crate's Context::set_slot/get_slot/remove_slot key storage by the Rc
// element type; Go has no type-keyed singletons, so the storage is keyed by
// an explicit any key (the same mapping the isolate slots already use).
// Purely host-side: the engine is never involved.

var contextSlots = struct {
	mu sync.Mutex
	m  map[*Context]map[any]any
}{m: make(map[*Context]map[any]any)}

// SetSlot stores value under key and returns the previous value, if any
// (the crate's set_slot hands back the replaced Rc).
func (c *Context) SetSlot(key, value any) (previous any, wasEmpty bool) {
	contextSlots.mu.Lock()
	defer contextSlots.mu.Unlock()
	m, ok := contextSlots.m[c]
	if !ok {
		m = make(map[any]any)
		contextSlots.m[c] = m
	}
	prev, existed := m[key]
	m[key] = value
	return prev, !existed
}

// GetSlot returns the value stored under key.
func (c *Context) GetSlot(key any) (any, bool) {
	contextSlots.mu.Lock()
	defer contextSlots.mu.Unlock()
	m, ok := contextSlots.m[c]
	if !ok {
		return nil, false
	}
	v, ok := m[key]
	return v, ok
}

// RemoveSlot removes and returns the value stored under key.
func (c *Context) RemoveSlot(key any) (any, bool) {
	contextSlots.mu.Lock()
	defer contextSlots.mu.Unlock()
	m, ok := contextSlots.m[c]
	if !ok {
		return nil, false
	}
	v, existed := m[key]
	if existed {
		delete(m, key)
	}
	return v, existed
}

// ClearAllSlots removes every host-side slot of the context. Embedder data
// is engine state and survives, exactly like the pinned clear_all_slots.
func (c *Context) ClearAllSlots() {
	contextSlots.mu.Lock()
	defer contextSlots.mu.Unlock()
	delete(contextSlots.m, c)
}

// ---------------------------------------------------------------------------
// Raw isolate data slots
// ---------------------------------------------------------------------------

// DataSlotCount reports the isolate's raw data-slot count (3 for a plain
// isolate in this build — the pinned oracle's value).
func (i *Isolate) DataSlotCount() (int, error) {
	if err := i.check(); err != nil {
		return 0, err
	}
	if err := requireInitialized(); err != nil {
		return 0, err
	}
	var out int64
	r1, _, _ := proc("gov8_ca_isolate_data_slot_count").Call(
		i.handleAssumingCheck(), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("DataSlotCount", r1)
	}
	return int(out), nil
}

// GetData reads the raw pointer stored in slot. Indices are validated
// against the slot count: the upstream GetData performs no bounds check in
// release builds.
func (i *Isolate) GetData(slot int) (uintptr, error) {
	if err := i.check(); err != nil {
		return 0, err
	}
	if err := requireInitialized(); err != nil {
		return 0, err
	}
	var out uintptr
	if err := callErr("Isolate.GetData", proc("gov8_ca_isolate_get_data"),
		i.handleAssumingCheck(), uintptr(slot), uintptr(unsafe.Pointer(&out))); err != nil {
		return 0, err
	}
	return out, nil
}

// SetData stores a raw pointer in slot (data is passed through verbatim;
// it must not be a Go pointer). Indices are validated against the slot
// count: the upstream SetData performs no bounds check in release builds
// and an out-of-range slot is an unchecked OOB write.
func (i *Isolate) SetData(slot int, data uintptr) error {
	if err := i.check(); err != nil {
		return err
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	return callErr("Isolate.SetData", proc("gov8_ca_isolate_set_data"),
		i.handleAssumingCheck(), uintptr(slot), data)
}

// ---------------------------------------------------------------------------
// Script origins and the compiler surface
// ---------------------------------------------------------------------------

// Origin mirrors the ScriptOrigin knobs of the pinned crate. The zero value
// is the neutral origin (line/column 0, script id 0, no source map, plain
// classic script).
type Origin struct {
	// ResourceName is the script's file name. Required (the engine keys
	// exception positions and stack frames off it). It is retained as the
	// convenient string form used by existing callers.
	ResourceName string
	// ResourceNameValue supplies the resource name as an existing scope-local
	// JavaScript Value, matching ScriptOrigin's arbitrary Local<Value> input.
	// A non-zero Value takes precedence over ResourceName and preserves its
	// exact type and identity. It must be live and belong to this Context's
	// isolate when CompileWithOrigin is called. CompileUnbound and
	// CompileCached reject this form before FFI because the pinned code-cache
	// path fatals for object-valued resource names.
	ResourceNameValue Value
	// LineOffset/ColumnOffset shift reported line/column numbers.
	LineOffset   int32
	ColumnOffset int32
	// ScriptID is the origin-declared id. Fresh compiles get their own
	// engine-assigned id (the oracle pins that the declared id is ignored).
	ScriptID int32
	// SourceMapURL, or "" for none.
	SourceMapURL        string
	IsOpaque            bool
	IsSharedCrossOrigin bool
	// IsWasm and IsModule exist for completeness. IsModule with a classic
	// compile is an upstream engine FATAL in this build (ApiCheck:
	// "CompileModule must be used to compile modules"); modules are out of
	// milestone scope, and the boundary is characterized by the Go
	// subprocess tests rather than reachable productively.
	IsWasm   bool
	IsModule bool
}

func (o *Origin) flags() int64 {
	var f int64
	if o.IsOpaque {
		f |= 1
	}
	if o.IsSharedCrossOrigin {
		f |= 2
	}
	if o.IsWasm {
		f |= 4
	}
	if o.IsModule {
		f |= 8
	}
	return f
}

// originArgs materializes the origin's strings as NUL-free byte buffers for
// one shim call. The returned keep slice must stay alive across the call
// (runtime.KeepAlive after it).
func originArgs(o *Origin) (name []byte, smap []byte, keep [][]byte) {
	if o == nil {
		return nil, nil, nil
	}
	if len(o.ResourceName) > 0 {
		name = []byte(o.ResourceName)
		keep = append(keep, name)
	}
	if len(o.SourceMapURL) > 0 {
		smap = []byte(o.SourceMapURL)
		keep = append(keep, smap)
	}
	return name, smap, keep
}

func bytesArg(b []byte) (p uintptr) {
	if len(b) > 0 {
		p = uintptr(unsafe.Pointer(&b[0]))
	}
	return p
}

func (c *Context) originResourceValue(o *Origin) (uintptr, error) {
	if o == nil || o.ResourceNameValue.h == 0 {
		return 0, nil
	}
	if err := o.ResourceNameValue.check(); err != nil {
		return 0, fmt.Errorf("gov8: origin resource name: %w", err)
	}
	if o.ResourceNameValue.iso != c.iso {
		return 0, foreignIsolate("origin resource name")
	}
	return o.ResourceNameValue.h, nil
}

// CompileWithOrigin compiles source with a script origin. TryCatch routing
// matches Context.Compile.
func (c *Context) CompileWithOrigin(s *Scope, source string, origin *Origin, tc *TryCatch) (*Script, error) {
	if origin == nil {
		return c.Compile(s, source, tc)
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	if s.iso != c.iso {
		return nil, foreignIsolate("scope")
	}
	if tc != nil {
		if tc.iso != c.iso {
			return nil, foreignIsolate("trycatch")
		}
		if err := tc.check(); err != nil {
			return nil, err
		}
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	src := []byte(source)
	name, smap, keep := originArgs(origin)
	resourceValue, err := c.originResourceValue(origin)
	if err != nil {
		return nil, err
	}
	var tcv uintptr
	if tc != nil {
		tcv = tc.handle
	}
	var out uintptr
	var r1 uintptr
	if resourceValue != 0 {
		r1, _, _ = proc("gov8_ca_script_compile_origin_value").Call(
			c.iso.handleAssumingCheck(), c.handle, sh, tcv,
			bytesArg(src), uintptr(len(src)), bytesArg(name), uintptr(len(name)),
			resourceValue, uintptr(int32(origin.LineOffset)), uintptr(int32(origin.ColumnOffset)),
			uintptr(int32(origin.ScriptID)), bytesArg(smap), uintptr(len(smap)),
			uintptr(origin.flags()), uintptr(unsafe.Pointer(&out)))
	} else {
		r1, _, _ = proc("gov8_ca_script_compile_origin").Call(
			c.iso.handleAssumingCheck(), c.handle, sh, tcv,
			bytesArg(src), uintptr(len(src)), bytesArg(name), uintptr(len(name)),
			uintptr(int32(origin.LineOffset)), uintptr(int32(origin.ColumnOffset)),
			uintptr(int32(origin.ScriptID)), bytesArg(smap), uintptr(len(smap)),
			uintptr(origin.flags()), uintptr(unsafe.Pointer(&out)))
	}
	runtime.KeepAlive(keep)
	runtime.KeepAlive(src)
	if int64(r1) < 0 {
		return nil, shimError("CompileWithOrigin", r1)
	}
	return &Script{iso: c.iso, ctx: c, handle: out}, nil
}

// UnboundScript is the context-independent form of a compiled script
// (rooted in a persistent handle). It can be re-bound into any context of
// its isolate.
type UnboundScript struct {
	iso    *Isolate
	handle uintptr
	closed bool
}

func (u *UnboundScript) check() error {
	if err := u.iso.check(); err != nil {
		return err
	}
	if u.closed {
		return errors.New("gov8: unbound script used after Close")
	}
	return nil
}

// Unbound returns the script's context-independent form
// (Script::GetUnboundScript).
func (sc *Script) Unbound() (*UnboundScript, error) {
	if err := sc.check(); err != nil {
		return nil, err
	}
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	h, err := callHandle("Script.Unbound", proc("gov8_ca_unbound_from_script"), sc.handle)
	if err != nil {
		return nil, err
	}
	return &UnboundScript{iso: sc.iso, handle: h}, nil
}

// ID returns the unbound script's engine id (equal to its source script's).
func (u *UnboundScript) ID() (int32, error) {
	if err := u.check(); err != nil {
		return 0, err
	}
	var out int32
	r1, _, _ := proc("gov8_ca_unbound_id").Call(
		u.iso.handleAssumingCheck(), u.handle, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("UnboundScript.ID", r1)
	}
	return out, nil
}

// BoundScript is a script bound into the currently entered context
// (UnboundScript::bind_to_current_context). It is a scope-local handle.
type BoundScript struct {
	Value
}

// Bind binds the unbound script into the context entered via Context.Enter
// (upstream the entered-context precondition is enforced by the type
// system; here it is a runtime requirement — binding without an entered
// context binds into the engine's default, which the checks never rely on).
func (u *UnboundScript) Bind(s *Scope) (BoundScript, error) {
	if err := u.check(); err != nil {
		return BoundScript{}, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return BoundScript{}, err
	}
	var out uintptr
	if err := callErr("UnboundScript.Bind", proc("gov8_ca_unbound_bind"),
		u.iso.handleAssumingCheck(), sh, u.handle, uintptr(unsafe.Pointer(&out))); err != nil {
		return BoundScript{}, err
	}
	return BoundScript{Value{iso: u.iso, sc: s, h: out}}, nil
}

// Run executes the bound script in c. TryCatch routing matches Script.Run.
func (b BoundScript) Run(c *Context, s *Scope, tc *TryCatch) (Value, error) {
	if err := c.check(); err != nil {
		return Value{}, err
	}
	if s.iso != c.iso {
		return Value{}, foreignIsolate("scope")
	}
	if err := b.check(); err != nil {
		return Value{}, err
	}
	if b.iso != c.iso {
		return Value{}, foreignIsolate("script")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, err
	}
	var tcv uintptr
	if tc != nil {
		if tc.iso != c.iso {
			return Value{}, foreignIsolate("trycatch")
		}
		if err := tc.check(); err != nil {
			return Value{}, err
		}
		tcv = tc.handle
	}
	var out uintptr
	r1, _, _ := proc("gov8_ca_local_script_run").Call(
		c.iso.handleAssumingCheck(), c.handle, sh, b.h, tcv, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("BoundScript.Run", r1)
	}
	return Value{iso: c.iso, sc: s, h: out}, nil
}

// Close releases the unbound script's persistent handle.
func (u *UnboundScript) Close() error {
	if err := u.iso.check(); err != nil {
		return err
	}
	if u.closed {
		return errors.New("gov8: unbound script already closed")
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	r1, _, _ := proc("gov8_ca_unbound_dispose").Call(u.handle)
	u.closed = true
	if int64(r1) < 0 {
		return shimError("UnboundScript.Close", r1)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ScriptCompiler
// ---------------------------------------------------------------------------

// CompileOptions mirror the pinned crate's script_compiler::CompileOptions
// subset this slice exercises.
type CompileOptions uint32

const (
	// OptNoCompileOptions is the default compile.
	OptNoCompileOptions CompileOptions = 0
	// OptConsumeCodeCache consumes pre-produced cache bytes.
	OptConsumeCodeCache CompileOptions = 1
	// OptEagerCompile eagerly compiles (no lazy inner functions).
	OptEagerCompile CompileOptions = 2
)

// CompileUnbound compiles an unbound script with a string-valued origin and options
// (ScriptCompiler::compile_unbound_script). TryCatch routing matches
// Context.Compile.
func (c *Context) CompileUnbound(s *Scope, source string, origin *Origin, opts CompileOptions, tc *TryCatch) (*UnboundScript, error) {
	if opts == OptConsumeCodeCache {
		return nil, fmt.Errorf("gov8: use CompileCached to consume a code cache")
	}
	if origin != nil && origin.ResourceNameValue.h != 0 {
		return nil, fmt.Errorf("gov8: CompileUnbound does not support Value resource names")
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	if s.iso != c.iso {
		return nil, foreignIsolate("scope")
	}
	if tc != nil {
		if tc.iso != c.iso {
			return nil, foreignIsolate("trycatch")
		}
		if err := tc.check(); err != nil {
			return nil, err
		}
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	src := []byte(source)
	name, smap, keep := originArgs(origin)
	var tcv uintptr
	if tc != nil {
		tcv = tc.handle
	}
	var out uintptr
	r1, _, _ := proc("gov8_ca_compile_unbound").Call(
		c.iso.handleAssumingCheck(), c.handle, sh, tcv,
		bytesArg(src), uintptr(len(src)), bytesArg(name), uintptr(len(name)),
		uintptr(int32(origin.LineOffset)), uintptr(int32(origin.ColumnOffset)),
		uintptr(int32(origin.ScriptID)), bytesArg(smap), uintptr(len(smap)),
		uintptr(origin.flags()), uintptr(opts), uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(keep)
	runtime.KeepAlive(src)
	if int64(r1) < 0 {
		return nil, shimError("CompileUnbound", r1)
	}
	return &UnboundScript{iso: c.iso, handle: out}, nil
}

// CheckCodeCache prevalidates consumer cache bytes with the engine's
// graceful header sanity check (CachedData::CompatibilityCheck) WITHOUT
// entering the code-cache deserializer. It returns the raw sanity-check
// result (0 = compatible) so callers can distinguish rejection reasons, and
// an error for wrapper-level misuse. Bytes below the minimum serialized-code
// header size are rejected without touching the engine. This prevents the
// upstream deserializer fatal for header-level corruption; mid-payload
// corruption that passes the header checks is not detectable without
// running the (fatal-prone) deserializer in this build and is characterized
// in the subprocess tests instead.
func (i *Isolate) CheckCodeCache(cache []byte) (int, error) {
	if err := i.check(); err != nil {
		return -1, err
	}
	if err := requireInitialized(); err != nil {
		return -1, err
	}
	r1, _, _ := proc("gov8_ca_code_cache_precheck").Call(
		i.handleAssumingCheck(), bytesArg(cache), uintptr(len(cache)))
	if int64(r1) < 0 {
		return -1, shimError("CheckCodeCache", r1)
	}
	return int(r1), nil
}

// CompileCached compiles source consuming pre-produced code-cache bytes
// (ScriptCompiler::compile with ConsumeCodeCache). The cache is
// prevalidated with CheckCodeCache first: an incompatible cache is rejected
// with an error instead of reaching the deserializer. rejected reports the
// engine's own post-compile rejected flag (false for a healthy cache).
// Value-valued resource names are rejected before FFI; only the direct
// CompileWithOrigin path is characterized as safe for arbitrary Values.
func (c *Context) CompileCached(s *Scope, source string, origin *Origin, cache []byte, tc *TryCatch) (script *Script, rejected bool, err error) {
	if origin != nil && origin.ResourceNameValue.h != 0 {
		return nil, false, fmt.Errorf("gov8: CompileCached does not support Value resource names")
	}
	if err := c.check(); err != nil {
		return nil, false, err
	}
	// Corruption prevalidation: never hand unchecked bytes to the code-cache
	// deserializer (a failed sanity check is a graceful rejection; corruption
	// past the header is an engine fatal upstream).
	if status, perr := c.iso.CheckCodeCache(cache); perr != nil {
		return nil, false, fmt.Errorf("gov8: code cache precheck: %w", perr)
	} else if status != 0 {
		return nil, true, fmt.Errorf("gov8: code cache rejected by sanity check (status %d)", status)
	}
	if s.iso != c.iso {
		return nil, false, foreignIsolate("scope")
	}
	if tc != nil {
		if tc.iso != c.iso {
			return nil, false, foreignIsolate("trycatch")
		}
		if err := tc.check(); err != nil {
			return nil, false, err
		}
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, false, err
	}
	src := []byte(source)
	name, smap, keep := originArgs(origin)
	var tcv uintptr
	if tc != nil {
		tcv = tc.handle
	}
	var out uintptr
	var rej int32
	r1, _, _ := proc("gov8_ca_script_compile_cached").Call(
		c.iso.handleAssumingCheck(), c.handle, sh, tcv,
		bytesArg(src), uintptr(len(src)), bytesArg(name), uintptr(len(name)),
		uintptr(int32(origin.LineOffset)), uintptr(int32(origin.ColumnOffset)),
		uintptr(int32(origin.ScriptID)), bytesArg(smap), uintptr(len(smap)),
		uintptr(origin.flags()), uintptr(OptConsumeCodeCache),
		bytesArg(cache), uintptr(len(cache)),
		uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&rej)))
	runtime.KeepAlive(keep)
	runtime.KeepAlive(src)
	runtime.KeepAlive(cache)
	if int64(r1) < 0 {
		return nil, false, shimError("CompileCached", r1)
	}
	return &Script{iso: c.iso, ctx: c, handle: out}, rej != 0, nil
}

// CreateCodeCache produces the script's code cache bytes
// (UnboundScript::CreateCodeCache). The bytes are a plain Go copy; the
// engine allocation is released inside the call.
func (u *UnboundScript) CreateCodeCache() ([]byte, error) {
	if err := u.check(); err != nil {
		return nil, err
	}
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	cd, err := callHandle("CreateCodeCache", proc("gov8_ca_code_cache_new"), u.handle)
	if err != nil {
		return nil, err
	}
	if cd == 0 {
		return nil, errors.New("gov8: code cache production failed")
	}
	// Two-step read with growth: start from a reasonable capacity and retry
	// once with the reported size (read_delete frees the engine object even
	// on the too-small path).
	capacity := 4096
	for i := 0; i < 2; i++ {
		buf := make([]byte, capacity)
		var n int64
		r1, _, _ := proc("gov8_ca_code_cache_read_delete").Call(
			cd, uintptr(unsafe.Pointer(&buf[0])), uintptr(capacity), uintptr(unsafe.Pointer(&n)))
		if int64(r1) == errNoMemory {
			if n <= 0 {
				return nil, shimError("CreateCodeCache", r1)
			}
			capacity = int(n)
			continue
		}
		if int64(r1) < 0 {
			return nil, shimError("CreateCodeCache", r1)
		}
		if n < 0 || int(n) > len(buf) {
			n = int64(len(buf))
		}
		return buf[:n], nil
	}
	return nil, errors.New("gov8: code cache read failed")
}

// CachedDataVersionTag returns the engine's code-cache version tag (pinned
// value 3252425384 for this build).
func CachedDataVersionTag() (uint32, error) {
	if err := requireInitialized(); err != nil {
		return 0, err
	}
	var out int64
	r1, _, _ := proc("gov8_ca_cached_data_version_tag").Call(uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("CachedDataVersionTag", r1)
	}
	return uint32(out), nil
}

// CompileFunction compiles source as a function body with declared
// parameter names (ScriptCompiler::compile_function). The result is a
// scope-local function value.
func (c *Context) CompileFunction(s *Scope, source string, params []string, tc *TryCatch) (Value, error) {
	if err := c.check(); err != nil {
		return Value{}, err
	}
	if s.iso != c.iso {
		return Value{}, foreignIsolate("scope")
	}
	if tc != nil {
		if tc.iso != c.iso {
			return Value{}, foreignIsolate("trycatch")
		}
		if err := tc.check(); err != nil {
			return Value{}, err
		}
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, err
	}
	src := []byte(source)
	// Param names cross as C strings for the duration of the call; the
	// backing byte slices are kept alive across it.
	paramPtrs := make([]uintptr, len(params))
	keep := make([][]byte, 0, len(params))
	for i, p := range params {
		b := make([]byte, len(p)+1)
		copy(b, p)
		keep = append(keep, b)
		paramPtrs[i] = uintptr(unsafe.Pointer(&b[0]))
	}
	var paramArg uintptr
	if len(paramPtrs) > 0 {
		paramArg = uintptr(unsafe.Pointer(&paramPtrs[0]))
	}
	var tcv uintptr
	if tc != nil {
		tcv = tc.handle
	}
	var out uintptr
	r1, _, _ := proc("gov8_ca_compile_function").Call(
		c.iso.handleAssumingCheck(), c.handle, sh, tcv,
		bytesArg(src), uintptr(len(src)),
		paramArg, uintptr(len(params)), uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(keep)
	runtime.KeepAlive(src)
	runtime.KeepAlive(paramPtrs)
	if int64(r1) < 0 {
		return Value{}, shimError("CompileFunction", r1)
	}
	return Value{iso: c.iso, sc: s, h: out}, nil
}

var (
	callFunctionProcOnce sync.Once
	callFunctionProc     *syscall.Proc
)

func ensureCallFunctionProc() {
	callFunctionProcOnce.Do(func() {
		callFunctionProc = proc("gov8_ca_function_call")
	})
}

// CallFunction invokes fn with recv and args in the context. TryCatch
// routing matches Script.Run: a thrown exception yields an error and is
// recorded in tc when supplied.
func CallFunction(c *Context, s *Scope, fn, recv Value, args []Value, tc *TryCatch) (Value, error) {
	if err := c.check(); err != nil {
		return Value{}, err
	}
	if s.iso != c.iso {
		return Value{}, foreignIsolate("scope")
	}
	if err := fn.check(); err != nil {
		return Value{}, err
	}
	if fn.iso != c.iso {
		return Value{}, foreignIsolate("function")
	}
	if err := recv.check(); err != nil {
		return Value{}, err
	}
	if recv.iso != c.iso {
		return Value{}, foreignIsolate("receiver")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, err
	}
	var tcv uintptr
	if tc != nil {
		if tc.iso != c.iso {
			return Value{}, foreignIsolate("trycatch")
		}
		if err := tc.check(); err != nil {
			return Value{}, err
		}
		tcv = tc.handle
	}
	wires := valueWires(args)
	var argv uintptr
	if len(wires) > 0 {
		argv = uintptr(unsafe.Pointer(&wires[0]))
	}
	var out uintptr
	ensureCallFunctionProc()
	r1, _, _ := callFunctionProc.Call(
		c.iso.handleAssumingCheck(), c.handle, sh, fn.h, recv.h,
		argv, uintptr(len(wires)), tcv, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("CallFunction", r1)
	}
	return Value{iso: c.iso, sc: s, h: out}, nil
}

// ---------------------------------------------------------------------------
// GC notifications
// ---------------------------------------------------------------------------

// GCType mirrors v8::GCType.
type GCType uint32

const (
	GCTypeScavenge             GCType = 1 << 0
	GCTypeMinorMarkSweep       GCType = 1 << 1
	GCTypeMarkSweepCompact     GCType = 1 << 2
	GCTypeIncrementalMarking   GCType = 1 << 3
	GCTypeProcessWeakCallbacks GCType = 1 << 4
	GCTypeAll                  GCType = 0x1F
)

// GCCallbackFlags mirrors v8::GCCallbackFlags.
type GCCallbackFlags uint32

// GCCallback is the prologue/epilogue callback. It runs on the isolate's
// thread inside engine GC and must not re-enter the engine.
type GCCallback func(gcType GCType, flags GCCallbackFlags)

type gcEntry struct {
	iso *Isolate
	cb  GCCallback
}

var gcRegistry = struct {
	mu      sync.Mutex
	next    int64
	entries map[int64]*gcEntry
}{entries: make(map[int64]*gcEntry)}

var (
	gcDispatcherOnce sync.Once
	gcDispatcherErr  error
)

var goGCDispatch = syscall.NewCallback(func(iso, gcType, flags, data uintptr) uintptr {
	gcRegistry.mu.Lock()
	entry := gcRegistry.entries[int64(data)]
	gcRegistry.mu.Unlock()
	if entry == nil {
		return 1
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in GC callback: %v\n", r)
			proc("gov8_host_panic_abort").Call()
		}
	}()
	entry.cb(GCType(gcType), GCCallbackFlags(flags))
	return 1
})

func ensureGCDispatcher() error {
	gcDispatcherOnce.Do(func() {
		gcDispatcherErr = callErr("SetGCDispatcher", proc("gov8_ca_gc_set_entry"), goGCDispatch)
	})
	return gcDispatcherErr
}

// GCCallbackToken identifies a registered GC callback for removal.
type GCCallbackToken struct {
	iso      *Isolate
	id       int64
	prologue bool
}

func addGCCallback(i *Isolate, cb GCCallback, filter GCType, prologue bool) (*GCCallbackToken, error) {
	if cb == nil {
		return nil, errors.New("gov8: GC callback required")
	}
	if err := i.check(); err != nil {
		return nil, err
	}
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	if err := ensureGCDispatcher(); err != nil {
		return nil, err
	}
	gcRegistry.mu.Lock()
	gcRegistry.next++
	id := gcRegistry.next
	gcRegistry.entries[id] = &gcEntry{iso: i, cb: cb}
	gcRegistry.mu.Unlock()
	var err error
	if prologue {
		err = callErr("AddGCPrologueCallback", proc("gov8_ca_add_gc_prologue_callback"),
			i.handle, uintptr(id), uintptr(filter))
	} else {
		err = callErr("AddGCEpilogueCallback", proc("gov8_ca_add_gc_epilogue_callback"),
			i.handle, uintptr(id), uintptr(filter))
	}
	if err != nil {
		gcRegistry.mu.Lock()
		delete(gcRegistry.entries, id)
		gcRegistry.mu.Unlock()
		return nil, err
	}
	return &GCCallbackToken{iso: i, id: id, prologue: prologue}, nil
}

func removeGCCallback(i *Isolate, t *GCCallbackToken) error {
	if t == nil {
		return errors.New("gov8: nil GC callback token")
	}
	if t.iso != i {
		return foreignIsolate("GC callback")
	}
	if err := i.check(); err != nil {
		return err
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	gcRegistry.mu.Lock()
	_, live := gcRegistry.entries[t.id]
	gcRegistry.mu.Unlock()
	if !live {
		return errors.New("gov8: GC callback already removed")
	}
	var err error
	if t.prologue {
		err = callErr("RemoveGCPrologueCallback", proc("gov8_ca_remove_gc_prologue_callback"),
			i.handle, uintptr(t.id))
	} else {
		err = callErr("RemoveGCEpilogueCallback", proc("gov8_ca_remove_gc_epilogue_callback"),
			i.handle, uintptr(t.id))
	}
	if err == nil {
		gcRegistry.mu.Lock()
		delete(gcRegistry.entries, t.id)
		gcRegistry.mu.Unlock()
	}
	return err
}

// AddGCPrologueCallback registers cb to run before each GC matching filter.
func (i *Isolate) AddGCPrologueCallback(cb GCCallback, filter GCType) (*GCCallbackToken, error) {
	return addGCCallback(i, cb, filter, true)
}

// AddGCEpilogueCallback registers cb to run after each GC matching filter.
func (i *Isolate) AddGCEpilogueCallback(cb GCCallback, filter GCType) (*GCCallbackToken, error) {
	return addGCCallback(i, cb, filter, false)
}

// RemoveGCPrologueCallback removes a prologue registration.
func (i *Isolate) RemoveGCPrologueCallback(t *GCCallbackToken) error {
	return removeGCCallback(i, t)
}

// RemoveGCEpilogueCallback removes an epilogue registration.
func (i *Isolate) RemoveGCEpilogueCallback(t *GCCallbackToken) error {
	return removeGCCallback(i, t)
}

// ---------------------------------------------------------------------------
// Heap statistics
// ---------------------------------------------------------------------------

// HeapStatistics is a snapshot of the isolate's heap counters. Sizes are
// machine-dependent; the deterministic invariants are the comparisons the
// oracle pins.
type HeapStatistics struct {
	TotalHeapSize            uint64
	TotalHeapSizeExecutable  uint64
	TotalPhysicalSize        uint64
	TotalAvailableSize       uint64
	UsedHeapSize             uint64
	HeapSizeLimit            uint64
	MallocedMemory           uint64
	ExternalMemory           uint64
	PeakMallocedMemory       uint64
	DoesZapGarbage           bool
	NumberOfNativeContexts   uint64
	NumberOfDetachedContexts uint64
	TotalGlobalHandlesSize   uint64
	UsedGlobalHandlesSize    uint64
	TotalAllocatedBytes      uint64
}

// GetHeapStatistics snapshots the isolate's heap statistics.
func (i *Isolate) GetHeapStatistics() (*HeapStatistics, error) {
	if err := i.check(); err != nil {
		return nil, err
	}
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	var out [15]int64
	r1, _, _ := proc("gov8_ca_heap_statistics").Call(
		i.handleAssumingCheck(), uintptr(unsafe.Pointer(&out[0])))
	if int64(r1) < 0 {
		return nil, shimError("GetHeapStatistics", r1)
	}
	return &HeapStatistics{
		TotalHeapSize:            uint64(out[0]),
		TotalHeapSizeExecutable:  uint64(out[1]),
		TotalPhysicalSize:        uint64(out[2]),
		TotalAvailableSize:       uint64(out[3]),
		UsedHeapSize:             uint64(out[4]),
		HeapSizeLimit:            uint64(out[5]),
		MallocedMemory:           uint64(out[6]),
		ExternalMemory:           uint64(out[7]),
		PeakMallocedMemory:       uint64(out[8]),
		DoesZapGarbage:           out[9] != 0,
		NumberOfNativeContexts:   uint64(out[10]),
		NumberOfDetachedContexts: uint64(out[11]),
		TotalGlobalHandlesSize:   uint64(out[12]),
		UsedGlobalHandlesSize:    uint64(out[13]),
		TotalAllocatedBytes:      uint64(out[14]),
	}, nil
}

// AdjustAmountOfExternalAllocatedMemory changes the external-memory
// accounting by delta and returns the new total (the engine's return, not
// the previous total).
func (i *Isolate) AdjustAmountOfExternalAllocatedMemory(delta int64) (int64, error) {
	if err := i.check(); err != nil {
		return 0, err
	}
	if err := requireInitialized(); err != nil {
		return 0, err
	}
	var out int64
	r1, _, _ := proc("gov8_ca_adjust_external_memory").Call(
		i.handleAssumingCheck(), uintptr(delta), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("AdjustAmountOfExternalAllocatedMemory", r1)
	}
	return out, nil
}

// NumberOfHeapSpaces reports the isolate's heap space count (13 in this
// build — the pinned oracle's value).
func (i *Isolate) NumberOfHeapSpaces() (int64, error) {
	if err := i.check(); err != nil {
		return 0, err
	}
	if err := requireInitialized(); err != nil {
		return 0, err
	}
	var out int64
	r1, _, _ := proc("gov8_ca_number_of_heap_spaces").Call(
		i.handleAssumingCheck(), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("NumberOfHeapSpaces", r1)
	}
	return out, nil
}
