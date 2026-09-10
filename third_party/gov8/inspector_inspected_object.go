//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// InspectorInspectableGetter is evaluated on every command-line API
// dereference of $0 through $4. The Context is the existing registered Go
// wrapper matching V8's callback context. The returned value must be created
// or reopened in callbackScope.Scope(); returning an error is unrecoverable at
// this native callback boundary and terminates the process.
type InspectorInspectableGetter func(callbackScope *CallbackScope, context *Context) (Value, error)

// InspectorInspectable is an owned pending Inspector inspectable. Close drops
// an unadded value. AddInspectedObject transfers ownership to the session and
// consumes this wrapper; V8 then drops it on eviction or session destruction.
type InspectorInspectable struct {
	iso         *Isolate
	id          uint64
	handle      uintptr
	transferred bool
	closed      bool
}

type inspectorInspectableEntry struct {
	iso           *Isolate
	inspectable   *InspectorInspectable
	getter        InspectorInspectableGetter
	onDrop        func()
	session       *InspectorSession
	active        int
	activeContext *Context
}

var inspectorInspectables = struct {
	sync.Mutex
	next    uint64
	entries map[uint64]*inspectorInspectableEntry
}{entries: make(map[uint64]*inspectorInspectableEntry)}

type inspectorInspectableFrame struct {
	id          uint64
	isolate     uintptr
	scopeWire   uintptr
	contextWire uintptr
	resultWire  uintptr
}

var inspectorInspectableDispatchOnce sync.Once
var inspectorInspectableDispatchErr error

var inspectorInspectableGetDispatch = syscall.NewCallback(func(frameWord uintptr) uintptr {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in Inspector Inspectable.Get callback: %v\n", recovered)
			proc("gov8_host_panic_abort").Call()
			panic(recovered)
		}
	}()
	if frameWord == 0 {
		fatalHostMisuse("nil Inspector Inspectable callback frame")
		return 1
	}
	frame := (*inspectorInspectableFrame)(abiWordToPtr(frameWord))
	inspectorInspectables.Lock()
	entry := inspectorInspectables.entries[frame.id]
	if entry != nil {
		entry.active++
	}
	inspectorInspectables.Unlock()
	if entry == nil || entry.getter == nil || entry.session == nil {
		fatalHostMisuse("unknown Inspector Inspectable callback %d", frame.id)
		return 1
	}
	defer func() {
		inspectorInspectables.Lock()
		entry.active--
		inspectorInspectables.Unlock()
	}()
	if frame.isolate != entry.iso.handleAssumingCheck() || frame.scopeWire == 0 || frame.contextWire == 0 {
		fatalHostMisuse("invalid Inspector Inspectable callback frame %d", frame.id)
		return 1
	}

	inspectorChannels.Lock()
	channelEntry := inspectorChannels.entries[entry.session.channelID]
	if channelEntry != nil {
		channelEntry.active++
		entry.session.active++
	}
	inspectorChannels.Unlock()
	if channelEntry == nil {
		fatalHostMisuse("Inspector Inspectable callback has no live session channel")
		return 1
	}
	defer func() {
		inspectorChannels.Lock()
		channelEntry.active--
		entry.session.active--
		inspectorChannels.Unlock()
	}()

	if err := entry.iso.check(); err != nil {
		fatalHostMisuse("Inspector Inspectable callback outside isolate lifetime: %v", err)
		return 1
	}
	var context *Context
	for candidate := range entry.session.inspector.contexts {
		if candidate == nil || candidate.iso != entry.iso || candidate.closed {
			continue
		}
		var matches int32
		if err := callErr("InspectorInspectable.ContextMatches", proc("gov8_iio_context_matches"),
			frame.isolate, frame.contextWire, candidate.handle,
			uintptr(unsafe.Pointer(&matches))); err != nil {
			fatalHostMisuse("cannot map Inspector Inspectable callback context: %v", err)
			return 1
		}
		if matches == 1 {
			context = candidate
			break
		}
	}
	if context == nil {
		fatalHostMisuse("Inspector Inspectable callback context is not registered")
		return 1
	}
	inspectorInspectables.Lock()
	entry.activeContext = context
	inspectorInspectables.Unlock()
	defer func() {
		inspectorInspectables.Lock()
		entry.activeContext = nil
		inspectorInspectables.Unlock()
	}()
	borrowed := &Scope{iso: entry.iso, handle: frame.scopeWire, borrowed: true}
	callbackScope := &CallbackScope{iso: entry.iso, sc: borrowed, ctxWire: frame.contextWire}
	return func() uintptr {
		defer func() { borrowed.closed = true }()
		value, err := entry.getter(callbackScope, context)
		if err != nil {
			fatalHostMisuse("Inspector Inspectable callback returned an error: %v", err)
			return 1
		}
		if value.iso != entry.iso || value.sc != borrowed {
			fatalHostMisuse("Inspector Inspectable callback returned a value outside its callback scope")
			return 1
		}
		if err := value.check(); err != nil {
			fatalHostMisuse("Inspector Inspectable callback returned an invalid value: %v", err)
			return 1
		}
		frame.resultWire = value.h
		return 0
	}()
})

var inspectorInspectableDropDispatch = syscall.NewCallback(func(idWord uintptr) uintptr {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in Inspector Inspectable.Drop callback: %v\n", recovered)
			proc("gov8_host_panic_abort").Call()
			panic(recovered)
		}
	}()
	id := uint64(idWord)
	inspectorInspectables.Lock()
	entry := inspectorInspectables.entries[id]
	if entry != nil && entry.active == 0 {
		delete(inspectorInspectables.entries, id)
		if entry.inspectable != nil {
			entry.inspectable.closed = true
			entry.inspectable.handle = 0
		}
	}
	inspectorInspectables.Unlock()
	if entry == nil {
		fatalHostMisuse("duplicate or unknown Inspector Inspectable drop %d", id)
		return 1
	}
	if entry.active != 0 {
		fatalHostMisuse("Inspector Inspectable %d dropped during its callback", id)
		return 1
	}
	if entry.onDrop != nil {
		entry.onDrop()
	}
	return 0
})

func ensureInspectorInspectableDispatch() error {
	inspectorInspectableDispatchOnce.Do(func() {
		inspectorInspectableDispatchErr = callErr("InspectorInspectable.Dispatchers",
			proc("gov8_iio_set_dispatchers"), inspectorInspectableGetDispatch,
			inspectorInspectableDropDispatch)
	})
	return inspectorInspectableDispatchErr
}

// NewInspectorInspectable creates an inspectable callback owned by isolate.
// onDrop, when non-nil, is called exactly once when an unadded inspectable is
// closed, V8 evicts a transferred inspectable, or its session is destroyed.
// onDrop must be pure Go and must not panic or re-enter V8.
func (i *Isolate) NewInspectorInspectable(getter InspectorInspectableGetter,
	onDrop func()) (*InspectorInspectable, error) {
	if i == nil {
		return nil, errors.New("gov8: nil isolate")
	}
	if getter == nil {
		return nil, errors.New("gov8: nil Inspector Inspectable getter")
	}
	if err := i.check(); err != nil {
		return nil, err
	}
	if err := ensureInspectorInspectableDispatch(); err != nil {
		return nil, err
	}
	inspectorInspectables.Lock()
	if inspectorInspectables.next == math.MaxUint64 {
		inspectorInspectables.Unlock()
		return nil, errors.New("gov8: Inspector Inspectable registry exhausted")
	}
	inspectorInspectables.next++
	id := inspectorInspectables.next
	value := &InspectorInspectable{iso: i, id: id}
	inspectorInspectables.entries[id] = &inspectorInspectableEntry{
		iso: i, inspectable: value, getter: getter, onDrop: onDrop,
	}
	inspectorInspectables.Unlock()
	var handle uintptr
	if err := callErr("InspectorInspectable.New", proc("gov8_iio_new"), uintptr(id),
		uintptr(unsafe.Pointer(&handle))); err != nil {
		inspectorInspectables.Lock()
		delete(inspectorInspectables.entries, id)
		inspectorInspectables.Unlock()
		return nil, err
	}
	if handle == 0 {
		inspectorInspectables.Lock()
		delete(inspectorInspectables.entries, id)
		inspectorInspectables.Unlock()
		return nil, errors.New("gov8: Inspector Inspectable constructor returned null")
	}
	value.handle = handle
	return value, nil
}

// Close releases an inspectable that has not been transferred to a session.
func (i *InspectorInspectable) Close() error {
	if i == nil {
		return errors.New("gov8: nil Inspector Inspectable")
	}
	if err := i.iso.check(); err != nil {
		return err
	}
	if i.transferred {
		return errors.New("gov8: Inspector Inspectable ownership was transferred to a session")
	}
	if i.closed || i.handle == 0 {
		return errors.New("gov8: Inspector Inspectable already closed")
	}
	if err := callErr("InspectorInspectable.Close", proc("gov8_iio_delete"), i.handle); err != nil {
		return err
	}
	if !i.closed || i.handle != 0 {
		return errors.New("gov8: Inspector Inspectable native drop was not observed")
	}
	return nil
}

func inspectorInspectableSessionActive(session *InspectorSession) bool {
	inspectorInspectables.Lock()
	defer inspectorInspectables.Unlock()
	for _, entry := range inspectorInspectables.entries {
		if entry.session == session && entry.active != 0 {
			return true
		}
	}
	return false
}

func inspectorInspectableContextActive(inspector *Inspector, context *Context) bool {
	inspectorInspectables.Lock()
	defer inspectorInspectables.Unlock()
	for _, entry := range inspectorInspectables.entries {
		if entry.session != nil && entry.session.inspector == inspector &&
			entry.active != 0 && entry.activeContext == context {
			return true
		}
	}
	return false
}

func inspectorInspectableIsolateCloseError(isolate *Isolate) error {
	inspectorInspectables.Lock()
	defer inspectorInspectables.Unlock()
	count := 0
	for _, entry := range inspectorInspectables.entries {
		if entry.iso == isolate {
			count++
		}
	}
	if count != 0 {
		return fmt.Errorf("gov8: isolate has %d live Inspector Inspectable(s)", count)
	}
	return nil
}

func releaseInspectorInspectableHostState(isolate *Isolate) error {
	return inspectorInspectableIsolateCloseError(isolate)
}

// AddInspectedObject transfers inspectable to the session. V8 retains only the
// five newest values and destroys evicted entries synchronously.
func (s *InspectorSession) AddInspectedObject(inspectable *InspectorInspectable) error {
	if err := s.check(); err != nil {
		return err
	}
	if inspectable == nil {
		return errors.New("gov8: nil Inspector Inspectable")
	}
	if inspectable.iso != s.inspector.iso {
		return foreignIsolate("Inspector Inspectable")
	}
	if inspectable.closed || inspectable.handle == 0 {
		return errors.New("gov8: Inspector Inspectable used after Close")
	}
	if inspectable.transferred {
		return errors.New("gov8: Inspector Inspectable was already transferred")
	}
	if inspectorInspectableSessionActive(s) {
		return errors.New("gov8: cannot add an Inspector Inspectable during its callback")
	}
	inspectorInspectables.Lock()
	entry := inspectorInspectables.entries[inspectable.id]
	if entry == nil || entry.inspectable != inspectable || entry.session != nil {
		inspectorInspectables.Unlock()
		return errors.New("gov8: invalid Inspector Inspectable registry entry")
	}
	entry.session = s
	inspectorInspectables.Unlock()
	handle := inspectable.handle
	var consumed int32
	err := callErr("InspectorSession.AddInspectedObject", proc("gov8_iio_add"),
		s.handle, handle, uintptr(unsafe.Pointer(&consumed)))
	if consumed != 0 && consumed != 1 {
		return errors.New("gov8: invalid Inspector Inspectable ownership result")
	}
	if consumed == 0 {
		inspectorInspectables.Lock()
		if current := inspectorInspectables.entries[inspectable.id]; current == entry {
			entry.session = nil
		}
		inspectorInspectables.Unlock()
		if err != nil {
			return err
		}
		return errors.New("gov8: Inspector did not consume the Inspectable")
	}
	// Once native unique ownership is established, both success and a later
	// V8 exception consume the wrapper and native DROP owns registry cleanup.
	inspectable.transferred = true
	inspectable.handle = 0
	if err != nil {
		return err
	}
	return nil
}

func inspectorInspectableRegistryCount() int {
	inspectorInspectables.Lock()
	defer inspectorInspectables.Unlock()
	return len(inspectorInspectables.entries)
}
