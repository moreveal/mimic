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

// ModuleSourceResolveRequest is active only while Instantiate2 is linking.
// ReturnValue must be an Object in the callback scope's isolate.
type ModuleSourceResolveRequest struct {
	Scope        *CallbackScope
	Specifier    string
	Referrer     *Module
	Phase        ModuleImportPhase
	SourceOffset int32
	Location     ModuleLocation
	Attributes   []ModuleImportAttribute
}

// ModuleSourceResolver resolves an import-source request to V8's source
// representation object. It runs synchronously during Instantiate2.
type ModuleSourceResolver func(ModuleSourceResolveRequest) (ReturnValue Value, Err error)

// StalledTopLevelAwait pairs the unresolved module with V8's diagnostic.
// Message is local to the Scope passed to StalledTopLevelAwaitMessages.
type StalledTopLevelAwait struct {
	Module  *Module
	Message *Message
}

// ImportMetaCallback initializes the stable import.meta object for a module.
type ImportMetaCallback func(scope *CallbackScope, module *Module, meta *Object) error

// DynamicImportRequest contains callback-local host import arguments.
type DynamicImportRequest struct {
	Scope              *CallbackScope
	HostDefinedOptions Data
	ResourceName       Value
	Specifier          Value
	Phase              ModuleImportPhase
	Attributes         *FixedArray
}

// DynamicImportCallback returns the promise forwarded to JavaScript.
type DynamicImportCallback func(DynamicImportRequest) (Promise, error)

// ShadowRealmContextCallback supplies the context for a new ShadowRealm. The
// returned persistent Context remains owned by the caller and must be closed.
type ShadowRealmContextCallback func(*CallbackScope) (*Context, error)

type moduleAdvancedEntry struct {
	iso     *Isolate
	source  ModuleSourceResolver
	meta    ImportMetaCallback
	dynamic DynamicImportCallback
	shadow  ShadowRealmContextCallback
	active  int
}

var moduleAdvancedRegistry = struct {
	sync.Mutex
	next    uint64
	entries map[uint64]*moduleAdvancedEntry
	byIso   map[*Isolate][4]uint64
}{entries: make(map[uint64]*moduleAdvancedEntry), byIso: make(map[*Isolate][4]uint64)}

var moduleAdvancedDispatchOnce sync.Once
var moduleAdvancedDispatchErr error

func moduleAdvancedPanic(kind string) {
	if recovered := recover(); recovered != nil {
		fmt.Fprintf(os.Stderr, "gov8: panic in %s callback: %v\n", kind, recovered)
		proc("gov8_host_panic_abort").Call()
		panic(recovered)
	}
}

func moduleForLocal(iso *Isolate, wire uintptr) *Module {
	moduleRegMu.Lock()
	defer moduleRegMu.Unlock()
	for _, module := range moduleByHandle {
		if module != nil && module.iso == iso && !module.closed {
			r, _, _ := proc("gov8_mar_module_matches").Call(module.handle, wire)
			if r == 1 {
				return module
			}
		}
	}
	return nil
}

var moduleSourceDispatcher = syscall.NewCallback(func(id, contextWire, scopeWire, specifierWire, attributesWire, referrerHandle, outWire uintptr) (handled uintptr) {
	defer moduleAdvancedPanic("module source resolver")
	moduleAdvancedRegistry.Lock()
	entry := moduleAdvancedRegistry.entries[uint64(id)]
	if entry != nil {
		entry.active++
	}
	moduleAdvancedRegistry.Unlock()
	if entry == nil || entry.source == nil {
		fatalHostMisuse("unknown module source resolver %d", id)
		return 0
	}
	defer func() { moduleAdvancedRegistry.Lock(); entry.active--; moduleAdvancedRegistry.Unlock() }()
	borrowed := &Scope{iso: entry.iso, handle: scopeWire, borrowed: true}
	cs := &CallbackScope{iso: entry.iso, sc: borrowed, ctxWire: contextWire}
	defer func() { borrowed.closed = true }()
	specifier, err := cs.ToString(cs.wrap(specifierWire))
	if err != nil {
		moduleThrowResolverError(entry.iso, err.Error())
		return 0
	}
	attributes, err := moduleResolverAttributes(entry.iso, attributesWire)
	if err != nil {
		moduleThrowResolverError(entry.iso, err.Error())
		return 0
	}
	moduleRegMu.Lock()
	referrer := moduleByHandle[referrerHandle]
	moduleRegMu.Unlock()
	request := ModuleSourceResolveRequest{Scope: cs, Specifier: specifier, Referrer: referrer, Phase: ModuleImportSource, Attributes: attributes}
	if referrer != nil {
		if requests, requestErr := referrer.Requests(); requestErr == nil {
			for _, candidate := range requests {
				if candidate.Phase == ModuleImportSource && candidate.Specifier == specifier {
					request.SourceOffset = candidate.SourceOffset
					request.Attributes = candidate.Attributes
					request.Location, _ = referrer.SourceOffsetToLocation(candidate.SourceOffset)
					break
				}
			}
		}
	}
	value, err := entry.source(request)
	if err != nil {
		moduleThrowResolverError(entry.iso, err.Error())
		return 0
	}
	if value.h == 0 {
		return 0
	}
	if err := value.check(); err != nil {
		moduleThrowResolverError(entry.iso, err.Error())
		return 0
	}
	if value.iso != entry.iso {
		moduleThrowResolverError(entry.iso, foreignIsolate("source resolver result").Error())
		return 0
	}
	isObject, err := value.IsObject()
	if err != nil || !isObject {
		moduleThrowResolverError(entry.iso, "source resolver result is not an Object")
		return 0
	}
	*(*uintptr)(abiWordToPtr(outWire)) = value.h
	return 1
})

var importMetaDispatcher = syscall.NewCallback(func(id, contextWire, scopeWire, moduleWire, metaWire, isolateWire uintptr) uintptr {
	defer moduleAdvancedPanic("import-meta")
	moduleAdvancedRegistry.Lock()
	entry := moduleAdvancedRegistry.entries[uint64(id)]
	if entry != nil {
		entry.active++
	}
	moduleAdvancedRegistry.Unlock()
	if entry == nil || entry.meta == nil || entry.iso.handle != isolateWire {
		fatalHostMisuse("unknown import-meta callback %d", id)
		return 0
	}
	defer func() { moduleAdvancedRegistry.Lock(); entry.active--; moduleAdvancedRegistry.Unlock() }()
	borrowed := &Scope{iso: entry.iso, handle: scopeWire, borrowed: true}
	cs := &CallbackScope{iso: entry.iso, sc: borrowed, ctxWire: contextWire}
	defer func() { borrowed.closed = true }()
	module := moduleForLocal(entry.iso, moduleWire)
	if module == nil {
		fatalHostMisuse("import-meta module is not registered")
		return 0
	}
	if err := entry.meta(cs, module, &Object{Value: cs.wrap(metaWire)}); err != nil {
		exception, makeErr := cs.NewError(err.Error())
		if makeErr != nil {
			fatalHostMisuse("import-meta error construction: %v", makeErr)
			return 0
		}
		_ = cs.ThrowException(exception)
	}
	return 1
})

var dynamicImportDispatcher = syscall.NewCallback(func(id, contextWire, scopeWire, hostWire, resourceWire, specifierWire, attributesWire, phaseWord, outWire uintptr) (handled uintptr) {
	defer moduleAdvancedPanic("dynamic import")
	moduleAdvancedRegistry.Lock()
	entry := moduleAdvancedRegistry.entries[uint64(id)]
	if entry != nil {
		entry.active++
	}
	moduleAdvancedRegistry.Unlock()
	if entry == nil || entry.dynamic == nil {
		fatalHostMisuse("unknown dynamic-import callback %d", id)
		return 0
	}
	defer func() { moduleAdvancedRegistry.Lock(); entry.active--; moduleAdvancedRegistry.Unlock() }()
	phase := ModuleImportPhase(int32(phaseWord))
	if phase < ModuleImportSource || phase > ModuleImportEvaluation {
		fatalHostMisuse("invalid dynamic-import phase %d", phase)
		return 0
	}
	borrowed := &Scope{iso: entry.iso, handle: scopeWire, borrowed: true}
	cs := &CallbackScope{iso: entry.iso, sc: borrowed, ctxWire: contextWire}
	defer func() { borrowed.closed = true }()
	promise, err := entry.dynamic(DynamicImportRequest{
		Scope: cs, HostDefinedOptions: Data{iso: entry.iso, sc: borrowed, h: hostWire},
		ResourceName: cs.wrap(resourceWire), Specifier: cs.wrap(specifierWire), Phase: phase,
		Attributes: &FixedArray{Data: Data{iso: entry.iso, sc: borrowed, h: attributesWire}},
	})
	if err != nil {
		exception, e := cs.NewError(err.Error())
		if e == nil {
			_ = cs.ThrowException(exception)
		}
		return 0
	}
	if promise.h == 0 {
		return 0
	}
	if err := promise.check(); err != nil || promise.iso != entry.iso {
		fatalHostMisuse("invalid dynamic-import promise: %v", err)
		return 0
	}
	*(*uintptr)(abiWordToPtr(outWire)) = promise.h
	return 1
})

var shadowRealmDispatcher = syscall.NewCallback(func(id, contextWire, scopeWire, outWire uintptr) (handled uintptr) {
	defer moduleAdvancedPanic("ShadowRealm context")
	moduleAdvancedRegistry.Lock()
	entry := moduleAdvancedRegistry.entries[uint64(id)]
	if entry != nil {
		entry.active++
	}
	moduleAdvancedRegistry.Unlock()
	if entry == nil || entry.shadow == nil {
		fatalHostMisuse("unknown ShadowRealm callback %d", id)
		return 0
	}
	defer func() { moduleAdvancedRegistry.Lock(); entry.active--; moduleAdvancedRegistry.Unlock() }()
	borrowed := &Scope{iso: entry.iso, handle: scopeWire, borrowed: true}
	cs := &CallbackScope{iso: entry.iso, sc: borrowed, ctxWire: contextWire}
	defer func() { borrowed.closed = true }()
	ctx, err := entry.shadow(cs)
	if err != nil {
		exception, e := cs.NewError(err.Error())
		if e == nil {
			_ = cs.ThrowException(exception)
		}
		return 0
	}
	if ctx == nil {
		return 0
	}
	if err := ctx.check(); err != nil || ctx.iso != entry.iso {
		fatalHostMisuse("invalid ShadowRealm context: %v", err)
		return 0
	}
	*(*uintptr)(abiWordToPtr(outWire)) = ctx.handle
	return 1
})

func ensureModuleAdvancedDispatchers() error {
	moduleAdvancedDispatchOnce.Do(func() {
		moduleAdvancedDispatchErr = callErr("ModuleAdvanced.Dispatchers", proc("gov8_mar_set_dispatchers"),
			moduleSourceDispatcher, importMetaDispatcher, dynamicImportDispatcher, shadowRealmDispatcher)
	})
	return moduleAdvancedDispatchErr
}

func registerModuleSourceResolver(i *Isolate, source ModuleSourceResolver) (uint64, error) {
	moduleAdvancedRegistry.Lock()
	defer moduleAdvancedRegistry.Unlock()
	if moduleAdvancedRegistry.next == math.MaxUint64 {
		return 0, errors.New("gov8: module callback registry exhausted")
	}
	moduleAdvancedRegistry.next++
	id := moduleAdvancedRegistry.next
	moduleAdvancedRegistry.entries[id] = &moduleAdvancedEntry{iso: i, source: source}
	return id, nil
}

func registerModuleAdvanced(i *Isolate, kind int, callback any) error {
	if err := i.check(); err != nil {
		return err
	}
	if err := ensureModuleAdvancedDispatchers(); err != nil {
		return err
	}
	moduleAdvancedRegistry.Lock()
	ids := moduleAdvancedRegistry.byIso[i]
	old := ids[kind]
	if old != 0 {
		if e := moduleAdvancedRegistry.entries[old]; e != nil && e.active != 0 {
			moduleAdvancedRegistry.Unlock()
			return errors.New("gov8: module host callback is active")
		}
		delete(moduleAdvancedRegistry.entries, old)
	}
	var id uint64
	if callback != nil {
		if moduleAdvancedRegistry.next == math.MaxUint64 {
			moduleAdvancedRegistry.Unlock()
			return errors.New("gov8: module callback registry exhausted")
		}
		moduleAdvancedRegistry.next++
		id = moduleAdvancedRegistry.next
		e := &moduleAdvancedEntry{iso: i}
		switch kind {
		case 0:
			e.meta = callback.(ImportMetaCallback)
		case 1, 2:
			e.dynamic = callback.(DynamicImportCallback)
		case 3:
			e.shadow = callback.(ShadowRealmContextCallback)
		}
		moduleAdvancedRegistry.entries[id] = e
	}
	ids[kind] = id
	moduleAdvancedRegistry.byIso[i] = ids
	moduleAdvancedRegistry.Unlock()
	return callErr("ModuleAdvanced.SetHostCallback", proc("gov8_mar_set_host_callback"), i.handleAssumingCheck(), uintptr(kind), uintptr(id))
}

// SetHostInitializeImportMetaObjectCallback replaces or clears the isolate's
// import-meta initializer. A nil callback clears the Go registration.
func (i *Isolate) SetHostInitializeImportMetaObjectCallback(cb ImportMetaCallback) error {
	if cb == nil {
		return registerModuleAdvanced(i, 0, nil)
	}
	return registerModuleAdvanced(i, 0, cb)
}

// SetHostImportModuleDynamicallyCallback replaces or clears the legacy
// evaluation-phase dynamic import callback.
func (i *Isolate) SetHostImportModuleDynamicallyCallback(cb DynamicImportCallback) error {
	if cb == nil {
		return registerModuleAdvanced(i, 1, nil)
	}
	return registerModuleAdvanced(i, 1, cb)
}

// SetHostImportModuleWithPhaseDynamicallyCallback replaces or clears the
// experimental phase-aware dynamic import callback.
func (i *Isolate) SetHostImportModuleWithPhaseDynamicallyCallback(cb DynamicImportCallback) error {
	if cb == nil {
		return registerModuleAdvanced(i, 2, nil)
	}
	return registerModuleAdvanced(i, 2, cb)
}

// SetHostCreateShadowRealmContextCallback replaces or clears the isolate's
// ShadowRealm context factory.
func (i *Isolate) SetHostCreateShadowRealmContextCallback(cb ShadowRealmContextCallback) error {
	if cb == nil {
		return registerModuleAdvanced(i, 3, nil)
	}
	return registerModuleAdvanced(i, 3, cb)
}

func releaseModuleAdvancedHostState(i *Isolate) error {
	moduleAdvancedRegistry.Lock()
	ids := moduleAdvancedRegistry.byIso[i]
	for _, id := range ids {
		if e := moduleAdvancedRegistry.entries[id]; e != nil && e.active != 0 {
			moduleAdvancedRegistry.Unlock()
			return errors.New("gov8: module host callback is active")
		}
	}
	for _, id := range ids {
		delete(moduleAdvancedRegistry.entries, id)
	}
	delete(moduleAdvancedRegistry.byIso, i)
	moduleAdvancedRegistry.Unlock()
	return callErr("ModuleAdvanced.Release", proc("gov8_mar_release"), i.handleAssumingCheck())
}

// NewCallbackPromiseResolver creates a resolver in the callback's current context.
func (cs *CallbackScope) NewCallbackPromiseResolver() (PromiseResolver, Promise, error) {
	if err := cs.check(); err != nil {
		return PromiseResolver{}, Promise{}, err
	}
	var resolver, promise uintptr
	r, _, _ := proc("gov8_mar_callback_promise_new").Call(cs.iso.handleAssumingCheck(), cs.ctxWire, cs.sc.handle, uintptr(unsafe.Pointer(&resolver)), uintptr(unsafe.Pointer(&promise)))
	if int64(r) < 0 {
		return PromiseResolver{}, Promise{}, shimError("CallbackPromiseResolver.New", r)
	}
	return PromiseResolver{Value{iso: cs.iso, sc: cs.sc, h: resolver}}, Promise{Value{iso: cs.iso, sc: cs.sc, h: promise}}, nil
}

// NewTypeError creates a callback-local TypeError in the callback's current context.
func (cs *CallbackScope) NewTypeError(message string) (Value, error) {
	if err := cs.check(); err != nil {
		return Value{}, err
	}
	if uint64(len(message)) > uint64(^uint32(0)>>1) {
		return Value{}, errors.New("gov8: exception message exceeds V8 string length")
	}
	bytes := []byte(message)
	var out uintptr
	r, _, _ := proc("gov8_mar_callback_type_error").Call(cs.iso.handleAssumingCheck(), cs.ctxWire, cs.sc.handle, bytesPtr(bytes), uintptr(len(bytes)), uintptr(unsafe.Pointer(&out)))
	if int64(r) < 0 {
		return Value{}, shimError("CallbackScope.NewTypeError", r)
	}
	return cs.wrap(out), nil
}

// SettleCallbackPromise resolves or rejects a callback-local resolver.
func (cs *CallbackScope) SettleCallbackPromise(resolver PromiseResolver, value Value, reject bool) (bool, error) {
	if err := cs.check(); err != nil {
		return false, err
	}
	if err := resolver.check(); err != nil {
		return false, err
	}
	if err := value.check(); err != nil {
		return false, err
	}
	if resolver.iso != cs.iso || value.iso != cs.iso {
		return false, foreignIsolate("promise value")
	}
	var rejectWord uintptr
	if reject {
		rejectWord = 1
	}
	var ok int32
	r, _, _ := proc("gov8_mar_callback_promise_settle").Call(cs.iso.handleAssumingCheck(), cs.ctxWire, resolver.h, value.h, rejectWord, uintptr(unsafe.Pointer(&ok)))
	if int64(r) < 0 {
		return false, shimError("CallbackPromiseResolver.Settle", r)
	}
	return ok == 1, nil
}

// Instantiate2 links evaluation-phase and source-phase requests with separate resolvers.
func (m *Module) Instantiate2(s *Scope, resolver ModuleResolver, source ModuleSourceResolver, tc *TryCatch) (bool, error) {
	if source == nil {
		return false, errors.New("gov8: nil module source resolver")
	}
	if resolver == nil {
		return false, errors.New("gov8: nil module resolver")
	}
	if err := m.check(); err != nil {
		return false, err
	}
	if s == nil || s.iso != m.iso {
		return false, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return false, err
	}
	if status, err := m.Status(); err != nil || status != ModuleUninstantiated {
		if err != nil {
			return false, err
		}
		return false, fmt.Errorf("gov8: Instantiate2 requires Uninstantiated module, got %s", status)
	}
	var tcHandle uintptr
	if tc != nil {
		if tc.iso != m.iso {
			return false, foreignIsolate("trycatch")
		}
		if err := tc.check(); err != nil {
			return false, err
		}
		tcHandle = tc.handle
	}
	installModuleResolveEntry()
	if err := ensureModuleAdvancedDispatchers(); err != nil {
		return false, err
	}
	sourceID, err := registerModuleSourceResolver(m.iso, source)
	if err != nil {
		return false, err
	}
	defer func() {
		moduleAdvancedRegistry.Lock()
		delete(moduleAdvancedRegistry.entries, sourceID)
		moduleAdvancedRegistry.Unlock()
	}()
	moduleRegMu.Lock()
	moduleResolveID++
	resolveID := moduleResolveID
	moduleResolveReg[resolveID] = &moduleResolveEntry{module: m, scope: s, fn: resolver}
	moduleRegMu.Unlock()
	defer func() {
		moduleRegMu.Lock()
		delete(moduleResolveReg, resolveID)
		moduleRegMu.Unlock()
	}()
	var ok int32
	r, _, _ := proc("gov8_mar_instantiate2").Call(m.iso.handleAssumingCheck(), m.ctx.handle, sh, m.handle, tcHandle, uintptr(resolveID), uintptr(sourceID), 0, 0, uintptr(unsafe.Pointer(&ok)))
	if int64(r) < 0 {
		return false, shimError("Module.Instantiate2", r)
	}
	return ok == 1, nil
}

// NamespaceWithPhase returns the namespace representation for phase.
func (m *Module) NamespaceWithPhase(s *Scope, phase ModuleImportPhase) (Value, error) {
	if phase < ModuleImportSource || phase > ModuleImportEvaluation {
		return Value{}, fmt.Errorf("gov8: invalid module import phase %d", phase)
	}
	if err := m.check(); err != nil {
		return Value{}, err
	}
	if s == nil || s.iso != m.iso {
		return Value{}, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, err
	}
	status, err := m.Status()
	if err != nil {
		return Value{}, err
	}
	if status < ModuleInstantiated {
		return Value{}, fmt.Errorf("gov8: NamespaceWithPhase requires an instantiated module, got %s", status)
	}
	var out uintptr
	r, _, _ := proc("gov8_mar_namespace_phase").Call(m.iso.handleAssumingCheck(), sh, m.handle, uintptr(phase), uintptr(unsafe.Pointer(&out)))
	if int64(r) < 0 {
		return Value{}, shimError("Module.NamespaceWithPhase", r)
	}
	return Value{iso: m.iso, sc: s, h: out}, nil
}

// EvaluateForImportDefer gathers asynchronous dependencies without evaluating
// the module itself. ok is false when V8 returned an empty MaybeLocal.
func (m *Module) EvaluateForImportDefer(s *Scope) (Value, bool, error) {
	if err := m.check(); err != nil {
		return Value{}, false, err
	}
	if s == nil || s.iso != m.iso {
		return Value{}, false, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, false, err
	}
	status, err := m.Status()
	if err != nil {
		return Value{}, false, err
	}
	if status != ModuleInstantiated {
		return Value{}, false, fmt.Errorf("gov8: EvaluateForImportDefer requires Instantiated module, got %s", status)
	}
	var out uintptr
	r, _, _ := proc("gov8_mar_evaluate_defer").Call(m.iso.handleAssumingCheck(), m.ctx.handle, sh, m.handle, uintptr(unsafe.Pointer(&out)))
	if int64(r) == errException {
		return Value{}, false, nil
	}
	if int64(r) < 0 {
		return Value{}, false, shimError("Module.EvaluateForImportDefer", r)
	}
	return Value{iso: m.iso, sc: s, h: out}, out != 0, nil
}

// StalledTopLevelAwaitMessages returns the pinned crate's diagnostic tuples.
// Like rusty_v8 152.2.0, the underlying query is capped at 16 entries.
func (m *Module) StalledTopLevelAwaitMessages(s *Scope) ([]StalledTopLevelAwait, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	if s == nil || s.iso != m.iso {
		return nil, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	var count int64
	r, _, _ := proc("gov8_mar_stalled").Call(m.iso.handleAssumingCheck(), sh, m.handle, 0, 0, 0, uintptr(unsafe.Pointer(&count)))
	if int64(r) != errNoMemory && int64(r) < 0 {
		return nil, shimError("Module.StalledTopLevelAwaitMessages", r)
	}
	if count == 0 {
		return nil, nil
	}
	modules := make([]uintptr, count)
	messages := make([]uintptr, count)
	r, _, _ = proc("gov8_mar_stalled").Call(m.iso.handleAssumingCheck(), sh, m.handle, uintptr(unsafe.Pointer(&modules[0])), uintptr(unsafe.Pointer(&messages[0])), uintptr(count), uintptr(unsafe.Pointer(&count)))
	if int64(r) < 0 {
		return nil, shimError("Module.StalledTopLevelAwaitMessages", r)
	}
	result := make([]StalledTopLevelAwait, count)
	for index := range result {
		resolved := moduleForLocal(m.iso, modules[index])
		if resolved == nil {
			return nil, errors.New("gov8: stalled module is not registered")
		}
		result[index] = StalledTopLevelAwait{Module: resolved, Message: &Message{iso: m.iso, sc: s, h: messages[index]}}
	}
	return result, nil
}
