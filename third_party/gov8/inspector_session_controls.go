//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// InspectorClient is the optional-capability base for Inspector callbacks.
// Implement the focused capability interfaces for callbacks of interest.
// Callback methods must not panic; a panic terminates the process.
type InspectorClient interface{}

// InspectorRunMessageLoopOnPauseClient receives debugger pause-loop entry.
type InspectorRunMessageLoopOnPauseClient interface {
	RunMessageLoopOnPause(contextGroupID int32)
}

// InspectorQuitMessageLoopOnPauseClient receives debugger pause-loop exit.
type InspectorQuitMessageLoopOnPauseClient interface {
	QuitMessageLoopOnPause()
}

// InspectorPauseLoopClient is the convenience combination of both optional
// pause-loop capabilities.
type InspectorPauseLoopClient interface {
	InspectorRunMessageLoopOnPauseClient
	InspectorQuitMessageLoopOnPauseClient
}

type inspectorClientEntry struct {
	iso       *Isolate
	inspector *Inspector
	client    InspectorClient
	active    int
}

var inspectorClients = struct {
	sync.Mutex
	next        uint64
	byID        map[uint64]*inspectorClientEntry
	byInspector map[*Inspector]uint64
}{
	byID:        make(map[uint64]*inspectorClientEntry),
	byInspector: make(map[*Inspector]uint64),
}

var inspectorClientDispatchOnce sync.Once
var inspectorClientDispatchErr error

func inspectorClientDispatchCallback(idWord, kindWord, groupWord uintptr) uintptr {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in inspector client callback: %v\n", recovered)
			proc("gov8_host_panic_abort").Call()
			panic(recovered)
		}
	}()
	id := uint64(idWord)
	inspectorClients.Lock()
	entry := inspectorClients.byID[id]
	if entry != nil {
		entry.active++
	}
	inspectorClients.Unlock()
	if entry == nil || entry.client == nil {
		fatalHostMisuse("unknown inspector client %d", id)
		return 1
	}
	defer func() {
		inspectorClients.Lock()
		entry.active--
		inspectorClients.Unlock()
	}()
	if err := entry.iso.check(); err != nil {
		fatalHostMisuse("inspector client invoked outside its isolate lifetime: %v", err)
		return 1
	}
	switch int32(kindWord) {
	case 0:
		if client, ok := entry.client.(InspectorRunMessageLoopOnPauseClient); ok {
			client.RunMessageLoopOnPause(int32(groupWord))
		}
	case 1:
		if client, ok := entry.client.(InspectorQuitMessageLoopOnPauseClient); ok {
			client.QuitMessageLoopOnPause()
		}
	default:
		fatalHostMisuse("invalid inspector client callback kind %d", kindWord)
		return 1
	}
	return 0
}

var inspectorClientDispatch = syscall.NewCallback(inspectorClientDispatchCallback)

func ensureInspectorClientDispatch() error {
	inspectorClientDispatchOnce.Do(func() {
		inspectorClientDispatchErr = callErr("Inspector.ClientDispatch", proc("gov8_isc_set_client_dispatch"), inspectorClientDispatch)
		if inspectorClientDispatchErr == nil {
			inspectorClientDispatchErr = ensureInspectorExtendedClientDispatch()
		}
	})
	return inspectorClientDispatchErr
}

// NewInspectorWithClient creates an Inspector whose synchronous pause-loop
// callbacks are delivered to client. NewInspector continues to use V8's
// default no-op client behavior.
func NewInspectorWithClient(i *Isolate, client InspectorClient) (*Inspector, error) {
	if client == nil {
		return nil, errors.New("gov8: nil inspector client")
	}
	if err := ensureInspectorClientDispatch(); err != nil {
		return nil, err
	}
	inspectorClients.Lock()
	if inspectorClients.next == math.MaxUint64 {
		inspectorClients.Unlock()
		return nil, errors.New("gov8: inspector client registry exhausted")
	}
	inspectorClients.next++
	id := inspectorClients.next
	entry := &inspectorClientEntry{iso: i, client: client}
	inspectorClients.byID[id] = entry
	inspectorClients.Unlock()
	inspector, err := newInspector(i, id)
	if err != nil {
		inspectorClients.Lock()
		if inspectorClients.byID[id] == entry {
			delete(inspectorClients.byID, id)
		}
		inspectorClients.Unlock()
		return nil, err
	}
	inspectorClients.Lock()
	entry.inspector = inspector
	inspectorClients.byInspector[inspector] = id
	inspectorClients.Unlock()
	return inspector, nil
}

func inspectorClientCloseError(inspector *Inspector) error {
	inspectorClients.Lock()
	defer inspectorClients.Unlock()
	id := inspectorClients.byInspector[inspector]
	entry := inspectorClients.byID[id]
	if entry != nil && entry.active != 0 {
		return errors.New("gov8: inspector client callback is active")
	}
	return nil
}

func inspectorClientCallbackActive(inspector *Inspector) bool {
	inspectorClients.Lock()
	defer inspectorClients.Unlock()
	entry := inspectorClients.byID[inspectorClients.byInspector[inspector]]
	return entry != nil && entry.active != 0
}

func dropInspectorClient(inspector *Inspector) {
	inspectorClients.Lock()
	id := inspectorClients.byInspector[inspector]
	delete(inspectorClients.byInspector, inspector)
	if entry := inspectorClients.byID[id]; entry != nil && entry.inspector == inspector {
		delete(inspectorClients.byID, id)
	}
	inspectorClients.Unlock()
}

// InspectorCanDispatchMethod reports whether Inspector can dispatch the CDP
// method. It preserves the view's 8-bit/16-bit encoding and embedded NULs.
func InspectorCanDispatchMethod(method InspectorStringView) (bool, error) {
	if err := requireInitialized(); err != nil {
		return false, err
	}
	is8, data, length := method.native()
	var out int32
	err := callErr("Inspector.CanDispatchMethod", proc("gov8_isc_can_dispatch_method"),
		is8, data, length, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(method)
	if err != nil {
		return false, err
	}
	return out != 0, nil
}

// ReleaseObjectGroup releases remote objects associated with group.
func (s *InspectorSession) ReleaseObjectGroup(group InspectorStringView) error {
	if err := s.check(); err != nil {
		return err
	}
	is8, data, length := group.native()
	err := callErr("InspectorSession.ReleaseObjectGroup", proc("gov8_isc_release_object_group"),
		s.handle, is8, data, length)
	runtime.KeepAlive(group)
	return err
}

// SchedulePauseOnNextStatement asks V8 to pause at the next statement.
func (s *InspectorSession) SchedulePauseOnNextStatement(reason, detail InspectorStringView) error {
	if err := s.check(); err != nil {
		return err
	}
	ri, rp, rl := reason.native()
	di, dp, dl := detail.native()
	err := callErr("InspectorSession.SchedulePauseOnNextStatement", proc("gov8_isc_schedule_pause"),
		s.handle, ri, rp, rl, di, dp, dl)
	runtime.KeepAlive(reason)
	runtime.KeepAlive(detail)
	return err
}

// CancelPauseOnNextStatement cancels a pending scheduled pause.
func (s *InspectorSession) CancelPauseOnNextStatement() error {
	if err := s.check(); err != nil {
		return err
	}
	return callErr("InspectorSession.CancelPauseOnNextStatement", proc("gov8_isc_cancel_pause"), s.handle)
}
