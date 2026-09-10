//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// CRDTPFrontendChannel receives owned protocol messages. The receiver owns
// each non-nil CRDTPSerializable and may retain it after the callback; it must
// eventually call Close. Notification and flush callbacks are implemented for
// completeness of the pinned interface, although the public dispatcher send
// path exercised here emits responses only.
type CRDTPFrontendChannel interface {
	SendProtocolResponse(callID int32, message *CRDTPSerializable)
	SendProtocolNotification(message *CRDTPSerializable)
	FlushProtocolNotifications()
}

// CRDTPDomainDispatcher handles a command synchronously. request and responder
// are callback-borrowed and reject use after this method returns. Returning
// false asks UberDispatcher to synthesize the pinned method-not-found response.
type CRDTPDomainDispatcher interface {
	Dispatch(command []byte, request *CRDTPDispatchRequest, responder *CRDTPDomainResponder) bool
}

// CRDTPFallthroughCallback receives an unhandled message synchronously.
type CRDTPFallthroughCallback interface {
	FallThrough(callID int32, method, message, associatedData []byte)
}

type CRDTPFallthroughFunc func(callID int32, method, message, associatedData []byte)

func (f CRDTPFallthroughFunc) FallThrough(callID int32, method, message, associatedData []byte) {
	f(callID, method, message, associatedData)
}

// CRDTPCallbackDropper optionally observes when native ownership releases a
// channel, domain handler, or fallthrough callback. It executes across the
// native callback boundary, so a panic is fail-fast like a Rust Drop panic.
type CRDTPCallbackDropper interface{ CRDTPCallbackDropped() }

type crdtpChannelEntry struct {
	handler     CRDTPFrontendChannel
	active      atomic.Int32
	dispatchers int
}

type crdtpDomainEntry struct {
	handler CRDTPDomainDispatcher
	owner   *CRDTPUberDispatcher
	active  atomic.Int32
}

type crdtpFallthroughEntry struct {
	handler CRDTPFallthroughCallback
	active  atomic.Int32
}

var crdtpCallbacks = struct {
	sync.RWMutex
	next     uint64
	channels map[uint64]*crdtpChannelEntry
	domains  map[uint64]*crdtpDomainEntry
	fall     map[uint64]*crdtpFallthroughEntry
}{next: 1, channels: map[uint64]*crdtpChannelEntry{}, domains: map[uint64]*crdtpDomainEntry{}, fall: map[uint64]*crdtpFallthroughEntry{}}

func crdtpNextIDLocked() (uint64, error) {
	if crdtpCallbacks.next == 0 || crdtpCallbacks.next == math.MaxUint64 {
		return 0, errors.New("gov8: CRDTP callback registry exhausted")
	}
	id := crdtpCallbacks.next
	crdtpCallbacks.next++
	return id, nil
}

var (
	crdtpDispatchersOnce        sync.Once
	crdtpDispatchersErr         error
	crdtpUberDispatchAddr       uintptr
	crdtpDomainSendResponseAddr uintptr
)

//go:uintptrescapes
func crdtpEscapingSyscall6(trap, nargs, a1, a2, a3, a4, a5, a6 uintptr) (uintptr, uintptr, syscall.Errno) {
	return syscall.Syscall6(trap, nargs, a1, a2, a3, a4, a5, a6)
}

func crdtpCallbackPanic(kind string, recovered any) {
	fmt.Fprintf(os.Stderr, "gov8: panic in CRDTP %s callback: %v\n", kind, recovered)
	proc("gov8_host_panic_abort").Call()
	panic(recovered)
}

func crdtpCopyCallbackBytes(data, length uintptr) []byte {
	if length > uintptr(^uint(0)>>1) || (data == 0 && length != 0) {
		fatalHostMisuse("invalid CRDTP callback byte span")
		panic("unreachable after CRDTP callback byte-span failure")
	}
	result := make([]byte, int(length))
	if length != 0 {
		copy(result, unsafe.Slice((*byte)(abiWordToPtr(data)), int(length)))
	}
	return result
}

func crdtpCopyDomainCommand(invocation *crdtpDomainInvocation, data, length uintptr) []byte {
	if length > uintptr(^uint(0)>>1) || (data == 0 && length != 0) {
		fatalHostMisuse("invalid CRDTP callback byte span")
		panic("unreachable after CRDTP callback byte-span failure")
	}
	if length <= uintptr(len(invocation.command)) {
		command := invocation.command[:int(length):int(length)]
		if length != 0 {
			copy(command, unsafe.Slice((*byte)(abiWordToPtr(data)), int(length)))
		}
		return command
	}
	return crdtpCopyCallbackBytes(data, length)
}

var crdtpDomainDispatcherCallback = syscall.NewCallback(func(
	idWord, commandData, commandLen, requestRaw, hasCallIDWord, callIDWord uintptr,
) (result uintptr) {
	defer func() {
		if recovered := recover(); recovered != nil {
			crdtpCallbackPanic("domain", recovered)
		}
	}()
	id := uint64(idWord)
	crdtpCallbacks.RLock()
	entry := crdtpCallbacks.domains[id]
	if entry != nil {
		entry.active.Add(1)
	}
	crdtpCallbacks.RUnlock()
	if entry == nil || entry.handler == nil || requestRaw == 0 {
		fatalHostMisuse("unknown CRDTP domain callback %d", id)
		panic("unreachable after unknown CRDTP domain callback")
	}
	defer entry.active.Add(-1)
	if hasCallIDWord != 0 && hasCallIDWord != 1 {
		fatalHostMisuse("invalid CRDTP callback call-ID presence %d", hasCallIDWord)
		panic("unreachable after invalid CRDTP callback call-ID presence")
	}
	invocation := &crdtpDomainInvocation{}
	invocation.frame = crdtpDomainFrame{
		id: id, request: requestRaw, tid: currentThreadID(),
		hasCallID: hasCallIDWord == 1, callID: int32(callIDWord),
	}
	invocation.request.frame = &invocation.frame
	invocation.responder.frame = &invocation.frame
	invocation.frame.state.Store(1)
	defer func() { invocation.frame.state.Store(0) }()
	if entry.handler.Dispatch(crdtpCopyDomainCommand(invocation, commandData, commandLen),
		&invocation.request, &invocation.responder) {
		return 1
	}
	return 0
})

var crdtpDomainDropCallback = syscall.NewCallback(func(idWord uintptr) uintptr {
	defer func() {
		if recovered := recover(); recovered != nil {
			crdtpCallbackPanic("domain-drop", recovered)
		}
	}()
	id := uint64(idWord)
	crdtpCallbacks.Lock()
	entry := crdtpCallbacks.domains[id]
	delete(crdtpCallbacks.domains, id)
	crdtpCallbacks.Unlock()
	if entry == nil {
		fatalHostMisuse("unknown CRDTP domain drop %d", id)
		panic("unreachable after unknown CRDTP domain drop")
	}
	if dropper, ok := entry.handler.(CRDTPCallbackDropper); ok {
		dropper.CRDTPCallbackDropped()
	}
	return 0
})

var crdtpFallthroughCallback = syscall.NewCallback(func(
	idWord, callIDWord, methodData, methodLen, messageData, messageLen, associatedData, associatedLen uintptr,
) uintptr {
	defer func() {
		if recovered := recover(); recovered != nil {
			crdtpCallbackPanic("fallthrough", recovered)
		}
	}()
	id := uint64(idWord)
	crdtpCallbacks.RLock()
	entry := crdtpCallbacks.fall[id]
	if entry != nil {
		entry.active.Add(1)
	}
	crdtpCallbacks.RUnlock()
	if entry == nil || entry.handler == nil {
		fatalHostMisuse("unknown CRDTP fallthrough callback %d", id)
		panic("unreachable after unknown CRDTP fallthrough callback")
	}
	defer entry.active.Add(-1)
	entry.handler.FallThrough(int32(callIDWord),
		crdtpCopyCallbackBytes(methodData, methodLen),
		crdtpCopyCallbackBytes(messageData, messageLen),
		crdtpCopyCallbackBytes(associatedData, associatedLen))
	return 0
})

var crdtpFallthroughDropCallback = syscall.NewCallback(func(idWord uintptr) uintptr {
	defer func() {
		if recovered := recover(); recovered != nil {
			crdtpCallbackPanic("fallthrough-drop", recovered)
		}
	}()
	id := uint64(idWord)
	crdtpCallbacks.Lock()
	entry := crdtpCallbacks.fall[id]
	delete(crdtpCallbacks.fall, id)
	crdtpCallbacks.Unlock()
	if entry == nil {
		fatalHostMisuse("unknown CRDTP fallthrough drop %d", id)
		panic("unreachable after unknown CRDTP fallthrough drop")
	}
	if dropper, ok := entry.handler.(CRDTPCallbackDropper); ok {
		dropper.CRDTPCallbackDropped()
	}
	return 0
})

var crdtpChannelCallback = syscall.NewCallback(func(
	idWord, kindWord, callIDWord, messageRaw, serializedData, serializedLen uintptr,
) uintptr {
	defer func() {
		if recovered := recover(); recovered != nil {
			crdtpCallbackPanic("channel", recovered)
		}
	}()
	id := uint64(idWord)
	crdtpCallbacks.RLock()
	entry := crdtpCallbacks.channels[id]
	if entry != nil {
		entry.active.Add(1)
	}
	crdtpCallbacks.RUnlock()
	if entry == nil || entry.handler == nil {
		fatalHostMisuse("unknown CRDTP channel callback %d", id)
		panic("unreachable after unknown CRDTP channel callback")
	}
	defer entry.active.Add(-1)
	switch kindWord {
	case 0:
		if messageRaw == 0 {
			fatalHostMisuse("CRDTP response callback received null message")
			panic("unreachable after null CRDTP response")
		}
		message := &CRDTPSerializable{
			handle: messageRaw, callbackView: serializedData, callbackViewLen: serializedLen,
		}
		defer message.clearCallbackView()
		entry.handler.SendProtocolResponse(int32(callIDWord), message)
	case 1:
		if messageRaw == 0 {
			fatalHostMisuse("CRDTP notification callback received null message")
			panic("unreachable after null CRDTP notification")
		}
		message := &CRDTPSerializable{
			handle: messageRaw, callbackView: serializedData, callbackViewLen: serializedLen,
		}
		defer message.clearCallbackView()
		entry.handler.SendProtocolNotification(message)
	case 2:
		if messageRaw != 0 || serializedData != 0 || serializedLen != 0 {
			fatalHostMisuse("CRDTP flush callback received a message")
			panic("unreachable after invalid CRDTP flush")
		}
		entry.handler.FlushProtocolNotifications()
	default:
		fatalHostMisuse("invalid CRDTP channel callback kind %d", kindWord)
		panic("unreachable after invalid CRDTP channel kind")
	}
	return 0
})

var crdtpChannelDropCallback = syscall.NewCallback(func(idWord uintptr) uintptr {
	defer func() {
		if recovered := recover(); recovered != nil {
			crdtpCallbackPanic("channel-drop", recovered)
		}
	}()
	id := uint64(idWord)
	crdtpCallbacks.Lock()
	entry := crdtpCallbacks.channels[id]
	delete(crdtpCallbacks.channels, id)
	crdtpCallbacks.Unlock()
	if entry == nil {
		fatalHostMisuse("unknown CRDTP channel drop %d", id)
		panic("unreachable after unknown CRDTP channel drop")
	}
	if dropper, ok := entry.handler.(CRDTPCallbackDropper); ok {
		dropper.CRDTPCallbackDropped()
	}
	return 0
})

func ensureCRDTPDispatchers() error {
	crdtpDispatchersOnce.Do(func() {
		crdtpDispatchersErr = callErr("CRDTPDispatcher.SetCallbacks",
			proc("gov8_crdtp_dispatcher_set_callbacks_v2"), crdtpDomainDispatcherCallback,
			crdtpDomainDropCallback, crdtpFallthroughCallback,
			crdtpFallthroughDropCallback, crdtpChannelCallback, crdtpChannelDropCallback)
		if crdtpDispatchersErr == nil {
			crdtpUberDispatchAddr = proc("gov8_crdtp_uber_dispatch").Addr()
			crdtpDomainSendResponseAddr = proc("gov8_crdtp_domain_send_response").Addr()
		}
	})
	return crdtpDispatchersErr
}

// CRDTPFrontendChannelHandle owns the native FrontendChannel bridge.
type CRDTPFrontendChannelHandle struct {
	mu     sync.Mutex
	handle uintptr
	id     uint64
	closed bool
}

func NewCRDTPFrontendChannel(handler CRDTPFrontendChannel) (*CRDTPFrontendChannelHandle, error) {
	if handler == nil {
		return nil, errors.New("gov8: nil CRDTP frontend channel")
	}
	if err := ensureCRDTPDispatchers(); err != nil {
		return nil, err
	}
	crdtpCallbacks.Lock()
	id, err := crdtpNextIDLocked()
	if err == nil {
		crdtpCallbacks.channels[id] = &crdtpChannelEntry{handler: handler}
	}
	crdtpCallbacks.Unlock()
	if err != nil {
		return nil, err
	}
	var out uintptr
	status, _, _ := proc("gov8_crdtp_channel_new").Call(
		uintptr(id), uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(&out)
	if int64(status) < 0 {
		crdtpCallbacks.Lock()
		delete(crdtpCallbacks.channels, id)
		crdtpCallbacks.Unlock()
		return nil, shimError("NewCRDTPFrontendChannel", status)
	}
	return &CRDTPFrontendChannelHandle{handle: out, id: id}, nil
}

func (c *CRDTPFrontendChannelHandle) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	crdtpCallbacks.Lock()
	entry := crdtpCallbacks.channels[c.id]
	if entry == nil {
		crdtpCallbacks.Unlock()
		c.mu.Unlock()
		return errors.New("gov8: CRDTP channel registry entry is missing")
	}
	if entry.dispatchers != 0 || entry.active.Load() != 0 {
		crdtpCallbacks.Unlock()
		c.mu.Unlock()
		return errors.New("gov8: CRDTP channel is still attached or active")
	}
	crdtpCallbacks.Unlock()
	c.closed = true
	handle := c.handle
	c.handle = 0
	c.mu.Unlock()
	return callErr("CRDTPFrontendChannel.Close", proc("gov8_crdtp_channel_delete"), handle)
}

// CRDTPUberDispatcher synchronously routes protocol messages to wired domains.
// It must be closed before its channel.
type CRDTPUberDispatcher struct {
	mu      sync.Mutex
	handle  uintptr
	channel *CRDTPFrontendChannelHandle
	closed  bool
	busy    bool
	domains map[string]uint64
}

func NewCRDTPUberDispatcher(channel *CRDTPFrontendChannelHandle) (*CRDTPUberDispatcher, error) {
	if channel == nil {
		return nil, errors.New("gov8: nil CRDTP frontend channel")
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if channel.closed || channel.handle == 0 {
		return nil, errCRDTPClosed
	}
	var out uintptr
	status, _, _ := proc("gov8_crdtp_uber_new").Call(
		channel.handle, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(&out)
	if int64(status) < 0 {
		return nil, shimError("NewCRDTPUberDispatcher", status)
	}
	crdtpCallbacks.Lock()
	entry := crdtpCallbacks.channels[channel.id]
	if entry == nil {
		crdtpCallbacks.Unlock()
		_ = callErr("CRDTPUberDispatcher.Close", proc("gov8_crdtp_uber_delete"), out)
		return nil, errors.New("gov8: CRDTP channel registry entry is missing")
	}
	entry.dispatchers++
	crdtpCallbacks.Unlock()
	return &CRDTPUberDispatcher{handle: out, channel: channel, domains: map[string]uint64{}}, nil
}

func (d *CRDTPUberDispatcher) WireDomain(domain string, handler CRDTPDomainDispatcher) error {
	if d == nil || handler == nil {
		return errors.New("gov8: nil CRDTP dispatcher or domain handler")
	}
	d.mu.Lock()
	if d.closed || d.handle == 0 {
		d.mu.Unlock()
		return errCRDTPClosed
	}
	if d.busy {
		d.mu.Unlock()
		return errors.New("gov8: CRDTP dispatcher is active")
	}
	if _, exists := d.domains[domain]; exists {
		d.mu.Unlock()
		return errors.New("gov8: CRDTP domain is already wired")
	}
	d.busy = true
	handle := d.handle
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		d.busy = false
		d.mu.Unlock()
	}()
	crdtpCallbacks.Lock()
	id, err := crdtpNextIDLocked()
	if err == nil {
		crdtpCallbacks.domains[id] = &crdtpDomainEntry{handler: handler, owner: d}
	}
	crdtpCallbacks.Unlock()
	if err != nil {
		return err
	}
	bytes := []byte(domain)
	var consumed int32
	status, _, _ := proc("gov8_crdtp_uber_wire").Call(handle, slicePointer(bytes),
		uintptr(len(bytes)), uintptr(id), uintptr(unsafe.Pointer(&consumed)))
	runtime.KeepAlive(bytes)
	if consumed == 0 {
		crdtpCallbacks.Lock()
		delete(crdtpCallbacks.domains, id)
		crdtpCallbacks.Unlock()
	} else if consumed != 1 {
		return errors.New("gov8: invalid CRDTP domain ownership state")
	}
	if int64(status) < 0 {
		return shimError("CRDTPUberDispatcher.WireDomain", status)
	}
	if consumed != 1 {
		return errors.New("gov8: native CRDTP dispatcher did not adopt domain")
	}
	d.mu.Lock()
	d.domains[domain] = id
	d.mu.Unlock()
	return nil
}

func (d *CRDTPUberDispatcher) Dispatch(message *CRDTPDispatchable) error {
	if d == nil || message == nil {
		return errors.New("gov8: nil CRDTP dispatcher or message")
	}
	d.mu.Lock()
	if d.closed || d.handle == 0 {
		d.mu.Unlock()
		return errCRDTPClosed
	}
	if d.busy {
		d.mu.Unlock()
		return errors.New("gov8: CRDTP dispatcher is already active")
	}
	d.busy = true
	handle := d.handle
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		d.busy = false
		d.mu.Unlock()
	}()
	message.mu.Lock()
	if message.closed || message.handle == 0 {
		message.mu.Unlock()
		return errCRDTPClosed
	}
	if message.active {
		message.mu.Unlock()
		return errors.New("gov8: CRDTP Dispatchable is active")
	}
	message.active = true
	messageHandle := message.handle
	message.mu.Unlock()
	defer func() {
		message.mu.Lock()
		message.active = false
		message.mu.Unlock()
	}()
	status, _, _ := syscall.Syscall(crdtpUberDispatchAddr, 2, handle, messageHandle, 0)
	if int64(status) < 0 {
		return shimError("CRDTPUberDispatcher.Dispatch", status)
	}
	return nil
}

func (d *CRDTPUberDispatcher) Close() error {
	if d == nil {
		return nil
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	if d.busy {
		d.mu.Unlock()
		return errors.New("gov8: CRDTP dispatcher is active")
	}
	d.closed = true
	handle := d.handle
	d.handle = 0
	channel := d.channel
	d.mu.Unlock()
	err := callErr("CRDTPUberDispatcher.Close", proc("gov8_crdtp_uber_delete"), handle)
	crdtpCallbacks.Lock()
	if entry := crdtpCallbacks.channels[channel.id]; entry != nil {
		entry.dispatchers--
	}
	crdtpCallbacks.Unlock()
	return err
}

type crdtpDomainFrame struct {
	id           uint64
	request      uintptr
	tid          uint32
	hasCallID    bool
	callID       int32
	state        atomic.Uint32 // 0=expired, 1=active, 2=active send in progress
	sendConsumed [2]int32
}

type crdtpDomainInvocation struct {
	frame     crdtpDomainFrame
	request   CRDTPDispatchRequest
	responder CRDTPDomainResponder
	command   [8]byte
}

func (f *crdtpDomainFrame) check() error {
	if f == nil || f.state.Load() == 0 || f.request == 0 {
		return errors.New("gov8: CRDTP domain callback value is no longer active")
	}
	if currentThreadID() != f.tid {
		return errors.New("gov8: CRDTP domain callback used from another thread")
	}
	return nil
}

type CRDTPDispatchRequest struct{ frame *crdtpDomainFrame }

func (r *CRDTPDispatchRequest) CallID() (int32, bool, error) {
	if r == nil || r.frame == nil {
		return 0, false, errors.New("gov8: nil CRDTP dispatch request")
	}
	if err := r.frame.check(); err != nil {
		return 0, false, err
	}
	return r.frame.callID, r.frame.hasCallID, nil
}

func (r *CRDTPDispatchRequest) Method() ([]byte, error)         { return r.bytes(0) }
func (r *CRDTPDispatchRequest) SessionID() ([]byte, error)      { return r.bytes(1) }
func (r *CRDTPDispatchRequest) Params() ([]byte, error)         { return r.bytes(2) }
func (r *CRDTPDispatchRequest) AssociatedData() ([]byte, error) { return r.bytes(3) }

func (r *CRDTPDispatchRequest) bytes(kind uintptr) ([]byte, error) {
	if r == nil || r.frame == nil {
		return nil, errors.New("gov8: nil CRDTP dispatch request")
	}
	if err := r.frame.check(); err != nil {
		return nil, err
	}
	var out uintptr
	status, _, _ := proc("gov8_crdtp_request_bytes").Call(
		r.frame.request, kind, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(&out)
	if int64(status) < 0 {
		return nil, shimError("CRDTPDispatchRequest.Accessor", status)
	}
	return takeCRDTPBytes(out)
}

type CRDTPDomainResponder struct{ frame *crdtpDomainFrame }

// SendResponse synchronously sends and consumes response and optional result.
// Both handles are detached before native entry because channel delivery may
// reenter Go before this call returns.
func (r *CRDTPDomainResponder) SendResponse(callID int32, response *CRDTPDispatchResponse, result *CRDTPSerializable) error {
	if r == nil || r.frame == nil || response == nil {
		return errors.New("gov8: nil CRDTP responder or response")
	}
	if err := r.frame.check(); err != nil {
		return err
	}
	response.mu.Lock()
	responseHandle, err := response.withHandle()
	if err != nil {
		response.mu.Unlock()
		return err
	}
	var resultHandle uintptr
	if result != nil {
		result.mu.Lock()
		resultHandle, err = result.withHandle()
		if err != nil {
			result.mu.Unlock()
			response.mu.Unlock()
			return err
		}
	}
	response.consumed = true
	response.handle = 0
	response.mu.Unlock()
	if result != nil {
		result.consumed = true
		result.handle = 0
		result.mu.Unlock()
	}
	// Native delivery reenters Go before returning. The callback frame is
	// already heap-stable, so its common-path ownership result remains valid if
	// the Go stack grows. A nested send must not alias the outer native call's
	// still-live outputs, so reentrant calls retain the independent heap slot.
	consumed := &r.frame.sendConsumed
	if !r.frame.state.CompareAndSwap(1, 2) {
		consumed = new([2]int32)
	} else {
		r.frame.sendConsumed = [2]int32{}
		defer func() { r.frame.state.Store(1) }()
	}
	status, _, _ := crdtpEscapingSyscall6(crdtpDomainSendResponseAddr, 6, uintptr(r.frame.id),
		uintptr(callID), responseHandle, resultHandle,
		uintptr(unsafe.Pointer(&consumed[0])), uintptr(unsafe.Pointer(&consumed[1])))
	runtime.KeepAlive(consumed)
	if int64(status) < 0 {
		return shimError("CRDTPDomainResponder.SendResponse", status)
	}
	if consumed[0] != 1 || (result != nil && consumed[1] != 1) ||
		(result == nil && consumed[1] != 0) {
		return errors.New("gov8: invalid CRDTP send-response ownership result")
	}
	return nil
}

func NewCRDTPDispatchableWithFallthrough(cbor, associatedData []byte, callback CRDTPFallthroughCallback) (*CRDTPDispatchable, error) {
	if callback == nil {
		return nil, errors.New("gov8: nil CRDTP fallthrough callback")
	}
	if err := ensureCRDTPDispatchers(); err != nil {
		return nil, err
	}
	crdtpCallbacks.Lock()
	id, err := crdtpNextIDLocked()
	if err == nil {
		crdtpCallbacks.fall[id] = &crdtpFallthroughEntry{handler: callback}
	}
	crdtpCallbacks.Unlock()
	if err != nil {
		return nil, err
	}
	var out uintptr
	status, _, _ := proc("gov8_crdtp_dispatchable_fallthrough_new").Call(slicePointer(cbor),
		uintptr(len(cbor)), slicePointer(associatedData), uintptr(len(associatedData)),
		uintptr(id), uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(cbor)
	runtime.KeepAlive(associatedData)
	runtime.KeepAlive(&out)
	if int64(status) < 0 {
		crdtpCallbacks.Lock()
		delete(crdtpCallbacks.fall, id)
		crdtpCallbacks.Unlock()
		return nil, shimError("NewCRDTPDispatchableWithFallthrough", status)
	}
	return &CRDTPDispatchable{handle: out}, nil
}
