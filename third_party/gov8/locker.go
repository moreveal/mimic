//go:build windows && amd64

package gov8

import (
	"fmt"
	"runtime"
	"sync"
)

// SharedIsolate and Locker: multi-thread isolate access, one thread at a
// time, mirroring the pinned crate's src/locker.rs observably.
//
// Parity mapping:
//   - OwnedIsolate::try_into_shared -> Isolate.TryIntoShared, returning
//     (*SharedIsolate, error). The rejection reasons surface as an
//     *IntoSharedError whose Kind is the stable string the oracle pins
//     ("another_isolate_entered", "live_weak_handles_or_pending_finalizers",
//     "snapshot_creator", "embedder_cpp_heap"); error recovery is
//     err.IntoIsolate() (the pinned IntoSharedError::into_isolate). The two
//     panic guards of the pinned lock() are Go errors with the same message
//     text (the package-wide panic-to-error deviation).
//   - SharedIsolate::lock -> SharedIsolate.Lock, yielding a *Locker. The
//     Locker pins the isolate's Go-side thread affinity to the locking
//     thread for its lifetime, so every regular gov8 wrapper (scopes,
//     contexts, values, scripts) works unchanged while the lock is held.
//   - Locker::unlock(f) -> Locker.UnlockWindow(fn): the isolate is exited,
//     an engine Unlocker releases the lock so another thread can Lock, and
//     the lock is reacquired after fn returns.
//   - Dropping the SharedIsolate -> SharedIsolate.Close, which disposes the
//     isolate WITHOUT the creation Exit (the conversion already exited it;
//     every lock cycle exits after itself).
//
// Ownership and thread rules: Lock/Close serialize through one engine-side
// v8::Locker per isolate. While no Locker is held the isolate accepts NO
// engine operations on any thread (the affinity check fails everywhere),
// except the thread-safe ThreadSafeHandle controls, which mirror the pinned
// IsolateHandle. Weak handles are not supported on shared isolates by the
// pinned crate; this port rejects conversion with any live non-empty weak or
// pending finalizer and rejects creation of a non-empty weak after conversion.
// Empty weak handles remain valid because they own no engine handle.
//
// SharedIsolate.Close may be called from any thread that satisfies the
// no-lock-held state machine; only the creating goroutine drops its OS
// thread pin (Go pins per goroutine).

// IntoSharedErrorKind is the stable string form of the pinned
// IntoSharedErrorKind.
type IntoSharedErrorKind string

const (
	// KindSnapshotCreator rejects creator-backed isolates. Unreachable
	// through this wrapper: creator isolates never present as a plain
	// *Isolate with an annex (the snapshot slice owns them); kept for
	// exhaustive matching.
	KindSnapshotCreator IntoSharedErrorKind = "snapshot_creator"
	// KindLiveWeakHandlesOrPendingFinalizers rejects isolates with live
	// weak handles or pending finalizers.
	KindLiveWeakHandlesOrPendingFinalizers = "live_weak_handles_or_pending_finalizers"
	// KindEmbedderCppHeap rejects isolates with an attached cppgc heap;
	// this module never attaches one.
	KindEmbedderCppHeap = "embedder_cpp_heap"
	// KindAnotherIsolateEntered rejects conversion while this or another
	// isolate is the thread's current one above the target.
	KindAnotherIsolateEntered = "another_isolate_entered"
)

// IntoSharedError reports why TryIntoShared refused an isolate. IntoIsolate
// hands the isolate back unchanged (the pinned IntoSharedError::into_isolate
// recovery): the conversion attempt had no engine-side effect.
type IntoSharedError struct {
	Kind IntoSharedErrorKind
	iso  *Isolate
}

func (e *IntoSharedError) Error() string {
	return fmt.Sprintf("gov8: isolate cannot be shared: %s", e.Kind)
}

// IntoIsolate returns the rejected isolate back to the caller. It is still
// fully usable (entered mode, owning thread).
func (e *IntoSharedError) IntoIsolate() *Isolate { return e.iso }

// sharedState is the Go-side mirror of the pinned shared-isolate state
// machine. All transitions hold sharedMu; the engine lock itself serializes
// the actual engine access.
type sharedState struct {
	shared         bool
	locked         bool
	window         bool
	ownerTid       uint32
	windowOwnerTid uint32
	creatorTid     uint32
}

var sharedRegistry = struct {
	mu sync.Mutex
	m  map[*Isolate]*sharedState
}{m: make(map[*Isolate]*sharedState)}

func sharedStateOf(i *Isolate) *sharedState {
	return sharedRegistry.m[i]
}

func isolateIsShared(i *Isolate) bool {
	sharedRegistry.mu.Lock()
	defer sharedRegistry.mu.Unlock()
	return sharedStateOf(i) != nil
}

func isolateClosedLocked(i *Isolate) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.closed
}

// TryIntoShared converts an owned isolate into a shared one. On rejection
// the returned error's IntoIsolate recovers the isolate unchanged; on
// success the isolate accepts engine work only under Lock (the thread-safe
// handle keeps working, exactly like the pinned IsolateHandle).
func (i *Isolate) TryIntoShared() (*SharedIsolate, error) {
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	sharedRegistry.mu.Lock()
	defer sharedRegistry.mu.Unlock()
	if sharedStateOf(i) != nil {
		return nil, fmt.Errorf("gov8: isolate is already shared")
	}
	if isolateClosedLocked(i) {
		return nil, fmt.Errorf("gov8: isolate used after Close")
	}
	// Live weaks / pending finalizers reject the conversion (the pinned
	// annex check). The Go registry is the source of truth for both.
	if liveWeakCount(i) > 0 {
		return nil, &IntoSharedError{Kind: KindLiveWeakHandlesOrPendingFinalizers, iso: i}
	}
	if i.customCppGCHeap {
		return nil, &IntoSharedError{Kind: KindEmbedderCppHeap, iso: i}
	}
	// Isolate::GetCurrent must be this isolate: the creation Enter makes
	// that true on the owning thread; any other isolate entered on top of
	// it (or a foreign thread) fails the check without engine changes.
	cur, _, _ := proc("gov8_ca_try_current").Call()
	if cur != i.handle {
		return nil, &IntoSharedError{Kind: KindAnotherIsolateEntered, iso: i}
	}
	if err := callErr("TryIntoShared", proc("gov8_ca_shared_convert"), i.handle); err != nil {
		return nil, err
	}
	st := &sharedState{shared: true, creatorTid: i.tid}
	sharedRegistry.m[i] = st
	// While unlocked the isolate accepts nothing: the affinity check must
	// fail everywhere. tid 0 never matches a real thread id.
	i.mu.Lock()
	i.tid = 0
	i.mu.Unlock()
	return &SharedIsolate{iso: i}, nil
}

// SharedIsolate is an isolate usable from multiple threads, one at a time.
// All engine access goes through Lock; the thread-safe handle keeps working
// without the lock.
type SharedIsolate struct {
	iso *Isolate
}

// Isolate returns the underlying isolate wrapper. Engine operations on it
// are only legal while a Lock is held.
func (s *SharedIsolate) Isolate() *Isolate { return s.iso }

// ThreadSafeHandle returns a handle usable from any thread without holding
// the lock (termination control and interrupt requests), matching the
// pinned SharedIsolate::thread_safe_handle.
func (s *SharedIsolate) ThreadSafeHandle() *ThreadSafeHandle {
	return s.iso.ThreadSafeHandle()
}

// Lock acquires the isolate's engine lock and enters the isolate on the
// calling goroutine's OS thread, blocking while any other thread holds it.
//
// Errors mirror the pinned lock() guards verbatim: locking again from a
// thread that already holds this isolate's lock ("already locked by this
// thread") and locking while another isolate is entered on this thread
// ("while another isolate is entered"). Both fire before any engine state
// changes, so the isolate remains fully usable after the error.
func (s *SharedIsolate) Lock() (*Locker, error) {
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	// Pin this goroutine to its OS thread for the lock's lifetime: the
	// affinity bookkeeping below and every engine call under the lock must
	// observe one stable thread id (worker goroutines are otherwise free to
	// migrate between syscalls).
	runtime.LockOSThread()
	iso := s.iso
	cur := currentThreadID()
	// Phase 1: state validation under the registry lock. The registry lock
	// is NOT held across the engine acquisition (which blocks): the
	// holding thread must be able to reach Unlock while we wait.
	sharedRegistry.mu.Lock()
	st := sharedStateOf(iso)
	if st == nil {
		sharedRegistry.mu.Unlock()
		return nil, fmt.Errorf("gov8: isolate is not shared")
	}
	if isolateClosedLocked(iso) {
		sharedRegistry.mu.Unlock()
		return nil, fmt.Errorf("gov8: isolate used after Close")
	}
	if st.window {
		if st.windowOwnerTid == cur {
			sharedRegistry.mu.Unlock()
			runtime.UnlockOSThread()
			return nil, fmt.Errorf("gov8: isolate is suspended in this thread's own unlock window")
		}
		// Another thread's window: this thread may lock — that is the
		// window's whole purpose. Fall through to the engine acquisition.
		sharedRegistry.mu.Unlock()
	} else if st.locked {
		sharedRegistry.mu.Unlock()
		if st.ownerTid == cur {
			runtime.UnlockOSThread()
			return nil, fmt.Errorf("gov8: attempted to lock an isolate that is already locked by this thread")
		}
		// Another thread holds it: fall through and block in the engine
		// lock, exactly like the pinned lock().
	} else if entered, _, _ := proc("gov8_ca_try_current").Call(); entered != 0 {
		sharedRegistry.mu.Unlock()
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("gov8: attempted to lock a shared isolate while another isolate is entered")
	} else {
		sharedRegistry.mu.Unlock()
	}
	// Phase 2: acquire the engine lock (blocking) and enter on this thread.
	w, err := callHandle("SharedIsolate.Lock", proc("gov8_ca_shared_lock"), iso.handle)
	if err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}
	// Phase 3: publish the new holder.
	sharedRegistry.mu.Lock()
	st.locked = true
	st.ownerTid = cur
	sharedRegistry.mu.Unlock()
	iso.mu.Lock()
	iso.tid = cur
	iso.mu.Unlock()
	return &Locker{iso: iso, handle: w, tid: cur}, nil
}

// Locker holds the isolate locked and entered on the locking thread.
// Regular engine operations on the isolate (scopes, contexts, values,
// scripts, try-catches) run unchanged while a Locker is alive.
type Locker struct {
	iso    *Isolate
	handle uintptr
	tid    uint32
	closed bool
}

// Isolate returns the locked isolate.
func (l *Locker) Isolate() *Isolate { return l.iso }

// Close releases the lock: the isolate is exited on this thread and the
// engine lock is released. It must be called from the locking thread and
// exactly once.
func (l *Locker) Close() error {
	sharedRegistry.mu.Lock()
	defer sharedRegistry.mu.Unlock()
	if l.closed {
		return fmt.Errorf("gov8: locker already closed")
	}
	st := sharedStateOf(l.iso)
	if st == nil {
		l.closed = true
		return fmt.Errorf("gov8: locker is not the active lock holder")
	}
	if st.window && st.windowOwnerTid == l.tid {
		l.closed = true
		return fmt.Errorf("gov8: cannot close a locker inside its own unlock window")
	}
	if !st.locked || st.ownerTid != l.tid {
		l.closed = true
		return fmt.Errorf("gov8: locker is not the active lock holder")
	}
	if currentThreadID() != l.tid {
		return fmt.Errorf("gov8: locker Close called from wrong thread (lock owner %s, caller %s)",
			quoteThreadID(l.tid), quoteThreadID(currentThreadID()))
	}
	err := callErr("Locker.Close", proc("gov8_ca_shared_unlock"), l.iso.handle, l.handle)
	l.closed = true
	if err == nil {
		st.locked = false
		st.ownerTid = 0
		l.iso.mu.Lock()
		l.iso.tid = 0
		l.iso.mu.Unlock()
		// Balances the LockOSThread in SharedIsolate.Lock.
		runtime.UnlockOSThread()
	}
	return err
}

// UnlockWindow releases the lock for the duration of fn so other threads
// can lock and use the isolate, then reacquires it before returning
// (Locker::unlock). The isolate must not be touched from fn's goroutine
// while suspended: the window clears the thread binding, so any accidental
// engine access from this goroutine fails the affinity check instead of
// racing the other thread.
//
// Errors mirror the pinned guards: calling unlock while another isolate was
// entered on top ("entered on top of this one") and a closure that returned
// with an isolate still entered (the pinned crate asserts; this port
// refuses to touch the engine and leaves the window open, which keeps the
// isolate unusable but sound). When another thread still holds the lock at
// window end, the reacquisition blocks until it is released — the pinned
// RelockGuard behavior.
func (l *Locker) UnlockWindow(fn func() error) error {
	if fn == nil {
		return fmt.Errorf("gov8: unlock window requires a closure")
	}
	sharedRegistry.mu.Lock()
	if l.closed {
		sharedRegistry.mu.Unlock()
		return fmt.Errorf("gov8: locker already closed")
	}
	st := sharedStateOf(l.iso)
	if st == nil || !st.locked || st.ownerTid != l.tid || st.window {
		sharedRegistry.mu.Unlock()
		return fmt.Errorf("gov8: locker is not the active lock holder")
	}
	if currentThreadID() != l.tid {
		sharedRegistry.mu.Unlock()
		return fmt.Errorf("gov8: locker unlock called from wrong thread (lock owner %s, caller %s)",
			quoteThreadID(l.tid), quoteThreadID(currentThreadID()))
	}
	if cur, _, _ := proc("gov8_ca_try_current").Call(); cur != l.iso.handle {
		sharedRegistry.mu.Unlock()
		return fmt.Errorf("gov8: unlock called while another isolate was entered on top of this one")
	}
	// Suspend: exit + engine Unlocker, then mark the window so other
	// threads can take the lock.
	w, err := callHandle("Locker.UnlockWindow", proc("gov8_ca_shared_unlock_window_begin"), l.iso.handle)
	if err != nil {
		sharedRegistry.mu.Unlock()
		return err
	}
	st.window = true
	st.windowOwnerTid = l.tid
	st.locked = false
	st.ownerTid = 0
	l.iso.mu.Lock()
	l.iso.tid = 0
	l.iso.mu.Unlock()
	sharedRegistry.mu.Unlock()

	fnErr := fn()

	// Post-closure contract check (the pinned assert, made non-fatal):
	// a closure that left an isolate entered must not drive the
	// Exit/Enter pair below.
	if cur, _, _ := proc("gov8_ca_try_current").Call(); cur != 0 {
		return fmt.Errorf("gov8: unlock window closure returned while an isolate was still entered")
	}
	// Reacquire: the engine Unlocker destructor blocks while another
	// thread still holds the lock, then this thread re-enters. The
	// registry lock is not held during the block so the other thread's
	// unlock can proceed.
	if err := callErr("Locker.UnlockWindowEnd", proc("gov8_ca_shared_unlock_window_end"), w); err != nil {
		return err
	}
	sharedRegistry.mu.Lock()
	st.window = false
	st.windowOwnerTid = 0
	st.locked = true
	st.ownerTid = l.tid
	sharedRegistry.mu.Unlock()
	l.iso.mu.Lock()
	l.iso.tid = l.tid
	l.iso.mu.Unlock()
	return fnErr
}

// Close disposes the shared isolate. No lock may be held and no unlock
// window may be open. The creating goroutine drops its OS thread pin; a
// Close from another goroutine leaves that pin to expire with the creating
// goroutine (Go pins threads per goroutine).
func (s *SharedIsolate) Close() error {
	if err := requireInitialized(); err != nil {
		return err
	}
	// Pin for the dispose (see Lock): engine calls and the thread-id
	// comparisons below need a stable thread.
	runtime.LockOSThread()
	sharedRegistry.mu.Lock()
	defer sharedRegistry.mu.Unlock()
	st := sharedStateOf(s.iso)
	if st == nil {
		runtime.UnlockOSThread()
		return fmt.Errorf("gov8: isolate is not shared")
	}
	if st.locked {
		runtime.UnlockOSThread()
		return fmt.Errorf("gov8: cannot dispose a shared isolate while a lock is held")
	}
	if st.window {
		runtime.UnlockOSThread()
		return fmt.Errorf("gov8: cannot dispose a shared isolate inside an unlock window")
	}
	if isolateClosedLocked(s.iso) {
		runtime.UnlockOSThread()
		return fmt.Errorf("gov8: isolate already closed")
	}
	if err := callErr("SharedIsolate.Close", proc("gov8_ca_shared_isolate_dispose"), s.iso.handle); err != nil {
		runtime.UnlockOSThread()
		return err
	}
	s.iso.mu.Lock()
	s.iso.handle = 0
	s.iso.publishClosedLocked()
	creatorTid := st.creatorTid
	s.iso.mu.Unlock()
	delete(sharedRegistry.m, s.iso)
	// Drop the isolate from the teardown accounting; the engine isolate no
	// longer exists.
	unregisterIsolate(s.iso)
	if currentThreadID() == creatorTid {
		// The creating goroutine's pin came from NewIsolate; release it.
		runtime.UnlockOSThread()
	}
	// A non-creator goroutine releases its own Close-scoped pin.
	runtime.UnlockOSThread()
	return nil
}

// IsLocked reports whether the thread currently holds the engine lock for
// the isolate (v8::Locker::IsLocked, the pinned thread_holds_lock probe).
func IsLocked(i *Isolate) bool {
	if isolateClosedLocked(i) {
		return false
	}
	r1, _, _ := proc("gov8_ca_is_locked").Call(i.handle)
	return r1 == 1
}
