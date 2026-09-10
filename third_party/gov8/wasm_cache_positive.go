//go:build windows && amd64

package gov8

import (
	"bytes"
	"errors"
	"runtime"
	"unsafe"
)

// SerializedWasmModuleCache is an immutable serialized CompiledWasmModule
// together with the exact wire bytes from which it was produced. Keeping both
// private lets SetCachedCompiledModule reject a wire mismatch before V8's
// process-fatal deserializer precondition. SerializedBytes returns a copy for
// persistence or diagnostics; arbitrary bytes deliberately cannot be turned
// back into this safe-provenance type.
//
// This producer type is an intentional Go extension: rusty_v8 152.2.0 omits
// the public V8 CompiledWasmModule::Serialize method.
type SerializedWasmModuleCache struct {
	serialized []byte
	wire       []byte
}

// Len returns the serialized cache size in bytes.
func (c *SerializedWasmModuleCache) Len() int {
	if c == nil {
		return 0
	}
	return len(c.serialized)
}

// SerializedBytes returns an independent copy suitable for persistence.
// Use SetCachedCompiledModule with the original typed cache when provenance is
// available; the raw byte setter retains rusty_v8 compatibility but inherits
// V8's fatal mismatch/truncation preconditions.
func (c *SerializedWasmModuleCache) SerializedBytes() []byte {
	if c == nil {
		return nil
	}
	return append([]byte(nil), c.serialized...)
}

// Serialize creates deterministic compiled-module cache bytes and binds them
// to a private copy of this module's source wire bytes. Serialization requires
// optimized Wasm code; V8 can report the module as not serializable when only
// Liftoff code exists.
func (m *CompiledWasmModule) Serialize() (*SerializedWasmModuleCache, error) {
	if m == nil {
		return nil, errors.New("gov8: nil compiled wasm module")
	}
	wire, err := m.WireBytes()
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.handle == 0 {
		return nil, errors.New("gov8: compiled wasm module used after Close")
	}
	var length uintptr
	if err := callErr("CompiledWasmModule.Serialize", proc("gov8_wcp_compiled_serialize"),
		m.handle, 0, 0, uintptr(unsafe.Pointer(&length))); err != nil {
		return nil, err
	}
	if length > uintptr(^uint(0)>>1) {
		return nil, errors.New("gov8: serialized wasm module exceeds Go slice capacity")
	}
	serialized := make([]byte, int(length))
	if length != 0 {
		capacity := length
		if err := callErr("CompiledWasmModule.Serialize", proc("gov8_wcp_compiled_serialize"),
			m.handle, slicePointer(serialized), capacity, uintptr(unsafe.Pointer(&length))); err != nil {
			return nil, err
		}
		runtime.KeepAlive(serialized)
		if length > capacity {
			return nil, errors.New("gov8: serialized wasm module size changed during copy")
		}
		serialized = serialized[:int(length)]
	}
	return &SerializedWasmModuleCache{
		serialized: serialized,
		wire:       append([]byte(nil), wire...),
	}, nil
}

// SetCachedCompiledModule offers a provenance-checked serialized module to
// V8. A mismatched source is rejected before the native fatal boundary. Like
// the raw setter, this operation is one-shot once native consumption begins.
func (m *ModuleCachingInterface) SetCachedCompiledModule(cache *SerializedWasmModuleCache) (bool, error) {
	if m == nil {
		return false, errors.New("gov8: nil module caching interface")
	}
	if cache == nil || len(cache.serialized) == 0 || len(cache.wire) == 0 {
		return false, errors.New("gov8: nil or empty serialized wasm module cache")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.active || m.handle == 0 {
		return false, errors.New("gov8: module caching interface is no longer active")
	}
	if m.setCalled {
		return false, errors.New("gov8: cached compiled module bytes already set")
	}
	wire, err := m.wireBytesLocked()
	if err != nil {
		return false, err
	}
	if !bytes.Equal(wire, cache.wire) {
		return false, errors.New("gov8: serialized wasm cache was produced for different wire bytes")
	}
	return m.setCachedCompiledModuleBytesLocked(cache.serialized)
}
