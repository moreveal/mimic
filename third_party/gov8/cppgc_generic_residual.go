//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// CppGCGenericCallbacks observes copied generic-payload events. CellDropped is
// called synchronously when SetCell replaces a value and once more when the
// managed owner is destroyed. NameObserved may run while a heap snapshot is
// visiting the object. Destroy runs once after the allocation's native state
// has been destroyed. Callbacks may run on a GC worker, must be concurrency
// safe, and must not call into V8. Panics fail fast at the native callback
// boundary.
type CppGCGenericCallbacks struct {
	CellDropped  func(int32)
	NameObserved func()
	Destroy      func()
}

// CppGCGenericOptions describes copied state stored in a native cppgc
// allocation. Size and Alignment describe the logical payload layout being
// adapted; the native managed envelope remains private. Alignment must be a
// power of two no greater than cppgc's public limit of 16. Name is copied and
// must not contain NUL.
type CppGCGenericOptions struct {
	ObjectID  int32
	Cell      int32
	Name      string
	Size      uint32
	Alignment uint32
	Callbacks CppGCGenericCallbacks
}

// CppGCGenericLayout is a copied layout/storage observation. AddressAligned
// and CellStorageStable are computed natively without exposing either address.
type CppGCGenericLayout struct {
	Size              uint32
	Alignment         uint32
	AddressAligned    bool
	CellStorageStable bool
}

type cppgcGenericEntry struct {
	iso       *Isolate
	callbacks CppGCGenericCallbacks
}

var cppgcGenericRegistry = struct {
	sync.Mutex
	next    uint64
	entries map[uint64]*cppgcGenericEntry
}{entries: make(map[uint64]*cppgcGenericEntry)}

const (
	cppgcGenericCellDrop = iota + 1
	cppgcGenericName
	cppgcGenericDestroy
)

var goCppGCGenericDispatch = syscall.NewCallback(func(idWord, kindWord, valueWord uintptr) (result uintptr) {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in generic cppgc callback: %v\n", recovered)
			proc("gov8_host_panic_abort").Call()
			fatalHostMisuse("gov8: cppgc generic panic abort unexpectedly returned")
		}
	}()
	id := uint64(idWord)
	cppgcGenericRegistry.Lock()
	entry := cppgcGenericRegistry.entries[id]
	if kindWord == cppgcGenericDestroy && entry != nil {
		delete(cppgcGenericRegistry.entries, id)
	}
	cppgcGenericRegistry.Unlock()
	if entry == nil {
		fatalHostMisuse("gov8: generic cppgc callback for unknown registry ID %d", id)
	}
	switch kindWord {
	case cppgcGenericCellDrop:
		if entry.callbacks.CellDropped != nil {
			entry.callbacks.CellDropped(int32(valueWord))
		}
	case cppgcGenericName:
		if entry.callbacks.NameObserved != nil {
			entry.callbacks.NameObserved()
		}
	case cppgcGenericDestroy:
		if entry.callbacks.Destroy != nil {
			entry.callbacks.Destroy()
		}
	default:
		fatalHostMisuse("gov8: invalid generic cppgc callback kind %d", kindWord)
	}
	return 1
})

func registerCppGCGeneric(iso *Isolate, callbacks CppGCGenericCallbacks) (uint64, error) {
	cppgcGenericRegistry.Lock()
	defer cppgcGenericRegistry.Unlock()
	for attempts := uint64(0); attempts < math.MaxUint64; attempts++ {
		cppgcGenericRegistry.next++
		id := cppgcGenericRegistry.next
		if id != 0 && cppgcGenericRegistry.entries[id] == nil {
			cppgcGenericRegistry.entries[id] = &cppgcGenericEntry{iso: iso, callbacks: callbacks}
			return id, nil
		}
	}
	return 0, errors.New("gov8: generic cppgc callback registry exhausted")
}

func dropCppGCGenericRegistration(id uint64) {
	cppgcGenericRegistry.Lock()
	delete(cppgcGenericRegistry.entries, id)
	cppgcGenericRegistry.Unlock()
}

// CppGCGenericObject is a strong native root for a copied-state cppgc object.
// Close releases the root; final destruction occurs at a later cppgc
// collection. Operations are isolate-owner-thread-only and never expose the
// managed address, GcCell reference, or Visitor.
type CppGCGenericObject struct {
	persistent *cppgcPersistentHandle
	registryID uint64
	genericID  uint64
}

// CollectCppGCGarbageForTesting synchronously collects the isolate's default
// cppgc heap with the explicit embedder-stack state. It is the checked Go form
// of PinScope::get_cpp_heap followed by Heap::collect_garbage_for_testing.
func (i *Isolate) CollectCppGCGarbageForTesting(state CppGCEmbedderStackState) error {
	if i == nil {
		return errors.New("gov8: nil isolate")
	}
	if state > CppGCStackNoHeapPointers {
		return errors.New("gov8: invalid cppgc stack state")
	}
	if err := i.check(); err != nil {
		return err
	}
	return callErr("Isolate.CollectCppGCGarbageForTesting", proc("gov8_cppgc_generic_collect"),
		i.handleAssumingCheck(), uintptr(state))
}

func validCppGCGenericOptions(options CppGCGenericOptions) error {
	if options.Alignment == 0 || options.Alignment > 16 || options.Alignment&(options.Alignment-1) != 0 {
		return fmt.Errorf("gov8: generic cppgc alignment %d must be a power of two no greater than 16", options.Alignment)
	}
	if strings.IndexByte(options.Name, 0) >= 0 {
		return errors.New("gov8: generic cppgc name contains NUL")
	}
	return nil
}

// NewCppGCGenericObject performs allocation, copied-state construction, and
// strong rooting in one native call on the isolate's default cppgc heap.
func (i *Isolate) NewCppGCGenericObject(options CppGCGenericOptions) (*CppGCGenericObject, error) {
	if i == nil {
		return nil, errors.New("gov8: nil isolate")
	}
	if err := i.check(); err != nil {
		return nil, err
	}
	if err := validCppGCGenericOptions(options); err != nil {
		return nil, err
	}
	registryID, err := registerCppGCObject(i, CppGCObjectCallbacks{})
	if err != nil {
		return nil, err
	}
	genericID, err := registerCppGCGeneric(i, options.Callbacks)
	if err != nil {
		dropCppGCRegistration(registryID)
		return nil, err
	}
	var namePointer uintptr
	nameBytes := []byte(options.Name)
	if len(nameBytes) != 0 {
		namePointer = uintptr(unsafe.Pointer(&nameBytes[0]))
	}
	var root uintptr
	var consumed int32
	r1, _, _ := proc("gov8_cppgc_generic_new").Call(
		i.handleAssumingCheck(), uintptr(registryID), uintptr(genericID),
		uintptr(options.ObjectID), uintptr(options.Cell), uintptr(options.Size),
		uintptr(options.Alignment), namePointer, uintptr(len(nameBytes)),
		goCppGCDispatch, goCppGCGenericDispatch, uintptr(unsafe.Pointer(&root)),
		uintptr(unsafe.Pointer(&consumed)))
	if int64(r1) < 0 {
		if consumed == 0 {
			dropCppGCGenericRegistration(genericID)
			dropCppGCRegistration(registryID)
		}
		return nil, shimError("Isolate.NewCppGCGenericObject", r1)
	}
	if root == 0 || consumed != 1 {
		return nil, errors.New("gov8: generic cppgc constructor returned invalid ownership state")
	}
	persistent := &cppgcPersistentHandle{iso: i, handle: root}
	registerCppGCPersistentLifecycle(persistent)
	return &CppGCGenericObject{persistent: persistent, registryID: registryID, genericID: genericID}, nil
}

func (object *CppGCGenericObject) withRoot(operation string, fn func(*cppgcPersistentHandle) error) error {
	if object == nil || object.persistent == nil {
		return errors.New("gov8: nil generic cppgc object")
	}
	handle := object.persistent
	if err := handle.iso.check(); err != nil {
		return err
	}
	handle.mu.Lock()
	defer handle.mu.Unlock()
	if handle.closed || handle.handle == 0 {
		return fmt.Errorf("gov8: generic cppgc object %s after Close", operation)
	}
	return fn(handle)
}

// Cell returns a copy of the object's scalar cell value.
func (object *CppGCGenericObject) Cell() (int32, error) {
	var value int32
	err := object.withRoot("Cell", func(handle *cppgcPersistentHandle) error {
		return callErr("CppGCGenericObject.Cell", proc("gov8_cppgc_generic_cell_get"),
			handle.handle, handle.iso.handleAssumingCheck(), uintptr(unsafe.Pointer(&value)))
	})
	return value, err
}

// SetCell replaces the copied scalar. CellDropped observes the replaced value
// synchronously, matching GcCell::set destruction timing.
func (object *CppGCGenericObject) SetCell(value int32) error {
	return object.withRoot("SetCell", func(handle *cppgcPersistentHandle) error {
		return callErr("CppGCGenericObject.SetCell", proc("gov8_cppgc_generic_cell_set"),
			handle.handle, handle.iso.handleAssumingCheck(), uintptr(value))
	})
}

// UpdateCell adds delta in native storage and returns a copied result. Overflow
// is rejected without changing the cell.
func (object *CppGCGenericObject) UpdateCell(delta int32) (int32, error) {
	var value int32
	err := object.withRoot("UpdateCell", func(handle *cppgcPersistentHandle) error {
		return callErr("CppGCGenericObject.UpdateCell", proc("gov8_cppgc_generic_cell_update"),
			handle.handle, handle.iso.handleAssumingCheck(), uintptr(delta),
			uintptr(unsafe.Pointer(&value)))
	})
	return value, err
}

// Layout returns copied logical-layout and native invariant observations.
func (object *CppGCGenericObject) Layout() (CppGCGenericLayout, error) {
	var size, alignment uint32
	var addressAligned, stable int32
	err := object.withRoot("Layout", func(handle *cppgcPersistentHandle) error {
		return callErr("CppGCGenericObject.Layout", proc("gov8_cppgc_generic_layout"),
			handle.handle, handle.iso.handleAssumingCheck(), uintptr(unsafe.Pointer(&size)),
			uintptr(unsafe.Pointer(&alignment)), uintptr(unsafe.Pointer(&addressAligned)),
			uintptr(unsafe.Pointer(&stable)))
	})
	if err != nil {
		return CppGCGenericLayout{}, err
	}
	if (addressAligned != 0 && addressAligned != 1) || (stable != 0 && stable != 1) {
		return CppGCGenericLayout{}, errors.New("gov8: generic cppgc layout returned invalid boolean")
	}
	return CppGCGenericLayout{size, alignment, addressAligned == 1, stable == 1}, nil
}

// SetOptionalMember sets the object's traced Option<Member<T>> equivalent.
// Both objects remain independently rooted until their Close calls.
func (object *CppGCGenericObject) SetOptionalMember(child *CppGCGenericObject) error {
	if object == nil || child == nil || object.persistent == nil || child.persistent == nil {
		return errors.New("gov8: nil generic cppgc member owner or child")
	}
	owner, target := object.persistent, child.persistent
	if owner.iso != target.iso {
		return foreignIsolate("generic cppgc member")
	}
	if err := owner.iso.check(); err != nil {
		return err
	}
	if owner == target {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		if owner.closed || owner.handle == 0 {
			return errors.New("gov8: generic cppgc member used after Close")
		}
		return callErr("CppGCGenericObject.SetOptionalMember", proc("gov8_cppgc_generic_member_set"),
			owner.handle, owner.handle, owner.iso.handleAssumingCheck())
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	target.mu.Lock()
	defer target.mu.Unlock()
	if owner.closed || owner.handle == 0 || target.closed || target.handle == 0 {
		return errors.New("gov8: generic cppgc member used after Close")
	}
	return callErr("CppGCGenericObject.SetOptionalMember", proc("gov8_cppgc_generic_member_set"),
		owner.handle, target.handle, owner.iso.handleAssumingCheck())
}

// ClearOptionalMember assigns None. Collection of the former target remains
// asynchronous until a later cppgc collection.
func (object *CppGCGenericObject) ClearOptionalMember() error {
	if object == nil {
		return errors.New("gov8: nil generic cppgc object")
	}
	return (&CppGCPersistent{persistent: object.persistent}).ClearStrongMember()
}

// OptionalMember returns copied metadata for the traced member.
func (object *CppGCGenericObject) OptionalMember() (CppGCObjectSnapshot, bool, error) {
	if object == nil {
		return CppGCObjectSnapshot{}, false, errors.New("gov8: nil generic cppgc object")
	}
	return (&CppGCPersistent{persistent: object.persistent}).StrongMember()
}

// NewWeakPersistent creates a weak observer initialized from this object.
func (object *CppGCGenericObject) NewWeakPersistent() (*CppGCWeakPersistent, error) {
	if object == nil || object.persistent == nil {
		return nil, errors.New("gov8: nil generic cppgc object")
	}
	weak, err := newEmptyCppGCPersistent(object.persistent.iso, true)
	if err != nil {
		return nil, err
	}
	if err := weak.setFromPersistent(object.persistent); err != nil {
		_ = weak.close()
		return nil, err
	}
	return &CppGCWeakPersistent{persistent: weak}, nil
}

// Close releases the strong root. It is idempotent; callbacks run only when a
// later collection destroys the now-unreachable allocation.
func (object *CppGCGenericObject) Close() error {
	if object == nil || object.persistent == nil {
		return nil
	}
	return object.persistent.close()
}
