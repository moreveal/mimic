//go:build windows && amd64

package gov8

import (
	"fmt"
	"sync"
	"unsafe"
)

// CppGCObjectSnapshot is copied metadata for an object reached through a
// cppgc persistent handle. It contains no native pointer and remains safe to
// inspect after the managed object is collected.
type CppGCObjectSnapshot struct {
	ObjectID int32
	Tag      CppGCTag
}

type cppgcPersistentHandle struct {
	mu     sync.Mutex
	iso    *Isolate
	handle uintptr
	weak   bool
	closed bool
}

var cppgcPersistentLifecycle = struct {
	sync.Mutex
	isolates map[*Isolate]map[*cppgcPersistentHandle]struct{}
}{isolates: make(map[*Isolate]map[*cppgcPersistentHandle]struct{})}

func registerCppGCPersistentLifecycle(handle *cppgcPersistentHandle) {
	cppgcPersistentLifecycle.Lock()
	handles := cppgcPersistentLifecycle.isolates[handle.iso]
	if handles == nil {
		handles = make(map[*cppgcPersistentHandle]struct{})
		cppgcPersistentLifecycle.isolates[handle.iso] = handles
	}
	handles[handle] = struct{}{}
	cppgcPersistentLifecycle.Unlock()
}

func unregisterCppGCPersistentLifecycle(handle *cppgcPersistentHandle) {
	cppgcPersistentLifecycle.Lock()
	handles := cppgcPersistentLifecycle.isolates[handle.iso]
	delete(handles, handle)
	if len(handles) == 0 {
		delete(cppgcPersistentLifecycle.isolates, handle.iso)
	}
	cppgcPersistentLifecycle.Unlock()
}

// afterCppGCPersistentIsolateDispose destroys native handle wrappers after V8
// has disposed the isolate's cppgc heap. The pinned cppgc persistent nodes are
// cleared by heap teardown, so their later same-thread destructors do not
// revisit or redestroy managed targets. disposedHandle is used only as the
// opaque owner identity recorded in each wrapper.
func afterCppGCPersistentIsolateDispose(iso *Isolate, disposedHandle uintptr) error {
	cppgcPersistentLifecycle.Lock()
	registered := cppgcPersistentLifecycle.isolates[iso]
	delete(cppgcPersistentLifecycle.isolates, iso)
	handles := make([]*cppgcPersistentHandle, 0, len(registered))
	for handle := range registered {
		handles = append(handles, handle)
	}
	cppgcPersistentLifecycle.Unlock()
	var firstErr error
	for _, handle := range handles {
		handle.mu.Lock()
		if !handle.closed && handle.handle != 0 {
			r1, _, _ := proc("gov8_cppgc_persistent_close_after_isolate_dispose").Call(
				handle.handle, disposedHandle)
			if int64(r1) < 0 && firstErr == nil {
				firstErr = shimError("CppGCPersistent.IsolateDispose", r1)
			}
			handle.handle = 0
			handle.closed = true
		}
		handle.mu.Unlock()
	}
	return firstErr
}

func newEmptyCppGCPersistent(iso *Isolate, weak bool) (*cppgcPersistentHandle, error) {
	if iso == nil {
		return nil, fmt.Errorf("gov8: nil isolate")
	}
	ih, err := iso.handleChecked()
	if err != nil {
		return nil, err
	}
	var handle uintptr
	r1, _, _ := proc("gov8_cppgc_persistent_empty").Call(
		ih, boolWord(weak), uintptr(unsafe.Pointer(&handle)))
	if int64(r1) < 0 {
		return nil, shimError("CppGCPersistent.Empty", r1)
	}
	if handle == 0 {
		return nil, fmt.Errorf("gov8: cppgc persistent constructor returned null")
	}
	persistent := &cppgcPersistentHandle{iso: iso, handle: handle, weak: weak}
	registerCppGCPersistentLifecycle(persistent)
	return persistent, nil
}

func cppgcObjectPersistentInputs(object *CppGCObject) (iso *Isolate, scopeHandle, wrapperHandle uintptr, tag CppGCTag, registryID uint64, err error) {
	if err = object.check(); err != nil {
		return nil, 0, 0, 0, 0, err
	}
	scope := object.wrapper.sc
	if scope == nil {
		return nil, 0, 0, 0, 0, fmt.Errorf("gov8: cppgc object has no live wrapper scope")
	}
	if err = scope.requireCurrent(); err != nil {
		return nil, 0, 0, 0, 0, err
	}
	scopeHandle, err = scope.checkedHandle()
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	return object.iso, scopeHandle, object.wrapper.h, object.tag, object.registryID, nil
}

func newCppGCPersistentFromObject(object *CppGCObject, weak bool) (*cppgcPersistentHandle, error) {
	iso, scopeHandle, wrapperHandle, tag, registryID, err := cppgcObjectPersistentInputs(object)
	if err != nil {
		return nil, err
	}
	ih, err := iso.handleChecked()
	if err != nil {
		return nil, err
	}
	var handle uintptr
	r1, _, _ := proc("gov8_cppgc_persistent_from_wrapper").Call(
		ih, scopeHandle, wrapperHandle, uintptr(tag), uintptr(registryID),
		boolWord(weak), uintptr(unsafe.Pointer(&handle)))
	if int64(r1) < 0 {
		return nil, shimError("CppGCPersistent.New", r1)
	}
	if handle == 0 {
		return nil, fmt.Errorf("gov8: cppgc persistent constructor returned null")
	}
	persistent := &cppgcPersistentHandle{iso: iso, handle: handle, weak: weak}
	registerCppGCPersistentLifecycle(persistent)
	return persistent, nil
}

func (persistent *cppgcPersistentHandle) set(object *CppGCObject) error {
	if persistent == nil {
		return fmt.Errorf("gov8: nil cppgc persistent handle")
	}
	if err := persistent.iso.check(); err != nil {
		return err
	}
	iso, scopeHandle, wrapperHandle, tag, registryID, err := cppgcObjectPersistentInputs(object)
	if err != nil {
		return err
	}
	if iso != persistent.iso {
		return foreignIsolate("cppgc object")
	}
	persistent.mu.Lock()
	defer persistent.mu.Unlock()
	if persistent.closed || persistent.handle == 0 {
		return fmt.Errorf("gov8: cppgc persistent handle used after Close")
	}
	r1, _, _ := proc("gov8_cppgc_persistent_set").Call(
		persistent.handle, persistent.iso.handleAssumingCheck(), scopeHandle,
		wrapperHandle, uintptr(tag), uintptr(registryID))
	if int64(r1) < 0 {
		return shimError("CppGCPersistent.Set", r1)
	}
	return nil
}

func (persistent *cppgcPersistentHandle) setFromPersistent(source *cppgcPersistentHandle) error {
	if persistent == nil || source == nil {
		return fmt.Errorf("gov8: nil cppgc persistent handle")
	}
	if persistent.iso != source.iso {
		return foreignIsolate("cppgc persistent source")
	}
	if err := persistent.iso.check(); err != nil {
		return err
	}
	if persistent == source {
		persistent.mu.Lock()
		defer persistent.mu.Unlock()
		if persistent.closed || persistent.handle == 0 {
			return fmt.Errorf("gov8: cppgc persistent handle used after Close")
		}
		return nil
	}
	persistent.mu.Lock()
	defer persistent.mu.Unlock()
	source.mu.Lock()
	defer source.mu.Unlock()
	if persistent.closed || persistent.handle == 0 {
		return fmt.Errorf("gov8: cppgc persistent handle used after Close")
	}
	if source.closed || source.handle == 0 {
		return fmt.Errorf("gov8: cppgc persistent source used after Close")
	}
	r1, _, _ := proc("gov8_cppgc_persistent_set_from_persistent").Call(
		persistent.handle, source.handle, persistent.iso.handleAssumingCheck())
	if int64(r1) < 0 {
		return shimError("CppGCPersistent.SetFromPersistent", r1)
	}
	return nil
}

func (persistent *cppgcPersistentHandle) get() (snapshot CppGCObjectSnapshot, registryID uint64, ok bool, err error) {
	if persistent == nil {
		return snapshot, 0, false, fmt.Errorf("gov8: nil cppgc persistent handle")
	}
	if err := persistent.iso.check(); err != nil {
		return snapshot, 0, false, err
	}
	persistent.mu.Lock()
	defer persistent.mu.Unlock()
	if persistent.closed || persistent.handle == 0 {
		return snapshot, 0, false, fmt.Errorf("gov8: cppgc persistent handle used after Close")
	}
	var objectID int32
	var tag uint16
	var present int32
	r1, _, _ := proc("gov8_cppgc_persistent_get").Call(
		persistent.handle, persistent.iso.handleAssumingCheck(),
		uintptr(unsafe.Pointer(&registryID)), uintptr(unsafe.Pointer(&objectID)),
		uintptr(unsafe.Pointer(&tag)), uintptr(unsafe.Pointer(&present)))
	if int64(r1) < 0 {
		return snapshot, 0, false, shimError("CppGCPersistent.Get", r1)
	}
	if present == 0 {
		return snapshot, 0, false, nil
	}
	if registryID == 0 || !liveCppGCRegistration(registryID, persistent.iso) {
		return snapshot, 0, false, fmt.Errorf("gov8: cppgc persistent returned stale metadata")
	}
	return CppGCObjectSnapshot{ObjectID: objectID, Tag: CppGCTag(tag)}, registryID, true, nil
}

func (persistent *cppgcPersistentHandle) matches(object *CppGCObject) (bool, error) {
	if err := object.check(); err != nil {
		return false, err
	}
	if persistent == nil || persistent.iso != object.iso {
		if persistent == nil {
			return false, fmt.Errorf("gov8: nil cppgc persistent handle")
		}
		return false, foreignIsolate("cppgc object")
	}
	_, registryID, ok, err := persistent.get()
	return ok && registryID == object.registryID, err
}

func (persistent *cppgcPersistentHandle) close() error {
	if persistent == nil {
		return nil
	}
	if persistent.iso == nil {
		return nil
	}
	if err := persistent.iso.check(); err != nil {
		// Isolate teardown marks every registered wrapper closed before clearing
		// the isolate handle. A later Close is therefore a safe idempotent no-op.
		persistent.mu.Lock()
		closed := persistent.closed
		persistent.mu.Unlock()
		if closed {
			return nil
		}
		return err
	}
	persistent.mu.Lock()
	defer persistent.mu.Unlock()
	if persistent.closed {
		return nil
	}
	r1, _, _ := proc("gov8_cppgc_persistent_close").Call(
		persistent.handle, persistent.iso.handleAssumingCheck())
	if int64(r1) < 0 {
		return shimError("CppGCPersistent.Close", r1)
	}
	persistent.handle = 0
	persistent.closed = true
	unregisterCppGCPersistentLifecycle(persistent)
	return nil
}

// CppGCPersistent is a strong, native-owned cppgc root. It keeps its managed
// object alive independently of the JavaScript API wrapper. Operations are
// owner-thread-only. Isolate teardown releases an outstanding native handle;
// later Close remains idempotent, while Get and Set report the closed isolate.
type CppGCPersistent struct{ persistent *cppgcPersistentHandle }

// NewEmptyCppGCPersistent creates an empty strong cppgc root.
func NewEmptyCppGCPersistent(iso *Isolate) (*CppGCPersistent, error) {
	persistent, err := newEmptyCppGCPersistent(iso, false)
	if err != nil {
		return nil, err
	}
	return &CppGCPersistent{persistent: persistent}, nil
}

// NewCppGCPersistent creates a strong cppgc root initialized from object.
func NewCppGCPersistent(object *CppGCObject) (*CppGCPersistent, error) {
	persistent, err := newCppGCPersistentFromObject(object, false)
	if err != nil {
		return nil, err
	}
	return &CppGCPersistent{persistent: persistent}, nil
}

// Set changes the managed object rooted by persistent.
func (persistent *CppGCPersistent) Set(object *CppGCObject) error {
	if persistent == nil {
		return fmt.Errorf("gov8: nil cppgc persistent")
	}
	return persistent.persistent.set(object)
}

// SetFromPersistent changes the rooted object to the object currently held by
// source. An empty source clears persistent.
func (persistent *CppGCPersistent) SetFromPersistent(source *CppGCPersistent) error {
	if persistent == nil || source == nil {
		return fmt.Errorf("gov8: nil cppgc persistent")
	}
	return persistent.persistent.setFromPersistent(source.persistent)
}

// SetFromWeakPersistent changes the rooted object to the object currently held
// by source. An empty source clears persistent.
func (persistent *CppGCPersistent) SetFromWeakPersistent(source *CppGCWeakPersistent) error {
	if persistent == nil || source == nil {
		return fmt.Errorf("gov8: nil cppgc persistent")
	}
	return persistent.persistent.setFromPersistent(source.persistent)
}

// Get returns copied managed-object metadata, or ok=false when empty.
func (persistent *CppGCPersistent) Get() (CppGCObjectSnapshot, bool, error) {
	if persistent == nil {
		return CppGCObjectSnapshot{}, false, fmt.Errorf("gov8: nil cppgc persistent")
	}
	snapshot, _, ok, err := persistent.persistent.get()
	return snapshot, ok, err
}

// Matches reports whether persistent currently points to object.
func (persistent *CppGCPersistent) Matches(object *CppGCObject) (bool, error) {
	if persistent == nil {
		return false, fmt.Errorf("gov8: nil cppgc persistent")
	}
	return persistent.persistent.matches(object)
}

// Close releases the strong root. Close is idempotent.
func (persistent *CppGCPersistent) Close() error {
	if persistent == nil {
		return nil
	}
	return persistent.persistent.close()
}

// CppGCWeakPersistent is a weak, native-owned cppgc handle. It does not keep
// its managed object alive and becomes empty when that object is collected.
// Operations are owner-thread-only. Isolate teardown releases an outstanding
// native handle; later Close remains idempotent.
type CppGCWeakPersistent struct{ persistent *cppgcPersistentHandle }

// NewEmptyCppGCWeakPersistent creates an empty weak cppgc handle.
func NewEmptyCppGCWeakPersistent(iso *Isolate) (*CppGCWeakPersistent, error) {
	persistent, err := newEmptyCppGCPersistent(iso, true)
	if err != nil {
		return nil, err
	}
	return &CppGCWeakPersistent{persistent: persistent}, nil
}

// NewCppGCWeakPersistent creates a weak cppgc handle initialized from object.
func NewCppGCWeakPersistent(object *CppGCObject) (*CppGCWeakPersistent, error) {
	persistent, err := newCppGCPersistentFromObject(object, true)
	if err != nil {
		return nil, err
	}
	return &CppGCWeakPersistent{persistent: persistent}, nil
}

// Set changes the managed object observed by persistent.
func (persistent *CppGCWeakPersistent) Set(object *CppGCObject) error {
	if persistent == nil {
		return fmt.Errorf("gov8: nil cppgc weak persistent")
	}
	return persistent.persistent.set(object)
}

// SetFromPersistent changes the observed object to the object currently held
// by source. An empty source clears persistent.
func (persistent *CppGCWeakPersistent) SetFromPersistent(source *CppGCPersistent) error {
	if persistent == nil || source == nil {
		return fmt.Errorf("gov8: nil cppgc weak persistent")
	}
	return persistent.persistent.setFromPersistent(source.persistent)
}

// SetFromWeakPersistent changes the observed object to the object currently
// held by source. An empty source clears persistent.
func (persistent *CppGCWeakPersistent) SetFromWeakPersistent(source *CppGCWeakPersistent) error {
	if persistent == nil || source == nil {
		return fmt.Errorf("gov8: nil cppgc weak persistent")
	}
	return persistent.persistent.setFromPersistent(source.persistent)
}

// Get returns copied managed-object metadata, or ok=false when empty or after
// the weakly referenced object has been collected.
func (persistent *CppGCWeakPersistent) Get() (CppGCObjectSnapshot, bool, error) {
	if persistent == nil {
		return CppGCObjectSnapshot{}, false, fmt.Errorf("gov8: nil cppgc weak persistent")
	}
	snapshot, _, ok, err := persistent.persistent.get()
	return snapshot, ok, err
}

// Matches reports whether persistent currently points to object.
func (persistent *CppGCWeakPersistent) Matches(object *CppGCObject) (bool, error) {
	if persistent == nil {
		return false, fmt.Errorf("gov8: nil cppgc weak persistent")
	}
	return persistent.persistent.matches(object)
}

// Close releases the weak handle. Close is idempotent.
func (persistent *CppGCWeakPersistent) Close() error {
	if persistent == nil {
		return nil
	}
	return persistent.persistent.close()
}
