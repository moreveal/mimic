//go:build windows && amd64

package gov8

import (
	"errors"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	wasmFromCompiledProcOnce sync.Once
	wasmFromCompiledProcAddr uintptr
)

func resolveWasmFromCompiledProc() {
	wasmFromCompiledProcAddr = proc("gov8_wasm_module_from_compiled").Addr()
}

// WasmModuleObject is a scope-local WebAssembly.Module value.
type WasmModuleObject struct{ Value }

// WasmMemoryObject is a scope-local WebAssembly.Memory value.
type WasmMemoryObject struct{ Value }

// CompiledWasmModule owns V8's shareable compiled representation. It is safe
// to inspect or use from another thread or isolate until Close.
type CompiledWasmModule struct {
	mu     sync.Mutex
	handle uintptr
	closed bool
}

func (v Value) wasmPredicate(kind uintptr) (bool, error) {
	if err := v.check(); err != nil {
		return false, err
	}
	var out int32
	r1, _, _ := proc("gov8_wasm_value_predicate").Call(v.iso.handleAssumingCheck(), v.sc.handle,
		v.h, kind, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return false, shimError("Value.WasmPredicate", r1)
	}
	return out != 0, nil
}

// IsWasmModuleObject reports whether v is a WebAssembly.Module.
func (v Value) IsWasmModuleObject() (bool, error) { return v.wasmPredicate(0) }

// IsWasmMemoryObject reports whether v is a WebAssembly.Memory.
func (v Value) IsWasmMemoryObject() (bool, error) { return v.wasmPredicate(1) }

// AsWasmModuleObject performs a checked local-value conversion.
func AsWasmModuleObject(v Value) (*WasmModuleObject, error) {
	ok, err := v.IsWasmModuleObject()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("gov8: value is not a WasmModuleObject")
	}
	return &WasmModuleObject{Value: v}, nil
}

// AsWasmMemoryObject performs a checked local-value conversion.
func AsWasmMemoryObject(v Value) (*WasmMemoryObject, error) {
	ok, err := v.IsWasmMemoryObject()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("gov8: value is not a WasmMemoryObject")
	}
	return &WasmMemoryObject{Value: v}, nil
}

// CompileWasmModule synchronously compiles WebAssembly wire bytes. Malformed
// bytes return an exception error and are captured by tc when supplied.
func (c *Context) CompileWasmModule(s *Scope, wireBytes []byte, tc *TryCatch) (*WasmModuleObject, error) {
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
	var tcHandle uintptr
	if tc != nil {
		if tc.iso != c.iso {
			return nil, foreignIsolate("trycatch")
		}
		if err := tc.check(); err != nil {
			return nil, err
		}
		tcHandle = tc.handle
	}
	var out uintptr
	r1, _, _ := proc("gov8_wasm_module_compile").Call(c.iso.handleAssumingCheck(), c.handle, sh,
		tcHandle, slicePointer(wireBytes), uintptr(len(wireBytes)), uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(wireBytes)
	if int64(r1) < 0 {
		return nil, shimError("CompileWasmModule", r1)
	}
	return &WasmModuleObject{Value: Value{iso: c.iso, sc: s, h: out}}, nil
}

// WasmModuleFromCompiled creates a local module object from a shareable
// compiled module. The compiled handle remains owned by the caller.
func (c *Context) WasmModuleFromCompiled(s *Scope, compiled *CompiledWasmModule) (result *WasmModuleObject, err error) {
	value, err := c.wasmModuleFromCompiled(s, compiled)
	if err == nil {
		result = &WasmModuleObject{Value: value}
	}
	return result, err
}

// wasmModuleFromCompiled performs the validated native restoration without
// imposing pointer-wrapper escape decisions on callers. Keeping the public
// wrapper tiny lets the compiler place a short-lived WasmModuleObject on the
// caller's stack; retained results preserve the same heap-backed pointer API.
func (c *Context) wasmModuleFromCompiled(s *Scope, compiled *CompiledWasmModule) (Value, error) {
	if err := c.check(); err != nil {
		return Value{}, err
	}
	if s == nil || s.iso != c.iso {
		return Value{}, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, err
	}
	if compiled == nil {
		return Value{}, errors.New("gov8: nil compiled wasm module")
	}
	wasmFromCompiledProcOnce.Do(resolveWasmFromCompiledProc)
	compiled.mu.Lock()
	if compiled.closed {
		compiled.mu.Unlock()
		return Value{}, errors.New("gov8: compiled wasm module used after Close")
	}
	var out uintptr
	// Proc.Call marks every uintptr argument as escaping. This fixed-arity
	// syscall keeps the output slot and argument frame on the stack while
	// preserving the Windows uintptr keep-alive contract. The compiled lock
	// remains held across native recreation so Close cannot free the shared
	// representation concurrently.
	r1, _, _ := syscall.Syscall6(wasmFromCompiledProcAddr, 5,
		c.iso.handleAssumingCheck(), c.handle, sh, compiled.handle,
		uintptr(unsafe.Pointer(&out)), 0)
	compiled.mu.Unlock()
	if int64(r1) < 0 {
		return Value{}, shimError("WasmModuleFromCompiled", r1)
	}
	return Value{iso: c.iso, sc: s, h: out}, nil
}

// CompiledModule returns a newly owned compiled representation.
func (m *WasmModuleObject) CompiledModule() (*CompiledWasmModule, error) {
	if m == nil {
		return nil, errors.New("gov8: nil wasm module")
	}
	if err := m.check(); err != nil {
		return nil, err
	}
	isModule, err := m.Value.IsWasmModuleObject()
	if err != nil {
		return nil, err
	}
	if !isModule {
		return nil, errors.New("gov8: value is not a WasmModuleObject")
	}
	var out uintptr
	r1, _, _ := proc("gov8_wasm_module_get_compiled").Call(m.iso.handleAssumingCheck(), m.sc.handle,
		m.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("WasmModuleObject.CompiledModule", r1)
	}
	return &CompiledWasmModule{handle: out}, nil
}

func (m *CompiledWasmModule) bytes(sourceURL bool) ([]byte, error) {
	if m == nil {
		return nil, errors.New("gov8: nil compiled wasm module")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, errors.New("gov8: compiled wasm module used after Close")
	}
	flag := uintptr(0)
	if sourceURL {
		flag = 1
	}
	var length uintptr
	r1, _, _ := proc("gov8_wasm_compiled_bytes").Call(m.handle, flag, 0, 0, uintptr(unsafe.Pointer(&length)))
	if int64(r1) < 0 {
		return nil, shimError("CompiledWasmModule.Bytes", r1)
	}
	if length > uintptr(^uint(0)>>1) {
		return nil, errors.New("gov8: compiled wasm data exceeds Go slice capacity")
	}
	result := make([]byte, int(length))
	if length == 0 {
		return result, nil
	}
	r1, _, _ = proc("gov8_wasm_compiled_bytes").Call(m.handle, flag, slicePointer(result), length,
		uintptr(unsafe.Pointer(&length)))
	runtime.KeepAlive(result)
	if int64(r1) < 0 {
		return nil, shimError("CompiledWasmModule.Bytes", r1)
	}
	return result, nil
}

// WireBytes returns a copy of the original wasm wire bytes.
func (m *CompiledWasmModule) WireBytes() ([]byte, error) { return m.bytes(false) }

// SourceURL returns the source URL attached during compilation, if any.
func (m *CompiledWasmModule) SourceURL() (string, error) {
	bytes, err := m.bytes(true)
	return string(bytes), err
}

// Close releases the compiled representation. It does not invalidate local
// WasmModuleObjects previously created from it.
func (m *CompiledWasmModule) Close() error {
	if m == nil {
		return errors.New("gov8: nil compiled wasm module")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("gov8: compiled wasm module already closed")
	}
	r1, _, _ := proc("gov8_wasm_compiled_dispose").Call(m.handle)
	if int64(r1) < 0 {
		return shimError("CompiledWasmModule.Close", r1)
	}
	m.closed = true
	m.handle = 0
	return nil
}

// Buffer returns the WebAssembly.Memory object's current ArrayBuffer. Context
// is explicit because gov8 does not keep a context pointer in local Values.
func (m *WasmMemoryObject) Buffer(c *Context) (*ArrayBuffer, error) {
	if m == nil {
		return nil, errors.New("gov8: nil wasm memory")
	}
	if err := m.check(); err != nil {
		return nil, err
	}
	isMemory, err := m.Value.IsWasmMemoryObject()
	if err != nil {
		return nil, err
	}
	if !isMemory {
		return nil, errors.New("gov8: value is not a WasmMemoryObject")
	}
	if c == nil || c.iso != m.iso {
		return nil, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_wasm_memory_buffer").Call(m.iso.handleAssumingCheck(), c.handle, m.sc.handle,
		m.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("WasmMemoryObject.Buffer", r1)
	}
	return &ArrayBuffer{Value: Value{iso: m.iso, sc: m.sc, h: out}}, nil
}
