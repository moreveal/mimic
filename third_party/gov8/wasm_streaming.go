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
	"unicode/utf8"
	"unsafe"
)

// WasmStreamingCallback is the embedder injection point for
// WebAssembly.compileStreaming. source and callbackScope are callback-local;
// stream is owned by the callback and must eventually be finished, aborted,
// or closed.
type WasmStreamingCallback func(callbackScope *CallbackScope, source Value, stream *WasmStreaming)

// ModuleCachingCallback runs synchronously during Finish when cached compiled
// bytes were announced. The interface is valid only during the callback.
type ModuleCachingCallback func(*ModuleCachingInterface)

// WasmModuleCompilationCallback runs once on the isolate thread after an
// experimental asynchronous compilation resolves. Exactly one of Module and
// Error is populated, and all local values are callback-local.
type WasmModuleCompilationCallback func(*WasmModuleCompilationResult)

// WasmSerializationCallback may run on a background thread. It receives an
// owned compiled module; the callback must close it or transfer its ownership.
type WasmSerializationCallback func(*CompiledWasmModule)

type wasmStreamingState uint8

const (
	wasmStreamingOpen wasmStreamingState = iota
	wasmStreamingFinished
	wasmStreamingAborted
	wasmStreamingClosed
)

// WasmStreaming is V8's streaming compilation sink. Unlike
// WasmModuleCompilation it is isolate/thread-affine.
type WasmStreaming struct {
	mu          sync.Mutex
	iso         *Isolate
	handle      uintptr
	state       wasmStreamingState
	received    bool
	cacheMarked bool
}

// ModuleCachingInterface exposes the completed wire bytes and accepts a
// candidate serialized module during a caching callback. Go restricts the
// setter to one call because repeated-set behavior is not characterized by
// the pinned API.
type ModuleCachingInterface struct {
	mu        sync.Mutex
	handle    uintptr
	active    bool
	setCalled bool
}

// WasmModuleCompilationResult is valid only while its callback is running.
type WasmModuleCompilationResult struct {
	CallbackScope *CallbackScope
	Module        *WasmModuleObject
	Error         Value
}

// WasmModuleCompilation is V8's experimental, movable asynchronous compiler.
// Calls are serialized; OnBytesReceived, SetURL, Abort, and Close may be made
// from any thread. Finish must run on the target isolate's owning thread.
type WasmModuleCompilation struct {
	mu              sync.Mutex
	handle          uintptr
	state           wasmStreamingState
	received        bool
	cacheMarked     bool
	serializationID uint64
}

type wasmStreamingEntry struct {
	iso      *Isolate
	callback WasmStreamingCallback
}

type wasmResolutionEntry struct {
	iso      *Isolate
	callback WasmModuleCompilationCallback
}

// wasmResolutionCallbackFrame keeps all callback-local wrappers in one heap
// object. The callback API exposes pointers to these values, so separate local
// variables would each escape independently. Their observable lifetime is
// unchanged: every wrapper is invalidated immediately after the callback.
type wasmResolutionCallbackFrame struct {
	scope         Scope
	callbackScope CallbackScope
	result        WasmModuleCompilationResult
	module        WasmModuleObject
}

type wasmStreamingFrame struct {
	id           uint64
	isolate      uintptr
	contextWire  uintptr
	scopeWire    uintptr
	sourceWire   uintptr
	streamHandle uintptr
}

type wasmResolutionFrame struct {
	id          uint64
	isolate     uintptr
	contextWire uintptr
	scopeWire   uintptr
	moduleWire  uintptr
	errorWire   uintptr
}

var wasmStreamingRegistry = struct {
	sync.Mutex
	next            atomic.Uint64
	streams         map[uint64]*wasmStreamingEntry
	streamByISO     map[*Isolate]uint64
	caching         map[uint64]ModuleCachingCallback
	resolutions     map[uint64]wasmResolutionEntry
	serializations  map[uint64]WasmSerializationCallback
	activeStreams   map[*Isolate]uint64
	activeCallbacks map[*Isolate]uint64
}{
	streams: make(map[uint64]*wasmStreamingEntry), streamByISO: make(map[*Isolate]uint64),
	caching: make(map[uint64]ModuleCachingCallback), resolutions: make(map[uint64]wasmResolutionEntry),
	serializations:  make(map[uint64]WasmSerializationCallback),
	activeStreams:   make(map[*Isolate]uint64),
	activeCallbacks: make(map[*Isolate]uint64),
}

func beginWasmCallback(i *Isolate) {
	wasmStreamingRegistry.Lock()
	wasmStreamingRegistry.activeCallbacks[i]++
	wasmStreamingRegistry.Unlock()
}

func endWasmCallback(i *Isolate) {
	wasmStreamingRegistry.Lock()
	if count := wasmStreamingRegistry.activeCallbacks[i]; count <= 1 {
		delete(wasmStreamingRegistry.activeCallbacks, i)
	} else {
		wasmStreamingRegistry.activeCallbacks[i] = count - 1
	}
	wasmStreamingRegistry.Unlock()
}

func nextWasmCallbackID() (uint64, error) {
	for {
		current := wasmStreamingRegistry.next.Load()
		if current == math.MaxUint64 {
			return 0, errors.New("gov8: wasm callback registry exhausted")
		}
		if wasmStreamingRegistry.next.CompareAndSwap(current, current+1) {
			return current + 1, nil
		}
	}
}

var (
	wasmDispatchersOnce sync.Once
	wasmDispatchersErr  error
	wasmHotProcsOnce    sync.Once
	wasmCompilationNew  uintptr
	wasmCompilationFeed uintptr
	wasmCompilationDone uintptr
	wasmPumpMessageLoop uintptr
)

func ensureWasmHotProcs() {
	wasmHotProcsOnce.Do(func() {
		wasmCompilationNew = proc("gov8_wmc_new").Addr()
		wasmCompilationFeed = proc("gov8_wmc_on_bytes").Addr()
		wasmCompilationDone = proc("gov8_wmc_finish").Addr()
		wasmPumpMessageLoop = proc("gov8_ws_pump_message_loop").Addr()
	})
}

var wasmStreamingDispatcher = syscall.NewCallback(func(frame *wasmStreamingFrame) (result uintptr) {
	defer wasmCallbackPanicBoundary("streaming callback")
	wasmStreamingRegistry.Lock()
	entry := wasmStreamingRegistry.streams[frame.id]
	wasmStreamingRegistry.Unlock()
	if entry == nil || entry.iso == nil || entry.callback == nil || entry.iso.handle != frame.isolate {
		fatalHostMisuse("wasm streaming callback for unknown handle %d", frame.id)
		return 1
	}
	if err := entry.iso.check(); err != nil {
		fatalHostMisuse("wasm streaming callback lifecycle: %v", err)
		return 1
	}
	beginWasmCallback(entry.iso)
	defer endWasmCallback(entry.iso)
	borrowed := &Scope{iso: entry.iso, handle: frame.scopeWire, borrowed: true}
	callbackScope := &CallbackScope{iso: entry.iso, sc: borrowed, ctxWire: frame.contextWire}
	stream := &WasmStreaming{iso: entry.iso, handle: frame.streamHandle, state: wasmStreamingOpen}
	wasmStreamingRegistry.Lock()
	wasmStreamingRegistry.activeStreams[entry.iso]++
	wasmStreamingRegistry.Unlock()
	defer func() {
		borrowed.closed = true
		callbackScope.iso = nil
		callbackScope.sc = nil
		callbackScope.ctxWire = 0
	}()
	entry.callback(callbackScope, callbackScope.wrap(frame.sourceWire), stream)
	return 0
})

var wasmCachingDispatcher = syscall.NewCallback(func(id, handle uintptr) (result uintptr) {
	defer wasmCallbackPanicBoundary("module caching callback")
	wasmStreamingRegistry.Lock()
	callback := wasmStreamingRegistry.caching[uint64(id)]
	wasmStreamingRegistry.Unlock()
	if callback == nil {
		fatalHostMisuse("wasm caching callback for unknown handle %d", id)
		return 1
	}
	iface := &ModuleCachingInterface{handle: handle, active: true}
	defer func() {
		iface.mu.Lock()
		iface.active = false
		iface.handle = 0
		iface.mu.Unlock()
	}()
	callback(iface)
	return 0
})

var wasmResolutionDispatcher = syscall.NewCallback(func(frame *wasmResolutionFrame) (result uintptr) {
	defer wasmCallbackPanicBoundary("module compilation callback")
	wasmStreamingRegistry.Lock()
	entry, found := wasmStreamingRegistry.resolutions[frame.id]
	valid := found && entry.iso != nil && entry.callback != nil && entry.iso.handle == frame.isolate
	if valid {
		delete(wasmStreamingRegistry.resolutions, frame.id)
		wasmStreamingRegistry.activeCallbacks[entry.iso]++
	}
	wasmStreamingRegistry.Unlock()
	if !valid {
		fatalHostMisuse("wasm resolution callback for unknown handle %d", frame.id)
		return 1
	}
	if err := entry.iso.check(); err != nil {
		endWasmCallback(entry.iso)
		fatalHostMisuse("wasm resolution callback lifecycle: %v", err)
		return 1
	}
	defer endWasmCallback(entry.iso)
	callbackFrame := &wasmResolutionCallbackFrame{}
	callbackFrame.scope = Scope{iso: entry.iso, handle: frame.scopeWire, borrowed: true}
	callbackFrame.callbackScope = CallbackScope{
		iso: entry.iso, sc: &callbackFrame.scope, ctxWire: frame.contextWire,
	}
	callbackFrame.result.CallbackScope = &callbackFrame.callbackScope
	if frame.moduleWire != 0 {
		callbackFrame.module.Value = Value{iso: entry.iso, sc: &callbackFrame.scope, h: frame.moduleWire}
		callbackFrame.result.Module = &callbackFrame.module
	} else {
		callbackFrame.result.Error = Value{iso: entry.iso, sc: &callbackFrame.scope, h: frame.errorWire}
	}
	defer func() {
		callbackFrame.scope.closed = true
		callbackFrame.callbackScope.iso = nil
		callbackFrame.callbackScope.sc = nil
		callbackFrame.callbackScope.ctxWire = 0
		callbackFrame.result.CallbackScope = nil
		callbackFrame.result.Module = nil
		callbackFrame.result.Error = Value{}
	}()
	entry.callback(&callbackFrame.result)
	return 0
})

const (
	wasmDropResolution    = 0
	wasmDropSerialization = 1
)

var wasmDropDispatcher = syscall.NewCallback(func(id, kind uintptr) uintptr {
	wasmStreamingRegistry.Lock()
	switch kind {
	case wasmDropResolution:
		delete(wasmStreamingRegistry.resolutions, uint64(id))
	case wasmDropSerialization:
		delete(wasmStreamingRegistry.serializations, uint64(id))
	}
	wasmStreamingRegistry.Unlock()
	return 0
})

var wasmSerializationDispatcher = syscall.NewCallback(func(id, compiled uintptr) (result uintptr) {
	defer wasmCallbackPanicBoundary("wasm serialization callback")
	wasmStreamingRegistry.Lock()
	callback := wasmStreamingRegistry.serializations[uint64(id)]
	wasmStreamingRegistry.Unlock()
	if callback == nil || compiled == 0 {
		fatalHostMisuse("wasm serialization callback for unknown handle %d", id)
		return 1
	}
	callback(&CompiledWasmModule{handle: compiled})
	return 0
})

func wasmCallbackPanicBoundary(name string) {
	if recovered := recover(); recovered != nil {
		fmt.Fprintf(os.Stderr, "gov8: panic in %s: %v\n", name, recovered)
		proc("gov8_host_panic_abort").Call()
		panic(recovered)
	}
}

func ensureWasmDispatchers() error {
	wasmDispatchersOnce.Do(func() {
		wasmDispatchersErr = callErr("WasmStreaming.SetDispatchers", proc("gov8_ws_set_dispatchers"),
			wasmStreamingDispatcher, wasmCachingDispatcher, wasmResolutionDispatcher,
			wasmDropDispatcher, wasmSerializationDispatcher)
	})
	return wasmDispatchersErr
}

// SetWasmStreamingCallback installs the isolate's compileStreaming callback.
// Call ClearWasmStreamingCallback, or ReleaseIsolateHostState, before closing
// the isolate.
func (i *Isolate) SetWasmStreamingCallback(callback WasmStreamingCallback) error {
	if callback == nil {
		return errors.New("gov8: wasm streaming callback is required")
	}
	if err := i.check(); err != nil {
		return err
	}
	i.mu.Lock()
	contextsCreated := i.contextsCreated
	i.mu.Unlock()
	if contextsCreated {
		return errors.New("gov8: wasm streaming callback must be installed before creating a context")
	}
	if err := ensureWasmDispatchers(); err != nil {
		return err
	}
	id, err := nextWasmCallbackID()
	if err != nil {
		return err
	}
	wasmStreamingRegistry.Lock()
	if wasmStreamingRegistry.streamByISO[i] != 0 {
		wasmStreamingRegistry.Unlock()
		return errors.New("gov8: wasm streaming callback already installed")
	}
	wasmStreamingRegistry.streams[id] = &wasmStreamingEntry{iso: i, callback: callback}
	wasmStreamingRegistry.streamByISO[i] = id
	wasmStreamingRegistry.Unlock()
	if err := callErr("Isolate.SetWasmStreamingCallback", proc("gov8_ws_set_callback"), i.handleAssumingCheck(), uintptr(id)); err != nil {
		wasmStreamingRegistry.Lock()
		delete(wasmStreamingRegistry.streams, id)
		delete(wasmStreamingRegistry.streamByISO, i)
		wasmStreamingRegistry.Unlock()
		return err
	}
	return nil
}

// ClearWasmStreamingCallback removes the installed streaming callback. It is
// safe when no callback is installed.
func (i *Isolate) ClearWasmStreamingCallback() error {
	if err := i.check(); err != nil {
		return err
	}
	return releaseWasmStreamingHostState(i)
}

// releaseWasmStreamingHostState is called by ReleaseIsolateHostState.
func releaseWasmStreamingHostState(i *Isolate) error {
	wasmStreamingRegistry.Lock()
	if wasmStreamingRegistry.activeCallbacks[i] != 0 {
		wasmStreamingRegistry.Unlock()
		return errors.New("gov8: cannot release isolate host state from an active wasm callback")
	}
	if wasmStreamingRegistry.activeStreams[i] != 0 {
		wasmStreamingRegistry.Unlock()
		return errors.New("gov8: cannot release isolate host state with active wasm streams")
	}
	for _, entry := range wasmStreamingRegistry.resolutions {
		if entry.iso == i {
			wasmStreamingRegistry.Unlock()
			return errors.New("gov8: cannot release isolate host state with pending wasm compilation")
		}
	}
	id := wasmStreamingRegistry.streamByISO[i]
	wasmStreamingRegistry.Unlock()
	if id == 0 {
		return nil
	}
	if err := callErr("Isolate.ClearWasmStreamingCallback", proc("gov8_ws_clear_callback"), i.handle); err != nil {
		return err
	}
	wasmStreamingRegistry.Lock()
	if wasmStreamingRegistry.streamByISO[i] == id {
		delete(wasmStreamingRegistry.streamByISO, i)
		delete(wasmStreamingRegistry.streams, id)
	}
	wasmStreamingRegistry.Unlock()
	return nil
}

func releaseActiveWasmStream(i *Isolate) {
	wasmStreamingRegistry.Lock()
	if count := wasmStreamingRegistry.activeStreams[i]; count <= 1 {
		delete(wasmStreamingRegistry.activeStreams, i)
	} else {
		wasmStreamingRegistry.activeStreams[i] = count - 1
	}
	wasmStreamingRegistry.Unlock()
}

func (s *WasmStreaming) lockOpen(operation string) error {
	if s == nil || s.iso == nil {
		return errors.New("gov8: nil wasm streaming handle")
	}
	if s.state != wasmStreamingOpen || s.handle == 0 {
		return fmt.Errorf("gov8: %s on completed wasm stream", operation)
	}
	if err := s.iso.check(); err != nil {
		return err
	}
	return nil
}

// OnBytesReceived feeds one chunk. Even an empty chunk establishes call order
// and prevents later SetHasCompiledModuleBytes.
func (s *WasmStreaming) OnBytesReceived(data []byte) error {
	if s == nil {
		return errors.New("gov8: nil wasm streaming handle")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.lockOpen("OnBytesReceived"); err != nil {
		return err
	}
	if err := callErr("WasmStreaming.OnBytesReceived", proc("gov8_ws_on_bytes"), s.handle,
		slicePointer(data), uintptr(len(data))); err != nil {
		return err
	}
	runtime.KeepAlive(data)
	s.received = true
	return nil
}

// SetURL sets the source URL before completion.
func (s *WasmStreaming) SetURL(url string) error {
	if s == nil {
		return errors.New("gov8: nil wasm streaming handle")
	}
	if !utf8.ValidString(url) {
		return errors.New("gov8: wasm source URL is not valid UTF-8")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.lockOpen("SetURL"); err != nil {
		return err
	}
	bytes := []byte(url)
	err := callErr("WasmStreaming.SetURL", proc("gov8_ws_set_url"), s.handle,
		slicePointer(bytes), uintptr(len(bytes)))
	runtime.KeepAlive(bytes)
	return err
}

// SetHasCompiledModuleBytes selects cache mode. Calling it after any byte
// notification is rejected in Go before V8's fatal CHECK boundary.
func (s *WasmStreaming) SetHasCompiledModuleBytes() error {
	if s == nil {
		return errors.New("gov8: nil wasm streaming handle")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.lockOpen("SetHasCompiledModuleBytes"); err != nil {
		return err
	}
	if s.received {
		return errors.New("gov8: SetHasCompiledModuleBytes must be called before OnBytesReceived")
	}
	if s.cacheMarked {
		return errors.New("gov8: compiled module bytes already announced")
	}
	if err := callErr("WasmStreaming.SetHasCompiledModuleBytes", proc("gov8_ws_set_has_cache"), s.handle); err != nil {
		return err
	}
	s.cacheMarked = true
	return nil
}

func registerWasmCachingCallback(callback ModuleCachingCallback) (uint64, error) {
	if callback == nil {
		return 0, nil
	}
	id, err := nextWasmCallbackID()
	if err != nil {
		return 0, err
	}
	wasmStreamingRegistry.Lock()
	wasmStreamingRegistry.caching[id] = callback
	wasmStreamingRegistry.Unlock()
	return id, nil
}

func dropWasmCachingCallback(id uint64) {
	if id == 0 {
		return
	}
	wasmStreamingRegistry.Lock()
	delete(wasmStreamingRegistry.caching, id)
	wasmStreamingRegistry.Unlock()
}

// Finish consumes the stream. callback is required in cache mode and must be
// nil otherwise.
func (s *WasmStreaming) Finish(callback ModuleCachingCallback) error {
	if s == nil {
		return errors.New("gov8: nil wasm streaming handle")
	}
	s.mu.Lock()
	if err := s.lockOpen("Finish"); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.cacheMarked != (callback != nil) {
		s.mu.Unlock()
		return errors.New("gov8: wasm caching callback must be supplied exactly in cache mode")
	}
	id, err := registerWasmCachingCallback(callback)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	handle := s.handle
	s.state = wasmStreamingFinished
	s.handle = 0
	s.mu.Unlock()
	defer dropWasmCachingCallback(id)
	err = callErr("WasmStreaming.Finish", proc("gov8_ws_finish"), handle, uintptr(id))
	releaseActiveWasmStream(s.iso)
	return err
}

// Abort consumes the stream. A non-nil exception must be a live local value
// from the same isolate.
func (s *WasmStreaming) Abort(exception *Value) error {
	if s == nil {
		return errors.New("gov8: nil wasm streaming handle")
	}
	s.mu.Lock()
	if err := s.lockOpen("Abort"); err != nil {
		s.mu.Unlock()
		return err
	}
	var wire uintptr
	if exception != nil {
		if exception.iso != s.iso {
			s.mu.Unlock()
			return foreignIsolate("exception")
		}
		if err := exception.check(); err != nil {
			s.mu.Unlock()
			return err
		}
		wire = exception.h
	}
	handle := s.handle
	s.state = wasmStreamingAborted
	s.handle = 0
	s.mu.Unlock()
	err := callErr("WasmStreaming.Abort", proc("gov8_ws_abort"), handle, wire)
	releaseActiveWasmStream(s.iso)
	return err
}

// Close drops an unfinished stream without finishing or rejecting its promise.
func (s *WasmStreaming) Close() error {
	if s == nil {
		return errors.New("gov8: nil wasm streaming handle")
	}
	s.mu.Lock()
	if err := s.lockOpen("Close"); err != nil {
		s.mu.Unlock()
		return err
	}
	handle := s.handle
	s.state = wasmStreamingClosed
	s.handle = 0
	s.mu.Unlock()
	err := callErr("WasmStreaming.Close", proc("gov8_ws_dispose"), handle)
	releaseActiveWasmStream(s.iso)
	return err
}

// WireBytes returns a copy of the complete wasm wire bytes. It is callback-only.
func (m *ModuleCachingInterface) WireBytes() ([]byte, error) {
	if m == nil {
		return nil, errors.New("gov8: nil module caching interface")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.active || m.handle == 0 {
		return nil, errors.New("gov8: module caching interface is no longer active")
	}
	return m.wireBytesLocked()
}

func (m *ModuleCachingInterface) wireBytesLocked() ([]byte, error) {
	var length uintptr
	if err := callErr("ModuleCachingInterface.WireBytes", proc("gov8_ws_cache_wire"),
		m.handle, 0, 0, uintptr(unsafe.Pointer(&length))); err != nil {
		return nil, err
	}
	if length > uintptr(^uint(0)>>1) {
		return nil, errors.New("gov8: wasm cache wire bytes exceed Go slice capacity")
	}
	result := make([]byte, int(length))
	if length == 0 {
		return result, nil
	}
	if err := callErr("ModuleCachingInterface.WireBytes", proc("gov8_ws_cache_wire"),
		m.handle, slicePointer(result), length, uintptr(unsafe.Pointer(&length))); err != nil {
		return nil, err
	}
	return result, nil
}

// SetCachedCompiledModuleBytes offers one raw serialized candidate to V8 and
// mirrors rusty_v8's byte-slice API. V8 152 CHECK-fails rather than returning
// false for mismatched wire bytes or a truncated cache; callers with cache
// provenance should use SetCachedCompiledModule, which validates those fatal
// preconditions in Go. Repeated calls are rejected safely before FFI.
func (m *ModuleCachingInterface) SetCachedCompiledModuleBytes(bytes []byte) (bool, error) {
	if m == nil {
		return false, errors.New("gov8: nil module caching interface")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.active || m.handle == 0 {
		return false, errors.New("gov8: module caching interface is no longer active")
	}
	if m.setCalled {
		return false, errors.New("gov8: cached compiled module bytes already set")
	}
	return m.setCachedCompiledModuleBytesLocked(bytes)
}

func (m *ModuleCachingInterface) setCachedCompiledModuleBytesLocked(bytes []byte) (bool, error) {
	m.setCalled = true
	var accepted int32
	err := callErr("ModuleCachingInterface.SetCachedCompiledModuleBytes", proc("gov8_ws_cache_set"),
		m.handle, slicePointer(bytes), uintptr(len(bytes)), uintptr(unsafe.Pointer(&accepted)))
	runtime.KeepAlive(bytes)
	return accepted != 0, err
}

// NewWasmModuleCompilation begins an isolate-independent asynchronous compile.
func NewWasmModuleCompilation() (*WasmModuleCompilation, error) {
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	if err := ensureWasmDispatchers(); err != nil {
		return nil, err
	}
	ensureWasmHotProcs()
	handle, _, _ := syscall.Syscall(wasmCompilationNew, 0, 0, 0, 0)
	if handle == 0 {
		return nil, shimError("WasmModuleCompilation.New", 0)
	}
	return &WasmModuleCompilation{handle: handle, state: wasmStreamingOpen}, nil
}

func (c *WasmModuleCompilation) lockOpen(operation string) error {
	if c == nil || c.handle == 0 || c.state != wasmStreamingOpen {
		return fmt.Errorf("gov8: %s on completed wasm module compilation", operation)
	}
	return nil
}

// OnBytesReceived feeds one owned chunk and may run on any thread.
func (c *WasmModuleCompilation) OnBytesReceived(data []byte) error {
	if c == nil {
		return errors.New("gov8: nil wasm module compilation")
	}
	c.mu.Lock()
	if err := c.lockOpen("OnBytesReceived"); err != nil {
		c.mu.Unlock()
		return err
	}
	ensureWasmHotProcs()
	r1, _, _ := syscall.Syscall(wasmCompilationFeed, 3,
		c.handle, uintptr(sliceUnsafePointer(data)), uintptr(len(data)))
	runtime.KeepAlive(data)
	if int64(r1) < 0 {
		err := shimError("WasmModuleCompilation.OnBytesReceived", r1)
		c.mu.Unlock()
		return err
	}
	c.received = true
	c.mu.Unlock()
	return nil
}

// SetURL sets the compiled module source URL and may run on any thread.
func (c *WasmModuleCompilation) SetURL(url string) error {
	if c == nil {
		return errors.New("gov8: nil wasm module compilation")
	}
	if !utf8.ValidString(url) {
		return errors.New("gov8: wasm source URL is not valid UTF-8")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.lockOpen("SetURL"); err != nil {
		return err
	}
	bytes := []byte(url)
	err := callErr("WasmModuleCompilation.SetURL", proc("gov8_wmc_set_url"),
		c.handle, slicePointer(bytes), uintptr(len(bytes)))
	runtime.KeepAlive(bytes)
	return err
}

// SetHasCompiledModuleBytes selects cache mode. Any earlier byte notification,
// including an empty chunk, is rejected before V8's fatal boundary.
func (c *WasmModuleCompilation) SetHasCompiledModuleBytes() error {
	if c == nil {
		return errors.New("gov8: nil wasm module compilation")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.lockOpen("SetHasCompiledModuleBytes"); err != nil {
		return err
	}
	if c.received {
		return errors.New("gov8: SetHasCompiledModuleBytes must be called before OnBytesReceived")
	}
	if c.cacheMarked {
		return errors.New("gov8: compiled module bytes already announced")
	}
	if err := callErr("WasmModuleCompilation.SetHasCompiledModuleBytes", proc("gov8_wmc_set_has_cache"), c.handle); err != nil {
		return err
	}
	c.cacheMarked = true
	return nil
}

// SetMoreFunctionsCanBeSerializedCallback installs the background-safe
// serialization notification callback.
func (c *WasmModuleCompilation) SetMoreFunctionsCanBeSerializedCallback(callback WasmSerializationCallback) error {
	if c == nil {
		return errors.New("gov8: nil wasm module compilation")
	}
	if callback == nil {
		return errors.New("gov8: wasm serialization callback is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.lockOpen("SetMoreFunctionsCanBeSerializedCallback"); err != nil {
		return err
	}
	if c.serializationID != 0 {
		return errors.New("gov8: wasm serialization callback already installed")
	}
	id, err := nextWasmCallbackID()
	if err != nil {
		return err
	}
	wasmStreamingRegistry.Lock()
	wasmStreamingRegistry.serializations[id] = callback
	wasmStreamingRegistry.Unlock()
	if err := callErr("WasmModuleCompilation.SetMoreFunctionsCanBeSerializedCallback",
		proc("gov8_wmc_set_serialization_callback"), c.handle, uintptr(id)); err != nil {
		wasmStreamingRegistry.Lock()
		delete(wasmStreamingRegistry.serializations, id)
		wasmStreamingRegistry.Unlock()
		return err
	}
	c.serializationID = id
	return nil
}

// Finish consumes the compilation and schedules callback on the target
// isolate. cacheCallback is required exactly when cache mode was selected.
func (c *WasmModuleCompilation) Finish(scope *Scope, context *Context, cacheCallback ModuleCachingCallback,
	callback WasmModuleCompilationCallback) error {
	if c == nil {
		return errors.New("gov8: nil wasm module compilation")
	}
	if callback == nil {
		return errors.New("gov8: wasm module compilation callback is required")
	}
	if scope == nil {
		return errors.New("gov8: nil scope")
	}
	if err := scope.check(); err != nil {
		return err
	}
	if context == nil || context.iso != scope.iso {
		return foreignIsolate("context")
	}
	if err := context.check(); err != nil {
		return err
	}
	c.mu.Lock()
	if err := c.lockOpen("Finish"); err != nil {
		c.mu.Unlock()
		return err
	}
	if c.cacheMarked != (cacheCallback != nil) {
		c.mu.Unlock()
		return errors.New("gov8: wasm caching callback must be supplied exactly in cache mode")
	}
	cacheID, err := registerWasmCachingCallback(cacheCallback)
	if err != nil {
		c.mu.Unlock()
		return err
	}
	resolutionID, err := nextWasmCallbackID()
	if err != nil {
		c.mu.Unlock()
		dropWasmCachingCallback(cacheID)
		return err
	}
	wasmStreamingRegistry.Lock()
	wasmStreamingRegistry.resolutions[resolutionID] = wasmResolutionEntry{iso: scope.iso, callback: callback}
	wasmStreamingRegistry.Unlock()
	handle := c.handle
	c.handle = 0
	c.state = wasmStreamingFinished
	c.mu.Unlock()
	defer dropWasmCachingCallback(cacheID)
	ensureWasmHotProcs()
	r1, _, _ := syscall.Syscall6(wasmCompilationDone, 6, handle,
		scope.iso.handleAssumingCheck(), context.handle, scope.handle, uintptr(cacheID), uintptr(resolutionID))
	if int64(r1) < 0 {
		wasmStreamingRegistry.Lock()
		delete(wasmStreamingRegistry.resolutions, resolutionID)
		wasmStreamingRegistry.Unlock()
		return shimError("WasmModuleCompilation.Finish", r1)
	}
	return nil
}

// Abort consumes the compilation and may run on any thread.
func (c *WasmModuleCompilation) Abort() error {
	if c == nil {
		return errors.New("gov8: nil wasm module compilation")
	}
	c.mu.Lock()
	if err := c.lockOpen("Abort"); err != nil {
		c.mu.Unlock()
		return err
	}
	handle := c.handle
	c.handle = 0
	c.state = wasmStreamingAborted
	c.mu.Unlock()
	return callErr("WasmModuleCompilation.Abort", proc("gov8_wmc_abort"), handle)
}

// Close drops an unfinished compilation and may run on any thread.
func (c *WasmModuleCompilation) Close() error {
	if c == nil {
		return errors.New("gov8: nil wasm module compilation")
	}
	c.mu.Lock()
	if err := c.lockOpen("Close"); err != nil {
		c.mu.Unlock()
		return err
	}
	handle := c.handle
	c.handle = 0
	c.state = wasmStreamingClosed
	c.mu.Unlock()
	return callErr("WasmModuleCompilation.Close", proc("gov8_wmc_dispose"), handle)
}

// PumpMessageLoop executes at most one platform task for the isolate. It is
// required to deliver asynchronous WasmModuleCompilation resolutions.
func (i *Isolate) PumpMessageLoop(waitForWork bool) (bool, error) {
	if err := i.check(); err != nil {
		return false, err
	}
	var wait uintptr
	if waitForWork {
		wait = 1
	}
	ensureWasmHotProcs()
	r1, _, _ := syscall.Syscall(wasmPumpMessageLoop, 2, i.handleAssumingCheck(), wait, 0)
	if int64(r1) < 0 {
		return false, shimError("Isolate.PumpMessageLoop", r1)
	}
	return r1 != 0, nil
}
