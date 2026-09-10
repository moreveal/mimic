//go:build windows && amd64

package gov8

import (
	"fmt"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
)

// ArrayBufferAllocatorCallbacks implements the observable part of V8's
// ArrayBuffer allocator contract without allowing Go pointers to escape into
// native memory. Allocate and AllocateUninitialized decide whether an
// allocation is accepted. Native code owns the returned storage. Free observes
// the allocation length and its first byte immediately before native release.
// Drop runs once when the last native shared reference to the allocator dies.
//
// V8 may invoke these callbacks on multiple isolate threads concurrently. They
// must be concurrency-safe and must not call into V8. A callback panic cannot
// unwind through V8 and therefore terminates the process through the standard
// gov8 fail-fast callback boundary.
type ArrayBufferAllocatorCallbacks struct {
	Allocate              func(byteLength int) bool
	AllocateUninitialized func(byteLength int) bool
	Free                  func(byteLength int, firstByte byte)
	Drop                  func()
}

// ArrayBufferAllocator is one owned shared reference to a V8 ArrayBuffer
// allocator. Close releases this reference; isolates and backing stores retain
// independent native shared references as required by V8.
type ArrayBufferAllocator struct {
	mu     sync.Mutex
	handle uintptr
	closed bool
}

type arrayBufferAllocatorEntry struct {
	id        uint64
	callbacks ArrayBufferAllocatorCallbacks
}

var arrayBufferAllocatorRegistry = struct {
	sync.Mutex
	next    uint64
	entries map[uint64]*arrayBufferAllocatorEntry
}{entries: make(map[uint64]*arrayBufferAllocatorEntry)}

// Keep the most recently used immutable registry entry on an atomic fast
// path; the mutex-protected map remains authoritative when allocator IDs
// alternate. Registry removal clears this pointer while holding the registry
// lock, so an entry can only be observed after removal by a callback that raced
// with (and began before) the removal, matching the former locked lookup
// semantics.
var activeArrayBufferAllocatorEntry atomic.Pointer[arrayBufferAllocatorEntry]

const (
	arrayBufferAllocatorAllocate = iota + 1
	arrayBufferAllocatorAllocateUninitialized
	arrayBufferAllocatorFree
	arrayBufferAllocatorDrop
)

var (
	arrayBufferAllocatorDispatcherOnce sync.Once
	arrayBufferAllocatorDispatcherErr  error
	arrayBufferAllocatorPanicAbortAddr uintptr
)

func lookupArrayBufferAllocatorEntry(id uint64) *arrayBufferAllocatorEntry {
	if entry := activeArrayBufferAllocatorEntry.Load(); entry != nil && entry.id == id {
		return entry
	}
	arrayBufferAllocatorRegistry.Lock()
	entry := arrayBufferAllocatorRegistry.entries[id]
	if entry != nil {
		// Publish before unlocking. A concurrent removal takes the same lock
		// and therefore cannot clear the cache before this store completes.
		activeArrayBufferAllocatorEntry.Store(entry)
	}
	arrayBufferAllocatorRegistry.Unlock()
	return entry
}

func takeArrayBufferAllocatorEntry(id uint64) *arrayBufferAllocatorEntry {
	arrayBufferAllocatorRegistry.Lock()
	entry := arrayBufferAllocatorRegistry.entries[id]
	if entry != nil {
		delete(arrayBufferAllocatorRegistry.entries, id)
		activeArrayBufferAllocatorEntry.CompareAndSwap(entry, nil)
	}
	arrayBufferAllocatorRegistry.Unlock()
	return entry
}

var arrayBufferAllocatorDispatcher = syscall.NewCallback(func(idWord, kindWord, lengthWord, firstWord uintptr) (result uintptr) {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in ArrayBuffer allocator callback: %v\n", recovered)
			_, _, _ = syscall.Syscall(arrayBufferAllocatorPanicAbortAddr, 0, 0, 0, 0)
			fatalHostMisuse("gov8: panic abort unexpectedly returned")
		}
	}()
	id := uint64(idWord)
	var entry *arrayBufferAllocatorEntry
	if kindWord == arrayBufferAllocatorDrop {
		entry = takeArrayBufferAllocatorEntry(id)
	} else {
		entry = lookupArrayBufferAllocatorEntry(id)
	}
	if entry == nil {
		fatalHostMisuse("gov8: ArrayBuffer allocator callback for unknown registry ID %d", id)
	}
	if uint64(lengthWord) > uint64(math.MaxInt) {
		fatalHostMisuse("gov8: ArrayBuffer allocator callback length exceeds Go int")
	}
	length := int(lengthWord)
	switch kindWord {
	case arrayBufferAllocatorAllocate:
		if entry.callbacks.Allocate == nil || entry.callbacks.Allocate(length) {
			return 1
		}
		return 0
	case arrayBufferAllocatorAllocateUninitialized:
		if entry.callbacks.AllocateUninitialized == nil || entry.callbacks.AllocateUninitialized(length) {
			return 1
		}
		return 0
	case arrayBufferAllocatorFree:
		if entry.callbacks.Free != nil {
			entry.callbacks.Free(length, byte(firstWord))
		}
		return 1
	case arrayBufferAllocatorDrop:
		if entry.callbacks.Drop != nil {
			entry.callbacks.Drop()
		}
		return 1
	default:
		fatalHostMisuse("gov8: invalid ArrayBuffer allocator callback kind %d", kindWord)
		return 0
	}
})

func installArrayBufferAllocatorDispatcher() error {
	arrayBufferAllocatorDispatcherOnce.Do(func() {
		arrayBufferAllocatorDispatcherErr = callErr("ArrayBufferAllocator.Dispatcher",
			proc("gov8_aba_set_dispatcher"), arrayBufferAllocatorDispatcher)
		if arrayBufferAllocatorDispatcherErr == nil {
			arrayBufferAllocatorPanicAbortAddr = proc("gov8_host_panic_abort").Addr()
		}
	})
	return arrayBufferAllocatorDispatcherErr
}

func registerArrayBufferAllocator(callbacks ArrayBufferAllocatorCallbacks) (uint64, error) {
	arrayBufferAllocatorRegistry.Lock()
	defer arrayBufferAllocatorRegistry.Unlock()
	for attempts := uint64(0); attempts < math.MaxUint64; attempts++ {
		arrayBufferAllocatorRegistry.next++
		id := arrayBufferAllocatorRegistry.next
		if id != 0 && arrayBufferAllocatorRegistry.entries[id] == nil {
			entry := &arrayBufferAllocatorEntry{id: id, callbacks: callbacks}
			arrayBufferAllocatorRegistry.entries[id] = entry
			activeArrayBufferAllocatorEntry.Store(entry)
			return id, nil
		}
	}
	return 0, fmt.Errorf("gov8: ArrayBuffer allocator registry exhausted")
}

func dropArrayBufferAllocatorRegistration(id uint64) {
	takeArrayBufferAllocatorEntry(id)
}

// NewDefaultArrayBufferAllocator creates V8's malloc/free based convenience
// allocator. Like rusty_v8's standalone factory, it may be called before
// Initialize so the result can be installed in CreateParams. The caller owns
// the returned reference and must Close it.
func NewDefaultArrayBufferAllocator() (*ArrayBufferAllocator, error) {
	if err := loadShim(); err != nil {
		return nil, err
	}
	handle, err := callHandle("NewDefaultArrayBufferAllocator", proc("gov8_aba_new_default"))
	if err != nil {
		return nil, err
	}
	return &ArrayBufferAllocator{handle: handle}, nil
}

// NewArrayBufferAllocator creates a native-memory allocator observed and
// controlled by safe Go callbacks. In contrast to rusty_v8's unsafe
// new_rust_allocator, callbacks never return or retain raw memory pointers.
// This standalone factory may be called before Initialize.
func NewArrayBufferAllocator(callbacks ArrayBufferAllocatorCallbacks) (*ArrayBufferAllocator, error) {
	if err := loadShim(); err != nil {
		return nil, err
	}
	if err := installArrayBufferAllocatorDispatcher(); err != nil {
		return nil, err
	}
	id, err := registerArrayBufferAllocator(callbacks)
	if err != nil {
		return nil, err
	}
	handle, err := callHandle("NewArrayBufferAllocator", proc("gov8_aba_new_host"), uintptr(id))
	if err != nil {
		dropArrayBufferAllocatorRegistration(id)
		return nil, err
	}
	return &ArrayBufferAllocator{handle: handle}, nil
}

func (a *ArrayBufferAllocator) cloneHandle() (uintptr, error) {
	if a == nil {
		return 0, fmt.Errorf("gov8: nil ArrayBuffer allocator")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.handle == 0 {
		return 0, fmt.Errorf("gov8: ArrayBuffer allocator used after Close")
	}
	return callHandle("ArrayBufferAllocator.clone", proc("gov8_aba_clone"), a.handle)
}

// UseCount returns the number of native shared references currently retaining
// the allocator. It is primarily useful for ownership diagnostics.
func (a *ArrayBufferAllocator) UseCount() (int, error) {
	if a == nil {
		return 0, fmt.Errorf("gov8: nil ArrayBuffer allocator")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.handle == 0 {
		return 0, fmt.Errorf("gov8: ArrayBuffer allocator used after Close")
	}
	r1, _, _ := proc("gov8_aba_use_count").Call(a.handle)
	if int64(r1) < 0 {
		return 0, shimError("ArrayBufferAllocator.UseCount", r1)
	}
	return int(r1), nil
}

// Close releases this owned reference. It does not invalidate isolates or
// backing stores that already retained the allocator. Close is concurrency-safe.
func (a *ArrayBufferAllocator) Close() error {
	if a == nil {
		return fmt.Errorf("gov8: nil ArrayBuffer allocator")
	}
	if err := loadShim(); err != nil {
		return err
	}
	a.mu.Lock()
	if a.closed || a.handle == 0 {
		a.mu.Unlock()
		return fmt.Errorf("gov8: ArrayBuffer allocator already closed")
	}
	handle := a.handle
	a.handle = 0
	a.closed = true
	a.mu.Unlock()
	return callErr("ArrayBufferAllocator.Close", proc("gov8_aba_dispose"), handle)
}
