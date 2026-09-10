//go:build windows && amd64

package gov8

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

type heapSnapshotEntry struct {
	iso      *Isolate
	callback func([]byte) bool
}

var heapSnapshotRegistry = struct {
	sync.Mutex
	next    uint64
	entries map[uint64]*heapSnapshotEntry
	active  map[*Isolate]uint64
}{
	entries: make(map[uint64]*heapSnapshotEntry),
	active:  make(map[*Isolate]uint64),
}

var (
	heapSnapshotDispatcherOnce sync.Once
	heapSnapshotDispatcherErr  error
)

var goHeapSnapshotDispatch = syscall.NewCallback(func(idWord, dataWord, lengthWord uintptr) (result uintptr) {
	defer func() {
		if recovered := recover(); recovered != nil {
			fatalHostMisuse("panic in heap snapshot callback: %v", recovered)
		}
	}()

	id := uint64(idWord)
	heapSnapshotRegistry.Lock()
	entry := heapSnapshotRegistry.entries[id]
	heapSnapshotRegistry.Unlock()
	if entry == nil {
		fatalHostMisuse("heap snapshot callback for unknown registry ID %d", id)
		return 0
	}
	if err := entry.iso.check(); err != nil {
		fatalHostMisuse("heap snapshot callback invoked off the owning thread: %v", err)
		return 0
	}
	maxInt := uintptr(^uint(0) >> 1)
	if lengthWord > maxInt {
		fatalHostMisuse("heap snapshot callback chunk length %d exceeds max int", lengthWord)
		return 0
	}
	if lengthWord != 0 && dataWord == 0 {
		fatalHostMisuse("heap snapshot callback received null data with length %d", lengthWord)
		return 0
	}

	// The native chunk is borrowed only for the callback invocation. Copy it so
	// callers may safely retain or mutate the delivered slice after returning.
	chunk := make([]byte, int(lengthWord))
	if lengthWord != 0 {
		copy(chunk, unsafe.Slice((*byte)(abiWordToPtr(dataWord)), int(lengthWord)))
	}
	if entry.callback(chunk) {
		return 1
	}
	return 0
})

func ensureHeapSnapshotDispatcher() error {
	heapSnapshotDispatcherOnce.Do(func() {
		heapSnapshotDispatcherErr = callErr("HeapSnapshot.SetDispatcher",
			proc("gov8_heap_snapshot_set_dispatcher"), goHeapSnapshotDispatch)
	})
	return heapSnapshotDispatcherErr
}

func registerHeapSnapshot(iso *Isolate, callback func([]byte) bool) (uint64, error) {
	heapSnapshotRegistry.Lock()
	defer heapSnapshotRegistry.Unlock()
	if heapSnapshotRegistry.active[iso] != 0 {
		return 0, fmt.Errorf("gov8: heap snapshot already active for isolate")
	}
	for attempts := uint64(0); attempts < math.MaxUint64; attempts++ {
		heapSnapshotRegistry.next++
		id := heapSnapshotRegistry.next
		if id != 0 && heapSnapshotRegistry.entries[id] == nil {
			heapSnapshotRegistry.entries[id] = &heapSnapshotEntry{iso: iso, callback: callback}
			heapSnapshotRegistry.active[iso] = id
			return id, nil
		}
	}
	return 0, fmt.Errorf("gov8: heap snapshot callback registry exhausted")
}

func unregisterHeapSnapshot(iso *Isolate, id uint64) {
	heapSnapshotRegistry.Lock()
	delete(heapSnapshotRegistry.entries, id)
	if heapSnapshotRegistry.active[iso] == id {
		delete(heapSnapshotRegistry.active, iso)
	}
	heapSnapshotRegistry.Unlock()
}

func heapSnapshotIsolateCloseError(iso *Isolate) error {
	heapSnapshotRegistry.Lock()
	active := heapSnapshotRegistry.active[iso] != 0
	heapSnapshotRegistry.Unlock()
	if active {
		return fmt.Errorf("gov8: isolate has an active heap snapshot")
	}
	return nil
}

// TakeHeapSnapshot serializes a V8 heap snapshot as JSON chunks. callback is
// called one or more times and receives a final empty chunk after successful
// serialization. Returning false aborts serialization without making the
// isolate unusable. Each chunk is copied into Go-owned memory and may be
// retained after callback returns.
//
// Snapshotting is synchronous and thread-affine. Starting another snapshot or
// closing the isolate from callback is rejected. A callback panic is a fatal
// host error because it cannot unwind through V8.
func (i *Isolate) TakeHeapSnapshot(callback func([]byte) bool) error {
	if i == nil {
		return fmt.Errorf("gov8: nil isolate")
	}
	if callback == nil {
		return fmt.Errorf("gov8: heap snapshot callback required")
	}
	handle, err := i.handleChecked()
	if err != nil {
		return err
	}
	if err := ensureHeapSnapshotDispatcher(); err != nil {
		return err
	}
	id, err := registerHeapSnapshot(i, callback)
	if err != nil {
		return err
	}
	defer unregisterHeapSnapshot(i, id)

	r1, _, _ := proc("gov8_heap_snapshot_take").Call(handle, uintptr(id))
	runtime.KeepAlive(callback)
	runtime.KeepAlive(i)
	if int64(r1) < 0 {
		return shimError("Isolate.TakeHeapSnapshot", r1)
	}
	return nil
}
