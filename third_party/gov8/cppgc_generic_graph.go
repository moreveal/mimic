//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"
	"unsafe"
)

// CppGCGenericGraphCallbacks controls copied typed-state ownership. Clone is
// required and must return a deep copy: it prevents callers from retaining a
// mutable alias to state owned by the managed object. Drop receives each
// logical state installed by construction or ReplaceState exactly once,
// synchronously on replacement and during final cppgc destruction. UpdateState
// mutates the same logical state and therefore does not end its lifetime.
// NameObserved and final Drop/Destroy may run on a GC
// worker; callbacks must be concurrency-safe and must not call V8. TraceObserved
// is invoked whenever cppgc traces the graph object. Callback panics fail fast
// because they cannot unwind through cppgc.
type CppGCGenericGraphCallbacks[T any] struct {
	Clone         func(T) (T, error)
	Drop          func(T)
	NameObserved  func()
	TraceObserved func()
	Destroy       func()
}

// CppGCGenericGraphOptions configures copied typed state and declarative
// traced slots. Slot counts are fixed for the object's lifetime; any number
// fitting uint32 is supported. Name is copied and must not contain NUL.
// Traced may be zero for an initially empty V8 traced reference.
type CppGCGenericGraphOptions[T any] struct {
	State       T
	Name        string
	StrongSlots uint32
	WeakSlots   uint32
	Traced      Value
	Callbacks   CppGCGenericGraphCallbacks[T]
}

// CppGCGenericGraphObservation is a copied edge observation. State is cloned
// through the target's codec and contains no borrowed managed pointer.
type CppGCGenericGraphObservation[T any] struct {
	State T
}

type cppgcGraphState interface {
	graphID() uint64
	drop()
}

type cppgcTypedGraphState[T any] struct {
	owner uint64
	value T
	clone func(T) (T, error)
	dropf func(T)
	once  sync.Once
}

func (state *cppgcTypedGraphState[T]) graphID() uint64 { return state.owner }
func (state *cppgcTypedGraphState[T]) drop() {
	state.once.Do(func() {
		if state.dropf != nil {
			state.dropf(state.value)
		}
	})
}

type cppgcGraphLifecycle struct {
	mu          sync.Mutex
	iso         *Isolate
	root        *cppgcPersistentHandle
	graphID     uint64
	stateID     uint64
	strongSlots uint32
	weakSlots   uint32
	active      bool
	closed      bool
	destroyed   bool
}

type cppgcGraphEntry struct {
	life          *cppgcGraphLifecycle
	nameObserved  func()
	traceObserved func()
	destroy       func()
}

var cppgcGraphRegistry = struct {
	sync.Mutex
	next    uint64
	entries map[uint64]*cppgcGraphEntry
}{entries: make(map[uint64]*cppgcGraphEntry)}

var cppgcGraphStateRegistry = struct {
	sync.Mutex
	next    uint64
	entries map[uint64]cppgcGraphState
}{entries: make(map[uint64]cppgcGraphState)}

const (
	cppgcGraphStateDrop = iota + 1
	cppgcGraphName
	cppgcGraphDestroy
	cppgcGraphTrace
)

func nextCppGCGraphID(next *uint64, occupied func(uint64) bool) (uint64, error) {
	for attempts := uint64(0); attempts < math.MaxUint64; attempts++ {
		*next++
		id := *next
		if id != 0 && !occupied(id) {
			return id, nil
		}
	}
	return 0, errors.New("gov8: cppgc generic graph registry exhausted")
}

func registerCppGCGraph(entry *cppgcGraphEntry) (uint64, error) {
	cppgcGraphRegistry.Lock()
	defer cppgcGraphRegistry.Unlock()
	id, err := nextCppGCGraphID(&cppgcGraphRegistry.next, func(id uint64) bool {
		return cppgcGraphRegistry.entries[id] != nil
	})
	if err == nil {
		cppgcGraphRegistry.entries[id] = entry
	}
	return id, err
}

func registerCppGCGraphState[T any](graphID uint64, value T, clone func(T) (T, error), drop func(T)) (uint64, error) {
	cppgcGraphStateRegistry.Lock()
	defer cppgcGraphStateRegistry.Unlock()
	id, err := nextCppGCGraphID(&cppgcGraphStateRegistry.next, func(id uint64) bool {
		return cppgcGraphStateRegistry.entries[id] != nil
	})
	if err == nil {
		cppgcGraphStateRegistry.entries[id] = &cppgcTypedGraphState[T]{
			owner: graphID, value: value, clone: clone, dropf: drop,
		}
	}
	return id, err
}

func discardCppGCGraphState(id uint64) {
	cppgcGraphStateRegistry.Lock()
	state := cppgcGraphStateRegistry.entries[id]
	delete(cppgcGraphStateRegistry.entries, id)
	cppgcGraphStateRegistry.Unlock()
	if state != nil {
		state.drop()
	}
}

func dropCppGCGraphState(graphID, stateID uint64) {
	cppgcGraphStateRegistry.Lock()
	state := cppgcGraphStateRegistry.entries[stateID]
	if state != nil && state.graphID() == graphID {
		delete(cppgcGraphStateRegistry.entries, stateID)
	} else {
		state = nil
	}
	cppgcGraphStateRegistry.Unlock()
	if state == nil {
		fatalHostMisuse("gov8: cppgc graph state drop for unknown token %d", stateID)
	}
	state.drop()
}

func cppgcGraphStateValue[T any](graphID, stateID uint64) (T, error) {
	var zero T
	cppgcGraphStateRegistry.Lock()
	state, ok := cppgcGraphStateRegistry.entries[stateID].(*cppgcTypedGraphState[T])
	if !ok || state.owner != graphID {
		cppgcGraphStateRegistry.Unlock()
		return zero, errors.New("gov8: cppgc graph state token is stale or has a different type")
	}
	value := state.value
	clone := state.clone
	cppgcGraphStateRegistry.Unlock()
	return clone(value)
}

func updateCppGCGraphState[T any](graphID, stateID uint64, value T) error {
	cppgcGraphStateRegistry.Lock()
	defer cppgcGraphStateRegistry.Unlock()
	state, ok := cppgcGraphStateRegistry.entries[stateID].(*cppgcTypedGraphState[T])
	if !ok || state.owner != graphID {
		return errors.New("gov8: cppgc graph state token is stale or has a different type")
	}
	state.value = value
	return nil
}

var goCppGCGraphDispatch = syscall.NewCallback(func(graphWord, kindWord, stateWord uintptr) uintptr {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in generic cppgc graph callback: %v\n", recovered)
			proc("gov8_host_panic_abort").Call()
			fatalHostMisuse("gov8: cppgc graph panic abort unexpectedly returned")
		}
	}()
	graphID := uint64(graphWord)
	cppgcGraphRegistry.Lock()
	entry := cppgcGraphRegistry.entries[graphID]
	if kindWord == cppgcGraphDestroy && entry != nil {
		delete(cppgcGraphRegistry.entries, graphID)
	}
	cppgcGraphRegistry.Unlock()
	if entry == nil || entry.life == nil {
		fatalHostMisuse("gov8: generic cppgc graph callback for unknown ID %d", graphID)
	}
	switch kindWord {
	case cppgcGraphStateDrop:
		dropCppGCGraphState(graphID, uint64(stateWord))
	case cppgcGraphName:
		entry.life.mu.Lock()
		if entry.life.active || entry.life.destroyed {
			entry.life.mu.Unlock()
			fatalHostMisuse("gov8: generic cppgc graph name callback during invalid lifecycle state")
		}
		entry.life.active = true
		entry.life.mu.Unlock()
		defer func() {
			entry.life.mu.Lock()
			entry.life.active = false
			entry.life.mu.Unlock()
		}()
		if entry.nameObserved != nil {
			entry.nameObserved()
		}
	case cppgcGraphDestroy:
		entry.life.mu.Lock()
		entry.life.destroyed = true
		entry.life.stateID = 0
		entry.life.mu.Unlock()
		if entry.destroy != nil {
			entry.destroy()
		}
	case cppgcGraphTrace:
		if entry.traceObserved != nil {
			entry.traceObserved()
		}
	default:
		fatalHostMisuse("gov8: invalid generic cppgc graph callback kind %d", kindWord)
	}
	return 1
})

// CppGCGenericGraph is a strong root for one typed-state cppgc graph object.
// The native object stores only numeric state/graph IDs and native tracing
// fields. All operations are isolate-owner-thread-only.
type CppGCGenericGraph[T any] struct {
	life  *cppgcGraphLifecycle
	clone func(T) (T, error)
	drop  func(T)
}

// NewCppGCGenericGraph creates a managed typed-state graph object.
func NewCppGCGenericGraph[T any](iso *Isolate, scope *Scope, options CppGCGenericGraphOptions[T]) (*CppGCGenericGraph[T], error) {
	if iso == nil || scope == nil {
		return nil, errors.New("gov8: nil isolate or scope for generic cppgc graph")
	}
	if err := iso.check(); err != nil {
		return nil, err
	}
	if scope.iso != iso {
		return nil, foreignIsolate("generic cppgc graph scope")
	}
	sh, err := scope.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	if err := scope.requireCurrent(); err != nil {
		return nil, err
	}
	if options.Callbacks.Clone == nil {
		return nil, errors.New("gov8: generic cppgc graph Clone callback is required")
	}
	if !utf8.ValidString(options.Name) || strings.IndexByte(options.Name, 0) >= 0 {
		return nil, errors.New("gov8: generic cppgc graph name must be valid NUL-free UTF-8")
	}
	if len(options.Name) > math.MaxInt32 {
		return nil, errors.New("gov8: generic cppgc graph name exceeds int32")
	}
	if options.Traced.h != 0 {
		if err := options.Traced.check(); err != nil {
			return nil, err
		}
		if options.Traced.iso != iso {
			return nil, foreignIsolate("generic cppgc graph traced value")
		}
	}
	initial, err := options.Callbacks.Clone(options.State)
	if err != nil {
		return nil, fmt.Errorf("gov8: clone initial generic cppgc graph state: %w", err)
	}
	life := &cppgcGraphLifecycle{
		iso: iso, strongSlots: options.StrongSlots, weakSlots: options.WeakSlots,
	}
	entry := &cppgcGraphEntry{
		life: life, nameObserved: options.Callbacks.NameObserved,
		traceObserved: options.Callbacks.TraceObserved,
		destroy:       options.Callbacks.Destroy,
	}
	graphID, err := registerCppGCGraph(entry)
	if err != nil {
		if options.Callbacks.Drop != nil {
			options.Callbacks.Drop(initial)
		}
		return nil, err
	}
	life.graphID = graphID
	stateID, err := registerCppGCGraphState(graphID, initial, options.Callbacks.Clone, options.Callbacks.Drop)
	if err != nil {
		cppgcGraphRegistry.Lock()
		delete(cppgcGraphRegistry.entries, graphID)
		cppgcGraphRegistry.Unlock()
		if options.Callbacks.Drop != nil {
			options.Callbacks.Drop(initial)
		}
		return nil, err
	}
	life.stateID = stateID
	objectRegistryID, err := registerCppGCObject(iso, CppGCObjectCallbacks{})
	if err != nil {
		discardCppGCGraphState(stateID)
		cppgcGraphRegistry.Lock()
		delete(cppgcGraphRegistry.entries, graphID)
		cppgcGraphRegistry.Unlock()
		return nil, err
	}
	name := []byte(options.Name)
	var namePointer uintptr
	if len(name) != 0 {
		namePointer = uintptr(unsafe.Pointer(&name[0]))
	}
	var root uintptr
	var consumed int32
	r1, _, _ := proc("gov8_cppgc_graph_new").Call(
		iso.handleAssumingCheck(), sh, uintptr(objectRegistryID), uintptr(graphID),
		uintptr(stateID), uintptr(options.StrongSlots), uintptr(options.WeakSlots),
		namePointer, uintptr(len(name)), options.Traced.h, goCppGCDispatch,
		goCppGCGraphDispatch, uintptr(unsafe.Pointer(&root)), uintptr(unsafe.Pointer(&consumed)))
	runtime.KeepAlive(name)
	if int64(r1) < 0 {
		if consumed == 0 {
			dropCppGCRegistration(objectRegistryID)
			discardCppGCGraphState(stateID)
			cppgcGraphRegistry.Lock()
			delete(cppgcGraphRegistry.entries, graphID)
			cppgcGraphRegistry.Unlock()
		}
		return nil, shimError("NewCppGCGenericGraph", r1)
	}
	if root == 0 || consumed != 1 {
		if consumed == 0 {
			dropCppGCRegistration(objectRegistryID)
			discardCppGCGraphState(stateID)
			cppgcGraphRegistry.Lock()
			delete(cppgcGraphRegistry.entries, graphID)
			cppgcGraphRegistry.Unlock()
		}
		return nil, errors.New("gov8: generic cppgc graph constructor returned invalid ownership state")
	}
	persistent := &cppgcPersistentHandle{iso: iso, handle: root}
	registerCppGCPersistentLifecycle(persistent)
	life.root = persistent
	return &CppGCGenericGraph[T]{life: life, clone: options.Callbacks.Clone, drop: options.Callbacks.Drop}, nil
}

func (graph *CppGCGenericGraph[T]) begin(operation string) (*cppgcGraphLifecycle, uintptr, error) {
	if graph == nil || graph.life == nil || graph.life.iso == nil {
		return nil, 0, errors.New("gov8: nil generic cppgc graph")
	}
	life := graph.life
	if err := life.iso.check(); err != nil {
		return nil, 0, err
	}
	life.mu.Lock()
	if life.closed || life.destroyed || life.root == nil {
		life.mu.Unlock()
		return nil, 0, fmt.Errorf("gov8: generic cppgc graph %s after Close or destruction", operation)
	}
	if life.active {
		life.mu.Unlock()
		return nil, 0, fmt.Errorf("gov8: generic cppgc graph %s during active operation", operation)
	}
	life.active = true
	root := life.root
	life.mu.Unlock()
	root.mu.Lock()
	handle := root.handle
	closed := root.closed || handle == 0
	root.mu.Unlock()
	if closed {
		graph.end(life)
		return nil, 0, fmt.Errorf("gov8: generic cppgc graph %s after Close", operation)
	}
	return life, handle, nil
}

func (graph *CppGCGenericGraph[T]) end(life *cppgcGraphLifecycle) {
	life.mu.Lock()
	life.active = false
	life.mu.Unlock()
}

// State returns a deep clone of the currently managed typed state.
func (graph *CppGCGenericGraph[T]) State() (T, error) {
	var zero T
	life, _, err := graph.begin("State")
	if err != nil {
		return zero, err
	}
	defer graph.end(life)
	life.mu.Lock()
	stateID := life.stateID
	graphID := life.graphID
	life.mu.Unlock()
	return cppgcGraphStateValue[T](graphID, stateID)
}

func (graph *CppGCGenericGraph[T]) installState(operation string, value T) error {
	life, root, err := graph.begin(operation)
	if err != nil {
		return err
	}
	defer graph.end(life)
	owned, err := graph.clone(value)
	if err != nil {
		return fmt.Errorf("gov8: clone generic cppgc graph state: %w", err)
	}
	stateID, err := registerCppGCGraphState(life.graphID, owned, graph.clone, graph.drop)
	if err != nil {
		if graph.drop != nil {
			graph.drop(owned)
		}
		return err
	}
	r1, _, _ := proc("gov8_cppgc_graph_state_replace").Call(
		root, life.iso.handleAssumingCheck(), uintptr(stateID))
	if int64(r1) < 0 {
		discardCppGCGraphState(stateID)
		return shimError("CppGCGenericGraph.ReplaceState", r1)
	}
	life.mu.Lock()
	life.stateID = stateID
	life.mu.Unlock()
	return nil
}

// ReplaceState deep-clones value, atomically installs the managed copy, and
// synchronously drops the old managed state before returning.
func (graph *CppGCGenericGraph[T]) ReplaceState(value T) error {
	return graph.installState("ReplaceState", value)
}

// UpdateState clones the current value, lets update modify only that detached
// copy, then writes a managed clone back into the same stable state entry. It
// models an in-place GcCell mutation: Drop is not called for the prior value.
// A returned error leaves managed state unchanged.
func (graph *CppGCGenericGraph[T]) UpdateState(update func(*T) error) error {
	if update == nil {
		return errors.New("gov8: generic cppgc graph update callback is required")
	}
	life, _, err := graph.begin("UpdateState")
	if err != nil {
		return err
	}
	defer graph.end(life)
	life.mu.Lock()
	stateID := life.stateID
	graphID := life.graphID
	life.mu.Unlock()
	current, err := cppgcGraphStateValue[T](graphID, stateID)
	if err != nil {
		return err
	}
	if err := update(&current); err != nil {
		return err
	}
	owned, err := graph.clone(current)
	if err != nil {
		return fmt.Errorf("gov8: clone updated generic cppgc graph state: %w", err)
	}
	if err := updateCppGCGraphState(graphID, stateID, owned); err != nil {
		if graph.drop != nil {
			graph.drop(owned)
		}
		return err
	}
	return nil
}

func (graph *CppGCGenericGraph[T]) edgeSet(index uint32, child *CppGCGenericGraph[T], weak bool) error {
	if child == nil || child.life == nil {
		return errors.New("gov8: nil generic cppgc graph child")
	}
	if graph == nil || graph.life == nil {
		return errors.New("gov8: nil generic cppgc graph owner")
	}
	if graph.life.iso != child.life.iso {
		return foreignIsolate("generic cppgc graph child")
	}
	ownerLife, ownerRoot, err := graph.begin("SetEdge")
	if err != nil {
		return err
	}
	defer graph.end(ownerLife)
	limit := ownerLife.strongSlots
	if weak {
		limit = ownerLife.weakSlots
	}
	if index >= limit {
		return fmt.Errorf("gov8: generic cppgc graph edge index %d out of bounds %d", index, limit)
	}
	childRoot := ownerRoot
	var childLife *cppgcGraphLifecycle
	if child != graph {
		childLife, childRoot, err = child.begin("edge child")
		if err != nil {
			return err
		}
		defer child.end(childLife)
	}
	return callErr("CppGCGenericGraph.SetEdge", proc("gov8_cppgc_graph_edge_set"),
		ownerRoot, childRoot, ownerLife.iso.handleAssumingCheck(), boolWord(weak), uintptr(index))
}

// SetStrong assigns one indexed strong edge using cppgc's write barrier.
func (graph *CppGCGenericGraph[T]) SetStrong(index uint32, child *CppGCGenericGraph[T]) error {
	return graph.edgeSet(index, child, false)
}

// SetWeak assigns one indexed weak edge using cppgc's write barrier.
func (graph *CppGCGenericGraph[T]) SetWeak(index uint32, child *CppGCGenericGraph[T]) error {
	return graph.edgeSet(index, child, true)
}

func (graph *CppGCGenericGraph[T]) edgeClear(index uint32, weak bool) error {
	life, root, err := graph.begin("ClearEdge")
	if err != nil {
		return err
	}
	defer graph.end(life)
	limit := life.strongSlots
	if weak {
		limit = life.weakSlots
	}
	if index >= limit {
		return fmt.Errorf("gov8: generic cppgc graph edge index %d out of bounds %d", index, limit)
	}
	return callErr("CppGCGenericGraph.ClearEdge", proc("gov8_cppgc_graph_edge_clear"),
		root, life.iso.handleAssumingCheck(), boolWord(weak), uintptr(index))
}

func (graph *CppGCGenericGraph[T]) ClearStrong(index uint32) error {
	return graph.edgeClear(index, false)
}
func (graph *CppGCGenericGraph[T]) ClearWeak(index uint32) error {
	return graph.edgeClear(index, true)
}

func (graph *CppGCGenericGraph[T]) edge(index uint32, weak bool) (CppGCGenericGraphObservation[T], bool, error) {
	var observation CppGCGenericGraphObservation[T]
	life, root, err := graph.begin("Edge")
	if err != nil {
		return observation, false, err
	}
	defer graph.end(life)
	limit := life.strongSlots
	if weak {
		limit = life.weakSlots
	}
	if index >= limit {
		return observation, false, fmt.Errorf("gov8: generic cppgc graph edge index %d out of bounds %d", index, limit)
	}
	var graphID, stateID uint64
	var present int32
	r1, _, _ := proc("gov8_cppgc_graph_edge_get").Call(
		root, life.iso.handleAssumingCheck(), boolWord(weak), uintptr(index),
		uintptr(unsafe.Pointer(&graphID)), uintptr(unsafe.Pointer(&stateID)),
		uintptr(unsafe.Pointer(&present)))
	if int64(r1) < 0 {
		return observation, false, shimError("CppGCGenericGraph.Edge", r1)
	}
	if present == 0 {
		return observation, false, nil
	}
	if present != 1 || graphID == 0 || stateID == 0 {
		return observation, false, errors.New("gov8: generic cppgc graph returned invalid edge metadata")
	}
	value, err := cppgcGraphStateValue[T](graphID, stateID)
	if err != nil {
		return observation, false, err
	}
	observation.State = value
	return observation, true, nil
}

func (graph *CppGCGenericGraph[T]) Strong(index uint32) (CppGCGenericGraphObservation[T], bool, error) {
	return graph.edge(index, false)
}
func (graph *CppGCGenericGraph[T]) Weak(index uint32) (CppGCGenericGraphObservation[T], bool, error) {
	return graph.edge(index, true)
}

// SetTraced replaces the embedded V8 traced reference. A zero Value clears it.
func (graph *CppGCGenericGraph[T]) SetTraced(scope *Scope, value Value) error {
	life, root, err := graph.begin("SetTraced")
	if err != nil {
		return err
	}
	defer graph.end(life)
	if scope == nil || scope.iso != life.iso {
		return foreignIsolate("generic cppgc graph traced scope")
	}
	sh, err := scope.checkedHandleAssumingIsolate()
	if err != nil {
		return err
	}
	if err := scope.requireCurrent(); err != nil {
		return err
	}
	if value.h != 0 {
		if err := value.check(); err != nil {
			return err
		}
		if value.iso != life.iso {
			return foreignIsolate("generic cppgc graph traced value")
		}
	}
	return callErr("CppGCGenericGraph.SetTraced", proc("gov8_cppgc_graph_traced_set"),
		root, life.iso.handleAssumingCheck(), sh, value.h)
}

// Traced copies the embedded traced reference into scope, or returns ok=false
// when it is empty.
func (graph *CppGCGenericGraph[T]) Traced(scope *Scope) (Value, bool, error) {
	life, root, err := graph.begin("Traced")
	if err != nil {
		return Value{}, false, err
	}
	defer graph.end(life)
	if scope == nil || scope.iso != life.iso {
		return Value{}, false, foreignIsolate("generic cppgc graph traced scope")
	}
	sh, err := scope.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, false, err
	}
	if err := scope.requireCurrent(); err != nil {
		return Value{}, false, err
	}
	var wire uintptr
	var present int32
	r1, _, _ := proc("gov8_cppgc_graph_traced_get").Call(
		root, life.iso.handleAssumingCheck(), sh, uintptr(unsafe.Pointer(&wire)),
		uintptr(unsafe.Pointer(&present)))
	if int64(r1) < 0 {
		return Value{}, false, shimError("CppGCGenericGraph.Traced", r1)
	}
	if present == 0 {
		return Value{}, false, nil
	}
	if present != 1 || wire == 0 {
		return Value{}, false, errors.New("gov8: generic cppgc graph returned invalid traced value")
	}
	return Value{iso: life.iso, sc: scope, h: wire}, true, nil
}

// Close releases this graph's strong off-heap root. The managed object remains
// alive while reached by another graph edge; final state Drop and Destroy run
// only when cppgc later determines it is unreachable.
func (graph *CppGCGenericGraph[T]) Close() error {
	if graph == nil || graph.life == nil {
		return nil
	}
	life := graph.life
	if err := life.iso.check(); err != nil {
		life.mu.Lock()
		closed := life.closed
		life.mu.Unlock()
		if closed {
			return nil
		}
		return err
	}
	life.mu.Lock()
	if life.closed {
		life.mu.Unlock()
		return nil
	}
	if life.active {
		life.mu.Unlock()
		return errors.New("gov8: generic cppgc graph Close during active operation")
	}
	life.closed = true
	root := life.root
	life.mu.Unlock()
	if err := root.close(); err != nil {
		life.mu.Lock()
		life.closed = false
		life.mu.Unlock()
		return err
	}
	return nil
}
