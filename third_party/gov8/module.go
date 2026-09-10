//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// ModuleStatus is the ECMAScript module state, with evaluated failures split
// into ModuleErrored as in V8 and rusty_v8 152.2.0.
type ModuleStatus int32

const (
	ModuleUninstantiated ModuleStatus = iota
	ModuleInstantiating
	ModuleInstantiated
	ModuleEvaluating
	ModuleEvaluated
	ModuleErrored
)

func (s ModuleStatus) String() string {
	switch s {
	case ModuleUninstantiated:
		return "Uninstantiated"
	case ModuleInstantiating:
		return "Instantiating"
	case ModuleInstantiated:
		return "Instantiated"
	case ModuleEvaluating:
		return "Evaluating"
	case ModuleEvaluated:
		return "Evaluated"
	case ModuleErrored:
		return "Errored"
	default:
		return fmt.Sprintf("ModuleStatus(%d)", int32(s))
	}
}

// ModuleImportPhase mirrors v8::ModuleImportPhase.
type ModuleImportPhase int32

const (
	ModuleImportSource ModuleImportPhase = iota
	ModuleImportDefer
	ModuleImportEvaluation
)

// ModuleLocation is a zero-based source line and column.
type ModuleLocation struct {
	Line   int32
	Column int32
}

// ModuleImportAttribute is one import attribute. SourceOffset points to the
// attribute in the module source.
type ModuleImportAttribute struct {
	Key          string
	Value        string
	SourceOffset int32
}

// ModuleRequest describes one direct module dependency.
type ModuleRequest struct {
	Specifier    string
	Phase        ModuleImportPhase
	SourceOffset int32
	Attributes   []ModuleImportAttribute
}

// ModuleCompileOptions controls the ScriptOrigin attached to a module.
type ModuleCompileOptions struct {
	ResourceName string
	LineOffset   int32
	ColumnOffset int32
}

// Module is a persistent source-text or synthetic module rooted in its isolate.
// It remains usable across handle scopes, but is bound to its creation context.
// Close must be called before closing that context or isolate.
type Module struct {
	iso                 *Isolate
	ctx                 *Context
	handle              uintptr
	closed              bool
	syntheticCallbackID uint64
	syntheticActive     bool
}

var (
	moduleRegMu                     sync.Mutex
	moduleByHandle                  = map[uintptr]*Module{}
	moduleResolveReg                = map[int64]*moduleResolveEntry{}
	moduleResolveID                 int64
	moduleResolveOnce               sync.Once
	moduleHotProcsOnce              sync.Once
	moduleCompileAddr               uintptr
	moduleDisposeAddr               uintptr
	moduleStatusAddr                uintptr
	moduleInstantiateAddr           uintptr
	moduleEvaluateAddr              uintptr
	moduleResolveSpecifierAddr      uintptr
	moduleResolveAttributeCountAddr uintptr
	moduleResolveAttributeAddr      uintptr
)

func ensureModuleHotProcs() {
	moduleHotProcsOnce.Do(func() {
		moduleCompileAddr = proc("gov8_module_compile").Addr()
		moduleDisposeAddr = proc("gov8_module_dispose").Addr()
		moduleStatusAddr = proc("gov8_module_status").Addr()
		moduleInstantiateAddr = proc("gov8_module_instantiate").Addr()
		moduleEvaluateAddr = proc("gov8_module_evaluate").Addr()
		moduleResolveSpecifierAddr = proc("gov8_module_resolve_specifier").Addr()
		moduleResolveAttributeCountAddr = proc("gov8_module_resolve_attribute_count").Addr()
		moduleResolveAttributeAddr = proc("gov8_module_resolve_attribute").Addr()
	})
}

//go:uintptrescapes
func moduleEscapingSyscall9(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9 uintptr) (uintptr, uintptr, syscall.Errno) {
	return syscall.Syscall9(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9)
}

//go:uintptrescapes
func moduleEscapingSyscall6(trap, nargs, a1, a2, a3, a4, a5, a6 uintptr) (uintptr, uintptr, syscall.Errno) {
	return syscall.Syscall6(trap, nargs, a1, a2, a3, a4, a5, a6)
}

func (m *Module) check() error {
	if m == nil {
		return errors.New("gov8: nil module")
	}
	if err := m.iso.check(); err != nil {
		return err
	}
	return m.checkAssumingIsolate()
}

func (m *Module) checkAssumingIsolate() error {
	if m == nil {
		return errors.New("gov8: nil module")
	}
	if m.closed {
		return errors.New("gov8: module used after Close")
	}
	return m.ctx.checkAssumingIsolate()
}

func bytesPtr(b []byte) uintptr {
	if len(b) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&b[0]))
}

// CompileModule parses source as an ECMAScript SourceTextModule. ResourceName
// is exposed to V8 diagnostics and import-meta host hooks. Syntax failures are
// reported as exception errors and, when supplied, recorded in tc.
func (c *Context) CompileModule(s *Scope, source, resourceName string, tc *TryCatch) (*Module, error) {
	return c.CompileModuleWithOptions(s, source, ModuleCompileOptions{ResourceName: resourceName}, tc)
}

// CompileModuleWithOptions is CompileModule with explicit ScriptOrigin line
// and column offsets.
func (c *Context) CompileModuleWithOptions(s *Scope, source string, options ModuleCompileOptions, tc *TryCatch) (*Module, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	if s == nil || s.iso != c.iso {
		return nil, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	if tc != nil {
		if tc.iso != c.iso {
			return nil, foreignIsolate("trycatch")
		}
		if err := tc.check(); err != nil {
			return nil, err
		}
	}
	src := []byte(source)
	name := []byte(options.ResourceName)
	var tcHandle, out uintptr
	if tc != nil {
		tcHandle = tc.handle
	}
	ensureModuleHotProcs()
	r1, _, _ := syscall.Syscall12(moduleCompileAddr, 11,
		c.iso.handleAssumingCheck(), c.handle, sh, tcHandle,
		bytesPtr(src), uintptr(len(src)), bytesPtr(name), uintptr(len(name)),
		uintptr(options.LineOffset), uintptr(options.ColumnOffset),
		uintptr(unsafe.Pointer(&out)), 0)
	runtime.KeepAlive(src)
	runtime.KeepAlive(name)
	if int64(r1) < 0 {
		return nil, shimError("CompileModule", r1)
	}
	m := &Module{iso: c.iso, ctx: c, handle: out}
	moduleRegMu.Lock()
	moduleByHandle[out] = m
	moduleRegMu.Unlock()
	return m, nil
}

// CompileSourceTextModule is an explicit alias for CompileModule.
func (c *Context) CompileSourceTextModule(s *Scope, source, resourceName string, tc *TryCatch) (*Module, error) {
	return c.CompileModule(s, source, resourceName, tc)
}

// Close releases the persistent module handle.
func (m *Module) Close() error {
	if m == nil {
		return errors.New("gov8: nil module")
	}
	if err := m.iso.check(); err != nil {
		return err
	}
	if m.closed {
		return errors.New("gov8: module already closed")
	}
	if m.syntheticActive {
		return errors.New("gov8: cannot close a synthetic module from its active evaluation callback")
	}
	ensureModuleHotProcs()
	r1, _, _ := syscall.Syscall(moduleDisposeAddr, 1, m.handle, 0, 0)
	if int64(r1) < 0 {
		return shimError("Module.Close", r1)
	}
	var syntheticCleanupErr error
	if m.syntheticCallbackID != 0 {
		r2, _, _ := syscall.Syscall(syntheticUnregisterAddr, 2,
			m.iso.handleAssumingCheck(), uintptr(m.syntheticCallbackID), 0)
		if int64(r2) < 0 {
			syntheticCleanupErr = shimError("SyntheticModule.Close", r2)
		}
		dropSyntheticModuleCallback(m.syntheticCallbackID)
		m.syntheticCallbackID = 0
	}
	moduleRegMu.Lock()
	delete(moduleByHandle, m.handle)
	moduleRegMu.Unlock()
	m.closed = true
	m.handle = 0
	return syntheticCleanupErr
}

// Status returns the current module state.
func (m *Module) Status() (ModuleStatus, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	return m.statusAssumingChecked()
}

func (m *Module) statusAssumingChecked() (ModuleStatus, error) {
	ensureModuleHotProcs()
	r1, _, _ := syscall.Syscall(moduleStatusAddr, 1, m.handle, 0, 0)
	if int64(r1) < 0 {
		return 0, shimError("Module.Status", r1)
	}
	return ModuleStatus(r1), nil
}

// IdentityHash returns V8's non-zero identity hash. It is stable but not
// guaranteed unique.
func (m *Module) IdentityHash() (int32, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	var out int32
	r1, _, _ := proc("gov8_module_identity_hash").Call(m.handle, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("Module.IdentityHash", r1)
	}
	return out, nil
}

// ScriptID returns the underlying script id. It is unavailable after the
// module enters the errored state.
func (m *Module) ScriptID() (int32, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	var out int32
	r1, _, _ := proc("gov8_module_script_id").Call(m.handle, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("Module.ScriptID", r1)
	}
	return out, nil
}

func (m *Module) modulePredicate(procName, op string) (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	r1, _, _ := proc(procName).Call(m.handle)
	if int64(r1) < 0 {
		return false, shimError(op, r1)
	}
	return r1 == 1, nil
}

// IsSourceTextModule reports whether this is a source-text module.
func (m *Module) IsSourceTextModule() (bool, error) {
	return m.modulePredicate("gov8_module_is_source_text", "Module.IsSourceTextModule")
}

// IsSyntheticModule reports whether this is a synthetic module.
func (m *Module) IsSyntheticModule() (bool, error) {
	return m.modulePredicate("gov8_module_is_synthetic", "Module.IsSyntheticModule")
}

// HasTopLevelAwait reports whether this module itself contains top-level await.
func (m *Module) HasTopLevelAwait() (bool, error) {
	return m.modulePredicate("gov8_module_has_top_level_await", "Module.HasTopLevelAwait")
}

// IsGraphAsync reports whether the instantiated graph contains top-level await.
func (m *Module) IsGraphAsync() (bool, error) {
	status, err := m.Status()
	if err != nil {
		return false, err
	}
	if status < ModuleInstantiated {
		return false, fmt.Errorf("gov8: IsGraphAsync requires an instantiated module, got %s", status)
	}
	return m.modulePredicate("gov8_module_is_graph_async", "Module.IsGraphAsync")
}

func moduleRequestInfo(handle uintptr, index int32) (ModuleRequest, error) {
	var phase, offset, attributeCount int32
	var needed int64
	p := proc("gov8_module_request_info")
	r1, _, _ := p.Call(handle, uintptr(index), uintptr(unsafe.Pointer(&phase)),
		uintptr(unsafe.Pointer(&offset)), uintptr(unsafe.Pointer(&attributeCount)),
		0, 0, uintptr(unsafe.Pointer(&needed)))
	if int64(r1) != errNoMemory && int64(r1) < 0 {
		return ModuleRequest{}, shimError("Module.Requests", r1)
	}
	buf := make([]byte, needed)
	if len(buf) == 0 {
		buf = make([]byte, 1)
	}
	r1, _, _ = p.Call(handle, uintptr(index), uintptr(unsafe.Pointer(&phase)),
		uintptr(unsafe.Pointer(&offset)), uintptr(unsafe.Pointer(&attributeCount)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)))
	if int64(r1) < 0 {
		return ModuleRequest{}, shimError("Module.Requests", r1)
	}
	request := ModuleRequest{Specifier: string(buf[:needed]), Phase: ModuleImportPhase(phase), SourceOffset: offset}
	request.Attributes = make([]ModuleImportAttribute, attributeCount)
	for j := int32(0); j < attributeCount; j++ {
		var keyLen, valueLen int64
		var attrOffset int32
		ap := proc("gov8_module_request_attribute")
		r1, _, _ = ap.Call(handle, uintptr(index), uintptr(j), 0, 0,
			uintptr(unsafe.Pointer(&keyLen)), 0, 0, uintptr(unsafe.Pointer(&valueLen)),
			uintptr(unsafe.Pointer(&attrOffset)))
		if int64(r1) != errNoMemory && int64(r1) < 0 {
			return ModuleRequest{}, shimError("Module.Requests", r1)
		}
		key := make([]byte, keyLen+1)
		value := make([]byte, valueLen+1)
		r1, _, _ = ap.Call(handle, uintptr(index), uintptr(j), uintptr(unsafe.Pointer(&key[0])),
			uintptr(keyLen), uintptr(unsafe.Pointer(&keyLen)), uintptr(unsafe.Pointer(&value[0])),
			uintptr(valueLen), uintptr(unsafe.Pointer(&valueLen)), uintptr(unsafe.Pointer(&attrOffset)))
		if int64(r1) < 0 {
			return ModuleRequest{}, shimError("Module.Requests", r1)
		}
		request.Attributes[j] = ModuleImportAttribute{Key: string(key[:keyLen]), Value: string(value[:valueLen]), SourceOffset: attrOffset}
	}
	return request, nil
}

// Requests returns all direct dependencies in source order, including import
// phase, source offsets, and import attributes.
func (m *Module) Requests() ([]ModuleRequest, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	r1, _, _ := proc("gov8_module_request_count").Call(m.handle)
	if int64(r1) < 0 {
		return nil, shimError("Module.Requests", r1)
	}
	requests := make([]ModuleRequest, int(r1))
	for i := range requests {
		request, err := moduleRequestInfo(m.handle, int32(i))
		if err != nil {
			return nil, err
		}
		requests[i] = request
	}
	return requests, nil
}

// SourceOffsetToLocation converts a module source offset to a zero-based line
// and column.
func (m *Module) SourceOffsetToLocation(offset int32) (ModuleLocation, error) {
	if err := m.check(); err != nil {
		return ModuleLocation{}, err
	}
	var line, column int32
	r1, _, _ := proc("gov8_module_source_location").Call(m.handle, uintptr(offset),
		uintptr(unsafe.Pointer(&line)), uintptr(unsafe.Pointer(&column)))
	if int64(r1) < 0 {
		return ModuleLocation{}, shimError("Module.SourceOffsetToLocation", r1)
	}
	return ModuleLocation{Line: line, Column: column}, nil
}

// ModuleResolveRequest is delivered synchronously while Instantiate is
// linking. Its referrer and specifier are valid for the callback duration.
type ModuleResolveRequest struct {
	Specifier  string
	Referrer   *Module
	Attributes []ModuleImportAttribute
}

// ModuleResolver resolves a direct import to a compiled module. Returning nil
// or an error fails linking. The returned module must share isolate and context.
type ModuleResolver func(ModuleResolveRequest) (*Module, error)

type moduleResolveEntry struct {
	module *Module
	scope  *Scope
	fn     ModuleResolver
	err    error
}

func installModuleResolveEntry() {
	moduleResolveOnce.Do(func() {
		entry := syscall.NewCallback(moduleResolveDispatch)
		_, _, _ = proc("gov8_module_resolve_set_entry").Call(entry)
	})
}

func moduleResolveDispatch(id, specifierWire, attributesWire, referrerHandle, outHandle uintptr) (handled uintptr) {
	defer func() {
		if p := recover(); p != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in module resolver: %v\n", p)
			proc("gov8_host_panic_abort").Call()
			panic(p) // unreachable: fail-fast does not return
		}
	}()
	moduleRegMu.Lock()
	entry := moduleResolveReg[int64(id)]
	var referrer *Module
	if entry != nil && entry.module.handle == referrerHandle {
		referrer = entry.module
	} else {
		referrer = moduleByHandle[referrerHandle]
	}
	moduleRegMu.Unlock()
	if entry == nil || referrer == nil {
		return 0
	}
	specifier, err := moduleResolverSpecifier(entry.module.iso, specifierWire)
	if err != nil {
		entry.err = err
		return 0
	}
	attributes, err := moduleResolverAttributes(entry.module.iso, attributesWire)
	if err != nil {
		entry.err = err
		return 0
	}
	resolved, err := entry.fn(ModuleResolveRequest{
		Specifier: specifier, Referrer: referrer, Attributes: attributes,
	})
	if err != nil {
		entry.err = err
		moduleThrowResolverError(entry.module.iso, err.Error())
		return 0
	}
	if resolved == nil {
		entry.err = fmt.Errorf("gov8: resolver returned nil for %q", specifier)
		moduleThrowResolverError(entry.module.iso, entry.err.Error())
		return 0
	}
	if resolved.iso != entry.module.iso {
		if err := resolved.check(); err != nil {
			entry.err = err
			return 0
		}
		entry.err = foreignIsolate("resolved module")
		return 0
	}
	if err := resolved.checkAssumingIsolate(); err != nil {
		entry.err = err
		return 0
	}
	if resolved.ctx != entry.module.ctx {
		entry.err = errors.New("gov8: resolved module belongs to a different context")
		return 0
	}
	*(*uintptr)(abiWordToPtr(outHandle)) = resolved.handle
	return 1
}

func moduleResolverSpecifier(iso *Isolate, specifierWire uintptr) (string, error) {
	var local [256]byte
	var needed int64
	r1, _, _ := syscall.Syscall6(moduleResolveSpecifierAddr, 5,
		iso.handleAssumingCheck(), specifierWire, uintptr(unsafe.Pointer(&local[0])),
		uintptr(len(local)), uintptr(unsafe.Pointer(&needed)), 0)
	runtime.KeepAlive(&local)
	if int64(r1) == errNoMemory {
		if needed <= int64(len(local)) || needed > int64(^uint(0)>>1) {
			return "", errors.New("gov8: invalid module specifier length")
		}
		buf := make([]byte, int(needed))
		r1, _, _ = syscall.Syscall6(moduleResolveSpecifierAddr, 5,
			iso.handleAssumingCheck(), specifierWire, bytesPtr(buf), uintptr(len(buf)),
			uintptr(unsafe.Pointer(&needed)), 0)
		runtime.KeepAlive(buf)
		if int64(r1) < 0 {
			return "", shimError("Module.Instantiate", r1)
		}
		if needed < 0 || needed > int64(len(buf)) {
			return "", errors.New("gov8: invalid module specifier length")
		}
		return string(buf[:needed]), nil
	}
	if int64(r1) < 0 {
		return "", shimError("Module.Instantiate", r1)
	}
	if needed < 0 || needed > int64(len(local)) {
		return "", errors.New("gov8: invalid module specifier length")
	}
	return string(local[:needed]), nil
}

func moduleResolverAttributes(iso *Isolate, attributesWire uintptr) ([]ModuleImportAttribute, error) {
	ensureModuleHotProcs()
	r1, _, _ := syscall.Syscall(moduleResolveAttributeCountAddr, 2,
		iso.handleAssumingCheck(), attributesWire, 0)
	if int64(r1) < 0 {
		return nil, shimError("Module.Instantiate", r1)
	}
	attributes := make([]ModuleImportAttribute, int(r1))
	for i := range attributes {
		var keyLen, valueLen int64
		var sourceOffset int32
		r1, _, _ = syscall.Syscall12(moduleResolveAttributeAddr, 10,
			iso.handleAssumingCheck(), attributesWire, uintptr(i),
			0, 0, uintptr(unsafe.Pointer(&keyLen)), 0, 0,
			uintptr(unsafe.Pointer(&valueLen)), uintptr(unsafe.Pointer(&sourceOffset)), 0, 0)
		if int64(r1) != errNoMemory && int64(r1) < 0 {
			return nil, shimError("Module.Instantiate", r1)
		}
		key := make([]byte, keyLen+1)
		value := make([]byte, valueLen+1)
		r1, _, _ = syscall.Syscall12(moduleResolveAttributeAddr, 10,
			iso.handleAssumingCheck(), attributesWire, uintptr(i),
			uintptr(unsafe.Pointer(&key[0])), uintptr(keyLen), uintptr(unsafe.Pointer(&keyLen)),
			uintptr(unsafe.Pointer(&value[0])), uintptr(valueLen), uintptr(unsafe.Pointer(&valueLen)),
			uintptr(unsafe.Pointer(&sourceOffset)), 0, 0)
		runtime.KeepAlive(key)
		runtime.KeepAlive(value)
		if int64(r1) < 0 {
			return nil, shimError("Module.Instantiate", r1)
		}
		attributes[i] = ModuleImportAttribute{
			Key: string(key[:keyLen]), Value: string(value[:valueLen]), SourceOffset: sourceOffset,
		}
	}
	return attributes, nil
}

func moduleThrowResolverError(iso *Isolate, message string) {
	b := []byte(message)
	_, _, _ = proc("gov8_module_resolve_throw").Call(iso.handleAssumingCheck(), bytesPtr(b), uintptr(len(b)))
}

// Instantiate links the module graph through resolver.
func (m *Module) Instantiate(s *Scope, resolver ModuleResolver, tc *TryCatch) (bool, error) {
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
	status, err := m.statusAssumingChecked()
	if err != nil {
		return false, err
	}
	if status != ModuleUninstantiated {
		return false, fmt.Errorf("gov8: Instantiate requires Uninstantiated module, got %s", status)
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
	moduleRegMu.Lock()
	moduleResolveID++
	id := moduleResolveID
	entry := &moduleResolveEntry{module: m, scope: s, fn: resolver}
	moduleResolveReg[id] = entry
	moduleRegMu.Unlock()
	defer func() {
		moduleRegMu.Lock()
		delete(moduleResolveReg, id)
		moduleRegMu.Unlock()
	}()
	var ok int32
	ensureModuleHotProcs()
	r1, _, _ := moduleEscapingSyscall9(moduleInstantiateAddr, 9,
		m.iso.handleAssumingCheck(), m.ctx.handle, sh, m.handle, tcHandle,
		uintptr(id), 0, 0, uintptr(unsafe.Pointer(&ok)))
	if entry.err != nil {
		return false, entry.err
	}
	if int64(r1) < 0 {
		return false, shimError("Module.Instantiate", r1)
	}
	return ok == 1, nil
}

// Evaluate evaluates a linked SourceTextModule graph and returns its promise.
// SyntheticModule has a general Value completion and uses EvaluateValue.
func (m *Module) Evaluate(s *Scope, tc *TryCatch) (Promise, error) {
	if err := m.check(); err != nil {
		return Promise{}, err
	}
	if m.syntheticCallbackID != 0 {
		return Promise{}, errors.New("gov8: synthetic module evaluation returns a general Value; use EvaluateValue")
	}
	value, err := m.evaluateValueAssumingChecked(s, tc)
	if err != nil {
		return Promise{}, err
	}
	return Promise{value}, nil
}

func (m *Module) evaluateValueAssumingChecked(s *Scope, tc *TryCatch) (Value, error) {
	if s == nil || s.iso != m.iso {
		return Value{}, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, err
	}
	status, err := m.statusAssumingChecked()
	if err != nil {
		return Value{}, err
	}
	if status < ModuleInstantiated {
		return Value{}, fmt.Errorf("gov8: Evaluate requires an instantiated module, got %s", status)
	}
	var tcHandle uintptr
	if tc != nil {
		if tc.iso != m.iso {
			return Value{}, foreignIsolate("trycatch")
		}
		if err := tc.check(); err != nil {
			return Value{}, err
		}
		tcHandle = tc.handle
	}
	var out uintptr
	ensureModuleHotProcs()
	r1, _, _ := moduleEscapingSyscall6(moduleEvaluateAddr, 6,
		m.iso.handleAssumingCheck(), m.ctx.handle, sh, m.handle, tcHandle,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("Module.Evaluate", r1)
	}
	return Value{iso: m.iso, sc: s, h: out}, nil
}

// EvaluateValue evaluates either module kind and returns V8's raw completion
// value. SourceTextModule callers normally use Evaluate. SyntheticModule
// callbacks may return any Value; after the first evaluation, repeated
// evaluation returns V8's fulfilled top-level Promise without invoking the
// callback again.
func (m *Module) EvaluateValue(s *Scope, tc *TryCatch) (Value, error) {
	if err := m.check(); err != nil {
		return Value{}, err
	}
	if m.syntheticCallbackID != 0 {
		return m.evaluateSyntheticValue(s, tc)
	}
	return m.evaluateValueAssumingChecked(s, tc)
}

// Namespace returns the module namespace once the graph has been instantiated.
func (m *Module) Namespace(s *Scope) (Value, error) {
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
		return Value{}, fmt.Errorf("gov8: Namespace requires an instantiated module, got %s", status)
	}
	var out uintptr
	r1, _, _ := proc("gov8_module_namespace").Call(m.iso.handleAssumingCheck(), sh,
		m.handle, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("Module.Namespace", r1)
	}
	return Value{iso: m.iso, sc: s, h: out}, nil
}

// Exception returns the exception stored by an errored module.
func (m *Module) Exception(s *Scope) (Value, error) {
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
	if status != ModuleErrored {
		return Value{}, fmt.Errorf("gov8: Exception requires Errored module, got %s", status)
	}
	var out uintptr
	r1, _, _ := proc("gov8_module_exception").Call(m.iso.handleAssumingCheck(), sh,
		m.handle, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("Module.Exception", r1)
	}
	return Value{iso: m.iso, sc: s, h: out}, nil
}
