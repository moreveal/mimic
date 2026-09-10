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
	"unicode/utf16"
	"unsafe"
)

// InspectorStringView is an owned 8-bit or UTF-16 inspector string view.
// The prefix distinguishes it from this package's existing V8 StringView.
// Ownership is the Go safety difference from rusty_v8's borrowed view.
type InspectorStringView struct {
	bytes []byte
	units []uint16
}

func NewInspectorStringView8(value []byte) InspectorStringView {
	copyValue := make([]byte, len(value))
	copy(copyValue, value)
	return InspectorStringView{bytes: copyValue}
}
func NewInspectorStringView16(value []uint16) InspectorStringView {
	copyValue := make([]uint16, len(value))
	copy(copyValue, value)
	return InspectorStringView{units: copyValue}
}
func EmptyInspectorStringView() InspectorStringView {
	return InspectorStringView{bytes: []byte{}}
}
func (v InspectorStringView) Is8Bit() bool  { return v.units == nil }
func (v InspectorStringView) IsEmpty() bool { return v.Len() == 0 }
func (v InspectorStringView) Len() int {
	if v.units != nil {
		return len(v.units)
	}
	return len(v.bytes)
}
func (v InspectorStringView) Characters8() ([]byte, bool) {
	if !v.Is8Bit() {
		return nil, false
	}
	return append([]byte(nil), v.bytes...), true
}
func (v InspectorStringView) Characters16() ([]uint16, bool) {
	if v.Is8Bit() {
		return nil, false
	}
	return append([]uint16(nil), v.units...), true
}
func (v InspectorStringView) String() string {
	if v.units != nil {
		return string(utf16.Decode(v.units))
	}
	runes := make([]rune, len(v.bytes))
	for i, b := range v.bytes {
		runes[i] = rune(b)
	}
	return string(runes)
}

func (v InspectorStringView) native() (is8 uintptr, data uintptr, length uintptr) {
	if v.units != nil {
		if len(v.units) > 0 {
			data = uintptr(unsafe.Pointer(&v.units[0]))
		}
		return 0, data, uintptr(len(v.units))
	}
	if len(v.bytes) > 0 {
		data = uintptr(unsafe.Pointer(&v.bytes[0]))
	}
	return 1, data, uintptr(len(v.bytes))
}

// InspectorStringBuffer owns a copied inspector string.
type InspectorStringBuffer struct{ view InspectorStringView }

func NewInspectorStringBuffer(source InspectorStringView) *InspectorStringBuffer {
	if source.Is8Bit() {
		b, _ := source.Characters8()
		return &InspectorStringBuffer{view: NewInspectorStringView8(b)}
	}
	u, _ := source.Characters16()
	return &InspectorStringBuffer{view: NewInspectorStringView16(u)}
}
func (b *InspectorStringBuffer) StringView() InspectorStringView {
	if b == nil {
		return EmptyInspectorStringView()
	}
	return NewInspectorStringBuffer(b.view).view
}

// InspectorChannel receives Chrome DevTools Protocol traffic synchronously.
type InspectorChannel interface {
	SendResponse(callID int32, message *InspectorStringBuffer)
	SendNotification(message *InspectorStringBuffer)
	FlushProtocolNotifications()
}

type InspectorClientTrustLevel int32

const (
	InspectorUntrusted InspectorClientTrustLevel = iota
	InspectorFullyTrusted
)

type Inspector struct {
	iso      *Isolate
	handle   uintptr
	closed   bool
	sessions int
	contexts map[*Context]struct{}
}
type InspectorSession struct {
	inspector *Inspector
	handle    uintptr
	channelID uint64
	closed    bool
	active    int
}

// inspectorLifecycleRegistry is keyed by the Go isolate wrapper identity, not
// its native address. Native addresses may be reused after disposal; wrapper
// identity keeps a later isolate independent from stale accounting belonging
// to an earlier isolate at the same address.
type inspectorIsolateLifecycle struct {
	inspectors int
	sessions   int
	contexts   map[*Context]int
}

var inspectorLifecycleRegistry = struct {
	sync.Mutex
	isolates map[*Isolate]*inspectorIsolateLifecycle
}{isolates: make(map[*Isolate]*inspectorIsolateLifecycle)}

func inspectorLifecycleForUpdate(iso *Isolate) *inspectorIsolateLifecycle {
	state := inspectorLifecycleRegistry.isolates[iso]
	if state == nil {
		state = &inspectorIsolateLifecycle{contexts: make(map[*Context]int)}
		inspectorLifecycleRegistry.isolates[iso] = state
	}
	return state
}

func pruneInspectorLifecycleLocked(iso *Isolate, state *inspectorIsolateLifecycle) {
	if state.inspectors == 0 && state.sessions == 0 && len(state.contexts) == 0 {
		delete(inspectorLifecycleRegistry.isolates, iso)
	}
}

func registerInspectorLifecycle(iso *Isolate) {
	inspectorLifecycleRegistry.Lock()
	inspectorLifecycleForUpdate(iso).inspectors++
	inspectorLifecycleRegistry.Unlock()
}

func unregisterInspectorLifecycle(iso *Isolate) {
	inspectorLifecycleRegistry.Lock()
	state := inspectorLifecycleRegistry.isolates[iso]
	if state != nil && state.inspectors > 0 {
		state.inspectors--
		pruneInspectorLifecycleLocked(iso, state)
	}
	inspectorLifecycleRegistry.Unlock()
}

func registerInspectorSessionLifecycle(iso *Isolate) {
	inspectorLifecycleRegistry.Lock()
	inspectorLifecycleForUpdate(iso).sessions++
	inspectorLifecycleRegistry.Unlock()
}

func unregisterInspectorSessionLifecycle(iso *Isolate) {
	inspectorLifecycleRegistry.Lock()
	state := inspectorLifecycleRegistry.isolates[iso]
	if state != nil && state.sessions > 0 {
		state.sessions--
		pruneInspectorLifecycleLocked(iso, state)
	}
	inspectorLifecycleRegistry.Unlock()
}

func registerInspectorContextLifecycle(iso *Isolate, context *Context) {
	inspectorLifecycleRegistry.Lock()
	inspectorLifecycleForUpdate(iso).contexts[context]++
	inspectorLifecycleRegistry.Unlock()
}

func unregisterInspectorContextLifecycle(iso *Isolate, context *Context) {
	inspectorLifecycleRegistry.Lock()
	state := inspectorLifecycleRegistry.isolates[iso]
	if state != nil {
		if state.contexts[context] <= 1 {
			delete(state.contexts, context)
		} else {
			state.contexts[context]--
		}
		pruneInspectorLifecycleLocked(iso, state)
	}
	inspectorLifecycleRegistry.Unlock()
}

func inspectorContextCloseError(context *Context) error {
	inspectorLifecycleRegistry.Lock()
	defer inspectorLifecycleRegistry.Unlock()
	state := inspectorLifecycleRegistry.isolates[context.iso]
	if state != nil && state.contexts[context] != 0 {
		return fmt.Errorf("gov8: context has %d active Inspector registration(s)", state.contexts[context])
	}
	return nil
}

func inspectorIsolateCloseError(iso *Isolate) error {
	inspectorLifecycleRegistry.Lock()
	defer inspectorLifecycleRegistry.Unlock()
	state := inspectorLifecycleRegistry.isolates[iso]
	if state == nil {
		return nil
	}
	contextRegistrations := 0
	for _, count := range state.contexts {
		contextRegistrations += count
	}
	return fmt.Errorf("gov8: isolate has live Inspector state (inspectors=%d, sessions=%d, context registrations=%d)",
		state.inspectors, state.sessions, contextRegistrations)
}

type inspectorChannelEntry struct {
	iso     *Isolate
	session *InspectorSession
	channel InspectorChannel
	active  int
}

var inspectorChannels = struct {
	sync.Mutex
	next    uint64
	entries map[uint64]*inspectorChannelEntry
}{entries: make(map[uint64]*inspectorChannelEntry)}
var inspectorDispatchOnce sync.Once
var inspectorDispatchErr error

var inspectorChannelDispatch = syscall.NewCallback(func(id, kindWord, callIDWord, is8Word, dataWord, lengthWord uintptr) uintptr {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in inspector channel callback: %v\n", recovered)
			proc("gov8_host_panic_abort").Call()
			panic(recovered)
		}
	}()
	inspectorChannels.Lock()
	entry := inspectorChannels.entries[uint64(id)]
	if entry != nil {
		entry.active++
		if entry.session != nil {
			entry.session.active++
		}
	}
	inspectorChannels.Unlock()
	if entry == nil || entry.channel == nil {
		fatalHostMisuse("unknown inspector channel %d", id)
		return 0
	}
	defer func() {
		inspectorChannels.Lock()
		entry.active--
		if entry.session != nil {
			entry.session.active--
		}
		inspectorChannels.Unlock()
	}()
	if lengthWord > uintptr(^uint(0)>>1) {
		fatalHostMisuse("inspector channel message length %d exceeds max int", lengthWord)
		return 0
	}
	if dataWord == 0 && lengthWord != 0 {
		fatalHostMisuse("inspector channel returned a null non-empty message")
		return 0
	}
	if is8Word != 0 && is8Word != 1 {
		fatalHostMisuse("inspector channel returned invalid string encoding %d", is8Word)
		return 0
	}
	length := int(lengthWord)
	var view InspectorStringView
	if is8Word != 0 {
		data := make([]byte, length)
		if length > 0 {
			copy(data, unsafe.Slice((*byte)(abiWordToPtr(dataWord)), length))
		}
		view = NewInspectorStringView8(data)
	} else {
		data := make([]uint16, length)
		if length > 0 {
			copy(data, unsafe.Slice((*uint16)(abiWordToPtr(dataWord)), length))
		}
		view = NewInspectorStringView16(data)
	}
	buffer := NewInspectorStringBuffer(view)
	switch int32(kindWord) {
	case 0:
		entry.channel.SendResponse(int32(callIDWord), buffer)
	case 1:
		entry.channel.SendNotification(buffer)
	case 2:
		entry.channel.FlushProtocolNotifications()
	default:
		fatalHostMisuse("invalid inspector channel callback kind %d", kindWord)
	}
	return 1
})

func ensureInspectorDispatch() error {
	inspectorDispatchOnce.Do(func() {
		inspectorDispatchErr = callErr("Inspector.ChannelDispatch", proc("gov8_inspector_set_channel_dispatch"), inspectorChannelDispatch)
	})
	return inspectorDispatchErr
}

func NewInspector(i *Isolate) (*Inspector, error) {
	return newInspector(i, 0)
}

// newInspector installs clientID in the native client base before V8Inspector
// construction. V8 invokes GenerateUniqueID during construction, so binding it
// afterward is observably too late.
func newInspector(i *Isolate, clientID uint64) (*Inspector, error) {
	if i == nil {
		return nil, errors.New("gov8: nil isolate")
	}
	if err := i.check(); err != nil {
		return nil, err
	}
	if err := ensureInspectorDispatch(); err != nil {
		return nil, err
	}
	var out uintptr
	r, _, _ := proc("gov8_inspector_create").Call(i.handleAssumingCheck(), uintptr(clientID), uintptr(unsafe.Pointer(&out)))
	if int64(r) < 0 {
		return nil, shimError("Inspector.New", r)
	}
	inspector := &Inspector{iso: i, handle: out, contexts: make(map[*Context]struct{})}
	registerInspectorLifecycle(i)
	return inspector, nil
}
func (i *Inspector) check() error {
	if i == nil {
		return errors.New("gov8: nil inspector")
	}
	if err := i.iso.check(); err != nil {
		return err
	}
	if i.closed {
		return errors.New("gov8: inspector used after Close")
	}
	return nil
}

func (i *Inspector) ContextCreated(c *Context, group int32, name, aux InspectorStringView) error {
	if err := i.check(); err != nil {
		return err
	}
	if c == nil || c.iso != i.iso {
		return foreignIsolate("context")
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return err
	}
	if group <= 0 {
		return errors.New("gov8: inspector context group must be positive")
	}
	if _, exists := i.contexts[c]; exists {
		return errors.New("gov8: inspector context is already registered")
	}
	ni, np, nl := name.native()
	ai, ap, al := aux.native()
	r, _, _ := proc("gov8_inspector_context_created").Call(i.handle, c.handle, uintptr(group), ni, np, nl, ai, ap, al)
	runtime.KeepAlive(name)
	runtime.KeepAlive(aux)
	if int64(r) < 0 {
		return shimError("Inspector.ContextCreated", r)
	}
	i.contexts[c] = struct{}{}
	registerInspectorContextLifecycle(i.iso, c)
	return nil
}
func (i *Inspector) ContextDestroyed(c *Context) error {
	if err := i.check(); err != nil {
		return err
	}
	if c == nil || c.iso != i.iso {
		return foreignIsolate("context")
	}
	if _, ok := i.contexts[c]; !ok {
		return errors.New("gov8: inspector context was not registered")
	}
	if inspectorClientCallbackActive(i) {
		return errors.New("gov8: Inspector client callback is active")
	}
	if inspectorInspectableContextActive(i, c) {
		return errors.New("gov8: Inspector Inspectable callback is active in this context")
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return err
	}
	r, _, _ := proc("gov8_inspector_context_destroyed").Call(i.handle, c.handle)
	if int64(r) < 0 {
		return shimError("Inspector.ContextDestroyed", r)
	}
	delete(i.contexts, c)
	unregisterInspectorContextLifecycle(i.iso, c)
	return nil
}

func (i *Inspector) Connect(group int32, channel InspectorChannel, state InspectorStringView, trust InspectorClientTrustLevel) (*InspectorSession, error) {
	if err := i.check(); err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, errors.New("gov8: nil inspector channel")
	}
	if group <= 0 {
		return nil, errors.New("gov8: inspector context group must be positive")
	}
	if trust < InspectorUntrusted || trust > InspectorFullyTrusted {
		return nil, fmt.Errorf("gov8: invalid inspector trust level %d", trust)
	}
	inspectorChannels.Lock()
	if inspectorChannels.next == math.MaxUint64 {
		inspectorChannels.Unlock()
		return nil, errors.New("gov8: inspector channel registry exhausted")
	}
	inspectorChannels.next++
	id := inspectorChannels.next
	entry := &inspectorChannelEntry{iso: i.iso, channel: channel}
	inspectorChannels.entries[id] = entry
	inspectorChannels.Unlock()
	is8, p, l := state.native()
	var out uintptr
	r, _, _ := proc("gov8_inspector_connect").Call(i.handle, uintptr(group), uintptr(id), is8, p, l, uintptr(trust), uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(state)
	if int64(r) < 0 {
		inspectorChannels.Lock()
		delete(inspectorChannels.entries, id)
		inspectorChannels.Unlock()
		return nil, shimError("Inspector.Connect", r)
	}
	session := &InspectorSession{inspector: i, handle: out, channelID: id}
	inspectorChannels.Lock()
	entry.session = session
	inspectorChannels.Unlock()
	i.sessions++
	registerInspectorSessionLifecycle(i.iso)
	return session, nil
}
func (s *InspectorSession) check() error {
	if s == nil || s.inspector == nil {
		return errors.New("gov8: nil inspector session")
	}
	if err := s.inspector.check(); err != nil {
		return err
	}
	if s.closed {
		return errors.New("gov8: inspector session used after Close")
	}
	return nil
}
func (s *InspectorSession) DispatchProtocolMessage(message InspectorStringView) error {
	if err := s.check(); err != nil {
		return err
	}
	if inspectorInspectableSessionActive(s) {
		return errors.New("gov8: cannot dispatch an Inspector protocol message during an Inspectable callback")
	}
	is8, p, l := message.native()
	r, _, _ := proc("gov8_inspector_session_dispatch").Call(s.handle, is8, p, l)
	runtime.KeepAlive(message)
	if int64(r) < 0 {
		return shimError("InspectorSession.DispatchProtocolMessage", r)
	}
	return nil
}
func (s *InspectorSession) Close() error {
	if err := s.check(); err != nil {
		return err
	}
	if inspectorInspectableSessionActive(s) {
		return errors.New("gov8: Inspector Inspectable callback is active")
	}
	if err := inspectorClientCloseError(s.inspector); err != nil {
		return err
	}
	inspectorChannels.Lock()
	entry := inspectorChannels.entries[s.channelID]
	if entry != nil && entry.active != 0 {
		inspectorChannels.Unlock()
		return errors.New("gov8: inspector channel callback is active")
	}
	inspectorChannels.Unlock()
	r, _, _ := proc("gov8_inspector_session_dispose").Call(s.handle)
	if int64(r) < 0 {
		return shimError("InspectorSession.Close", r)
	}
	inspectorChannels.Lock()
	delete(inspectorChannels.entries, s.channelID)
	inspectorChannels.Unlock()
	s.inspector.sessions--
	unregisterInspectorSessionLifecycle(s.inspector.iso)
	s.closed = true
	s.handle = 0
	return nil
}
func (i *Inspector) Close() error {
	if err := i.check(); err != nil {
		return err
	}
	if i.sessions != 0 {
		return errors.New("gov8: inspector has live sessions")
	}
	if len(i.contexts) != 0 {
		return errors.New("gov8: inspector has registered contexts")
	}
	if err := inspectorClientCloseError(i); err != nil {
		return err
	}
	r, _, _ := proc("gov8_inspector_dispose").Call(i.handle)
	if int64(r) < 0 {
		return shimError("Inspector.Close", r)
	}
	i.closed = true
	i.handle = 0
	dropInspectorClient(i)
	unregisterInspectorLifecycle(i.iso)
	return nil
}
