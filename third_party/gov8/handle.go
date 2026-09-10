//go:build windows && amd64

package gov8

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// Public persistent handles: Global (strong) and Weak, mirroring the pinned
// crate's handle.rs observably:
//
// - A Global is a strong persistent cell. Clone creates a NEW cell holding
//   the same object; identity comparison (Equal) compares the objects, so a
//   clone in a different cell still compares equal and a distinct object
//   stays unequal — matching v8__Data__EQ-based PartialEq. Globals of two
//   different live isolates compare unequal without touching either engine
//   (the pinned PartialEq early-return, preserved).
// - IntoRaw/GlobalFromRaw round-trip the cell pointer; a dropped raw cell
//   that is never re-adopted pins its object until the isolate dies (same
//   caveat as the pinned into_raw/from_raw pair).
// - A Global may be closed after its isolate: the engine's global handles
//   died with the isolate, so Close is a silent no-op there (matching the
//   pinned Drop-for-disposed-isolate path). Every other use after the
//   isolate was closed returns an error instead of panicking — the
//   module-wide panic-to-error deviation (the pinned crate aborts on handle
//   equality after dispose; characterized as mode=global-eq-after-dispose).
// - A Weak mirrors the pinned Weak: Clone carries no finalizer and points
//   at the same object; a collected weak equals another collected weak but
//   nothing else; Weak.Close while the object is still strongly held resets
//   the cell and cancels the pending finalizer; a guaranteed finalizer runs
//   before the isolate is disposed even if the Weak itself is never closed.
//
// Ownership: finalizers are dispatched through an INTEGER registry (no Go
// pointer ever crosses the boundary). Finalizer callbacks must be pure Go —
// they run during GC or isolate teardown, where re-entering the engine is
// not permitted. Weak objects, like everything else in this module, are
// thread-affine and must be closed deterministically on the owning thread.

// Global is a strong persistent handle to a JS value.
type Global struct {
	iso    *Isolate
	cell   uintptr
	closed bool
}

// NewGlobal roots v in a new strong persistent cell. The value (and the
// scope it was created in) must belong to the same isolate as the scope.
func NewGlobal(s *Scope, v Value) (*Global, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if err := v.check(); err != nil {
		return nil, err
	}
	if v.iso != s.iso {
		return nil, foreignIsolate("value")
	}
	ih, err := s.iso.handleChecked()
	if err != nil {
		return nil, err
	}
	cell, err := callHandle("Global.New", proc("gov8_global_new"), ih, s.handle, v.h)
	if err != nil {
		return nil, err
	}
	return &Global{iso: s.iso, cell: cell}, nil
}

func (g *Global) check() error {
	if g == nil {
		return fmt.Errorf("gov8: nil global")
	}
	if g.closed {
		return fmt.Errorf("gov8: global used after Close")
	}
	return g.iso.check()
}

// Clone creates a new strong cell holding the same object (Global::clone).
func (g *Global) Clone() (*Global, error) {
	if err := g.check(); err != nil {
		return nil, err
	}
	ih, err := g.iso.handleChecked()
	if err != nil {
		return nil, err
	}
	cell, err := callHandle("Global.Clone", proc("gov8_global_clone"), ih, g.cell)
	if err != nil {
		return nil, err
	}
	return &Global{iso: g.iso, cell: cell}, nil
}

// Equal reports whether both globals hold the same object (object identity,
// not cell identity). Globals hosted by different live isolates compare
// unequal without touching either isolate, exactly like the pinned crate.
func (g *Global) Equal(other *Global) (bool, error) {
	if other == nil {
		return false, fmt.Errorf("gov8: cannot compare a global with nil")
	}
	if g.iso != other.iso {
		return false, nil
	}
	if err := g.check(); err != nil {
		return false, err
	}
	if err := other.check(); err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_data_eq").Call(g.cell, other.cell)
	if int64(r1) < 0 {
		return false, shimError("Global.Equal", r1)
	}
	return r1 == 1, nil
}

// ToLocal reopens the global as a scope-local value. The value is valid
// while the scope is open.
func (g *Global) ToLocal(s *Scope) (Value, error) {
	if err := g.check(); err != nil {
		return Value{}, err
	}
	if s.iso != g.iso {
		return Value{}, foreignIsolate("scope")
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	if err := callErr("Global.ToLocal", proc("gov8_global_to_local"),
		g.iso.handle, sh, g.cell, uintptr(unsafe.Pointer(&out))); err != nil {
		return Value{}, err
	}
	return Value{iso: g.iso, sc: s, h: out}, nil
}

// Close releases the strong cell. After the host isolate was closed this is
// a silent no-op (the engine handles died with the isolate), matching the
// pinned Drop behavior for disposed isolates.
func (g *Global) Close() error {
	if g == nil {
		return fmt.Errorf("gov8: nil global")
	}
	if g.closed {
		return fmt.Errorf("gov8: global already closed")
	}
	if isolateClosed(g.iso) {
		g.closed = true
		return nil
	}
	if err := g.iso.check(); err != nil {
		return err
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	err := callErr("Global.Close", proc("gov8_global_reset"), g.iso.handle, g.cell)
	g.closed = true
	return err
}

// IntoRaw consumes the global and returns the raw cell pointer. The caller
// MUST eventually pass it to GlobalFromRaw (on the same isolate), otherwise
// the referenced object stays pinned until the isolate dies. The global
// wrapper is consumed and must not be used afterwards.
func (g *Global) IntoRaw() (uintptr, error) {
	if err := g.check(); err != nil {
		return 0, err
	}
	raw := g.cell
	g.cell = 0
	g.closed = true
	return raw, nil
}

// GlobalFromRaw adopts a raw cell produced by (*Global).IntoRaw. The raw
// pointer must originate from the same isolate and must not be adopted
// twice.
func GlobalFromRaw(i *Isolate, raw uintptr) (*Global, error) {
	if raw == 0 {
		return nil, fmt.Errorf("gov8: zero raw global handle")
	}
	if err := i.check(); err != nil {
		return nil, err
	}
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	return &Global{iso: i, cell: raw}, nil
}

// WeakFinalizer is the regular weak-finalizer callback. It runs after the
// object lost its last strong reference and the engine processed the weak
// cell. It must be pure Go: the engine must not be re-entered from a
// finalizer (it runs during GC or isolate teardown).
type WeakFinalizer func(i *Isolate)

// weakEntry is one registered finalizer. Exactly one of regular/guaranteed
// is non-nil. weak/wrap back-references let the guaranteed-finalizer drain
// mark the Go Weak consumed and free the engine wrap.
type weakEntry struct {
	iso        *Isolate
	weak       *Weak
	wrap       uintptr
	regular    WeakFinalizer
	guaranteed func()
}

var weakRegistry = struct {
	mu      sync.Mutex
	next    int64
	entries map[int64]*weakEntry
}{entries: make(map[int64]*weakEntry)}

// weakLiveness counts every non-empty Weak wrapper, including handles with no
// finalizer registration. rusty_v8 refuses OwnedIsolate::try_into_shared while
// any such handle is alive and refuses creating one after conversion.
var weakLiveness = struct {
	sync.Mutex
	byIsolate map[*Isolate]int
}{byIsolate: make(map[*Isolate]int)}

func registerWeakLiveness(w *Weak) {
	weakLiveness.Lock()
	weakLiveness.byIsolate[w.iso]++
	w.tracked = true
	weakLiveness.Unlock()
}

func unregisterWeakLiveness(w *Weak) {
	if w == nil || !w.tracked {
		return
	}
	weakLiveness.Lock()
	if count := weakLiveness.byIsolate[w.iso]; count <= 1 {
		delete(weakLiveness.byIsolate, w.iso)
	} else {
		weakLiveness.byIsolate[w.iso] = count - 1
	}
	w.tracked = false
	weakLiveness.Unlock()
}

func liveWeakCount(i *Isolate) int {
	weakLiveness.Lock()
	defer weakLiveness.Unlock()
	return weakLiveness.byIsolate[i]
}

var (
	weakDispatcherOnce sync.Once
	weakDispatcherErr  error
)

// goWeakDispatch is the single entry point handed to the shim; all weak
// finalizers funnel through it. It runs on the isolate's owning thread,
// inside the engine's GC or teardown processing.
var goWeakDispatch = syscall.NewCallback(weakFinalizerDispatch)

func ensureWeakDispatcher() error {
	weakDispatcherOnce.Do(func() {
		weakDispatcherErr = callErr("SetWeakDispatcher",
			proc("gov8_weak_set_entry"), goWeakDispatch)
	})
	return weakDispatcherErr
}

func weakFinalizerDispatch(id int64) uintptr {
	weakRegistry.mu.Lock()
	entry := weakRegistry.entries[id]
	weakRegistry.mu.Unlock()
	if entry == nil {
		// Cancelled by Weak.Close between the engine scheduling the
		// callback and its dispatch: nothing to run (the pinned crate's
		// remove_finalizer path).
		return 1
	}
	weakRegistry.mu.Lock()
	delete(weakRegistry.entries, id)
	weakRegistry.mu.Unlock()
	// A panic here would unwind through engine GC frames; convert it into
	// the process abort documented for native callbacks (same boundary).
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in weak finalizer: %v\n", r)
			proc("gov8_host_panic_abort").Call()
		}
	}()
	if entry.regular != nil {
		entry.regular(entry.iso)
	} else {
		entry.guaranteed()
	}
	return 1
}

func addWeakEntry(w *Weak, wrap uintptr, regular WeakFinalizer, guaranteed func()) int64 {
	weakRegistry.mu.Lock()
	weakRegistry.next++
	id := weakRegistry.next
	weakRegistry.entries[id] = &weakEntry{iso: w.iso, weak: w, wrap: wrap, regular: regular, guaranteed: guaranteed}
	weakRegistry.mu.Unlock()
	return id
}

func removeWeakEntry(id int64) {
	if id == 0 {
		return
	}
	weakRegistry.mu.Lock()
	delete(weakRegistry.entries, id)
	weakRegistry.mu.Unlock()
}

// Weak is a weak persistent handle to a JS value.
type Weak struct {
	iso     *Isolate
	wrap    uintptr // engine WeakWrap; 0 for empty weaks
	empty   bool
	id      int64 // Go finalizer registry id; 0 = none
	tracked bool
	closed  bool
}

// NewWeak creates a weak handle over the global's object without a
// finalizer (Weak::new).
func (g *Global) NewWeak() (*Weak, error) {
	return g.newWeak(nil, nil)
}

// NewWeakWithFinalizer creates a weak handle that invokes cb after the
// object is collected. There is no guarantee the callback runs at all
// (GC-based finalization is best effort, matching the pinned crate); use
// NewWeakWithGuaranteedFinalizer for resource management.
func (g *Global) NewWeakWithFinalizer(cb WeakFinalizer) (*Weak, error) {
	if cb == nil {
		return nil, fmt.Errorf("gov8: weak finalizer required")
	}
	return g.newWeak(cb, nil)
}

// NewWeakWithGuaranteedFinalizer creates a weak handle whose callback is
// guaranteed to run before the isolate is disposed (it may run earlier,
// upon collection). The callback receives no isolate: it may run during
// isolate teardown.
func (g *Global) NewWeakWithGuaranteedFinalizer(cb func()) (*Weak, error) {
	if cb == nil {
		return nil, fmt.Errorf("gov8: weak finalizer required")
	}
	return g.newWeak(nil, cb)
}

func (g *Global) newWeak(regular WeakFinalizer, guaranteed func()) (*Weak, error) {
	if err := g.check(); err != nil {
		return nil, err
	}
	if isolateIsShared(g.iso) {
		return nil, fmt.Errorf("gov8: weak handles are not supported on shared isolates")
	}
	if err := ensureWeakDispatcher(); err != nil {
		return nil, err
	}
	var id int64
	if regular != nil || guaranteed != nil {
		weakRegistry.mu.Lock()
		weakRegistry.next++
		id = weakRegistry.next
		weakRegistry.mu.Unlock()
	}
	wrap, err := callHandle("Weak.New", proc("gov8_weak_new"),
		g.iso.handle, g.cell, uintptr(id))
	if err != nil {
		removeWeakEntry(id)
		return nil, err
	}
	w := &Weak{iso: g.iso, wrap: wrap, id: id}
	registerWeakLiveness(w)
	if id != 0 {
		weakRegistry.mu.Lock()
		weakRegistry.entries[id] = &weakEntry{
			iso: g.iso, weak: w, wrap: wrap, regular: regular, guaranteed: guaranteed,
		}
		weakRegistry.mu.Unlock()
	}
	return w, nil
}

// DrainGuaranteedWeakFinalizers runs every guaranteed finalizer registered
// on the isolate that has not run yet (its object was never collected, or
// collection was never forced). The pinned crate guarantees these callbacks
// "run before the isolate is destroyed" by draining its finalizer map
// inside OwnedIsolate::Drop; Go has no destructor hook for Isolate.Close,
// so the drain is this explicit, deterministic call: invoke it on the
// owning thread after the last engine work and before Isolate.Close. The
// documented guarantee — the callback runs before the isolate is destroyed
// — is preserved. Drained weaks are consumed (further use reports the
// usual closed errors). It is safe to call when nothing is pending.
func DrainGuaranteedWeakFinalizers(i *Isolate) error {
	if err := i.check(); err != nil {
		return err
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	type pending struct {
		id   int64
		wrap uintptr
		cb   func()
		w    *Weak
	}
	weakRegistry.mu.Lock()
	var ps []pending
	for id, e := range weakRegistry.entries {
		if e.iso == i && e.guaranteed != nil {
			ps = append(ps, pending{id: id, wrap: e.wrap, cb: e.guaranteed, w: e.weak})
		}
	}
	weakRegistry.mu.Unlock()
	for _, p := range ps {
		if err := callErr("DrainGuaranteedWeakFinalizers",
			proc("gov8_weak_drain_guaranteed"), i.handle, p.wrap); err != nil {
			return err
		}
		removeWeakEntry(p.id)
		if p.w != nil {
			unregisterWeakLiveness(p.w)
			p.w.closed = true
			p.w.wrap = 0
			p.w.id = 0
		}
		p.cb()
	}
	return nil
}

// EmptyWeak creates a new empty weak handle, identical to one whose object
// was already collected (Weak::empty).
func (i *Isolate) EmptyWeak() (*Weak, error) {
	if err := i.check(); err != nil {
		return nil, err
	}
	return &Weak{iso: i, empty: true}, nil
}

// check validates weak state and isolate affinity. Post-dispose reads are
// handled by the callers (the pinned crate reports empty/false there).
func (w *Weak) check() error {
	if w == nil {
		return fmt.Errorf("gov8: nil weak")
	}
	if w.closed {
		return fmt.Errorf("gov8: weak used after Close")
	}
	return w.iso.check()
}

// collected reports whether the engine cleared the weak cell. It reads the
// shim-side flag (set by the first-pass weak callback on the owning
// thread), so no engine state is touched.
func (w *Weak) collected() (bool, error) {
	if w.empty {
		return true, nil
	}
	r1, _, _ := proc("gov8_weak_is_empty").Call(w.iso.handle, w.wrap)
	if int64(r1) < 0 {
		return false, shimError("Weak.IsEmpty", r1)
	}
	return r1 == 1, nil
}

// IsEmpty reports whether the object was collected (or the weak is empty).
// After the host isolate was closed this reports true without touching the
// engine, matching the pinned crate.
func (w *Weak) IsEmpty() (bool, error) {
	if err := w.check(); err != nil {
		return false, err
	}
	if isolateClosed(w.iso) {
		return true, nil
	}
	return w.collected()
}

// ToLocal reopens the weak's object as a scope-local value; ok is false
// when the object was collected.
func (w *Weak) ToLocal(s *Scope) (Value, bool, error) {
	if err := w.check(); err != nil {
		return Value{}, false, err
	}
	if s.iso != w.iso {
		return Value{}, false, foreignIsolate("scope")
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return Value{}, false, err
	}
	if w.empty || isolateClosed(w.iso) {
		return Value{}, false, nil
	}
	var out uintptr
	r1, _, _ := proc("gov8_weak_to_local").Call(
		w.iso.handle, sh, w.wrap, uintptr(unsafe.Pointer(&out)))
	switch {
	case int64(r1) < 0:
		return Value{}, false, shimError("Weak.ToLocal", r1)
	case r1 == 1:
		return Value{}, false, nil
	}
	return Value{iso: w.iso, sc: s, h: out}, true, nil
}

// ToGlobal creates a strong global over the weak's object; ok is false when
// the object was collected.
func (w *Weak) ToGlobal() (*Global, bool, error) {
	if err := w.check(); err != nil {
		return nil, false, err
	}
	if w.empty || isolateClosed(w.iso) {
		return nil, false, nil
	}
	cell, _, _ := proc("gov8_weak_to_global").Call(w.iso.handle, w.wrap)
	if cell == 0 {
		return nil, false, nil
	}
	return &Global{iso: w.iso, cell: cell}, true, nil
}

// Clone creates a new weak handle over the same object without a finalizer
// (Weak::clone). A clone of an empty/collected weak is an empty weak.
func (w *Weak) Clone() (*Weak, error) {
	if err := w.check(); err != nil {
		return nil, err
	}
	if w.empty {
		return &Weak{iso: w.iso, empty: true}, nil
	}
	if isolateClosed(w.iso) {
		return nil, fmt.Errorf("gov8: weak used after isolate Close")
	}
	r1, _, _ := proc("gov8_weak_clone").Call(w.iso.handle, w.wrap)
	if int64(r1) < 0 {
		return nil, shimError("Weak.Clone", r1)
	}
	if r1 == 0 {
		return &Weak{iso: w.iso, empty: true}, nil
	}
	clone := &Weak{iso: w.iso, wrap: r1}
	registerWeakLiveness(clone)
	return clone, nil
}

// EqualWeak reports weak-to-weak equality with the pinned crate's
// semantics: same-isolate weaks compare by object identity, two collected
// weaks compare equal, and a collected weak equals nothing live.
func (w *Weak) EqualWeak(other *Weak) (bool, error) {
	if other == nil {
		return false, fmt.Errorf("gov8: cannot compare a weak with nil")
	}
	if w.iso != other.iso {
		return false, nil
	}
	if err := w.check(); err != nil {
		return false, err
	}
	if err := other.check(); err != nil {
		return false, err
	}
	if isolateClosed(w.iso) {
		return false, nil
	}
	aCollected, err := w.collected()
	if err != nil {
		return false, err
	}
	bCollected, err := other.collected()
	if err != nil {
		return false, err
	}
	if aCollected != bCollected {
		return false, nil
	}
	if aCollected {
		return true, nil
	}
	cellA, _, _ := proc("gov8_weak_cell").Call(w.iso.handle, w.wrap)
	cellB, _, _ := proc("gov8_weak_cell").Call(w.iso.handle, other.wrap)
	if cellA == 0 || cellB == 0 {
		return false, nil
	}
	r1, _, _ := proc("gov8_data_eq").Call(cellA, cellB)
	if int64(r1) < 0 {
		return false, shimError("Weak.EqualWeak", r1)
	}
	return r1 == 1, nil
}

// EqualGlobal reports weak-to-global equality: true while the weak holds
// the same object as the global, false once the object was collected.
func (w *Weak) EqualGlobal(g *Global) (bool, error) {
	if g == nil {
		return false, fmt.Errorf("gov8: cannot compare a weak with nil")
	}
	if w.iso != g.iso {
		return false, nil
	}
	if err := w.check(); err != nil {
		return false, err
	}
	if err := g.check(); err != nil {
		return false, err
	}
	if isolateClosed(w.iso) {
		return false, nil
	}
	collected, err := w.collected()
	if err != nil {
		return false, err
	}
	if collected {
		return false, nil
	}
	cell, _, _ := proc("gov8_weak_cell").Call(w.iso.handle, w.wrap)
	if cell == 0 {
		return false, nil
	}
	r1, _, _ := proc("gov8_data_eq").Call(cell, g.cell)
	if int64(r1) < 0 {
		return false, shimError("Weak.EqualGlobal", r1)
	}
	return r1 == 1, nil
}

// Close drops the weak handle. While the object is still strongly held this
// resets the underlying weak cell and cancels the pending finalizer (the
// callback never fires, even after forced GCs or isolate teardown). When
// collection already happened but the finalizer has not run, the callback
// is cancelled here too. After the host isolate was closed this is a
// silent no-op.
func (w *Weak) Close() error {
	if w == nil {
		return fmt.Errorf("gov8: nil weak")
	}
	if w.closed {
		return fmt.Errorf("gov8: weak already closed")
	}
	if w.empty || isolateClosed(w.iso) {
		unregisterWeakLiveness(w)
		w.closed = true
		removeWeakEntry(w.id)
		return nil
	}
	if err := w.iso.check(); err != nil {
		return err
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	r1, _, _ := proc("gov8_weak_close").Call(w.iso.handle, w.wrap)
	w.closed = true
	switch {
	case int64(r1) == 1:
		removeWeakEntry(w.id)
	case int64(r1) < 0:
		return shimError("Weak.Close", r1)
	}
	unregisterWeakLiveness(w)
	return nil
}
