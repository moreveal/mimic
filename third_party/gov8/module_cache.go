//go:build windows && amd64

package gov8

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	moduleCacheProcsOnce      sync.Once
	moduleCacheCompileAddr    uintptr
	moduleCacheCreateAddr     uintptr
	moduleCacheReadDeleteAddr uintptr
)

func ensureModuleCacheProcs() {
	moduleCacheProcsOnce.Do(func() {
		moduleCacheCompileAddr = proc("gov8_module_cache_compile").Addr()
		moduleCacheCreateAddr = proc("gov8_module_cache_create").Addr()
		moduleCacheReadDeleteAddr = proc("gov8_module_cache_read_delete").Addr()
	})
}

// UnboundModuleScript is the context-independent compiled script underlying a
// SourceTextModule. It is isolate- and thread-affine, but remains valid after
// the producing Module and its Context are closed.
type UnboundModuleScript struct {
	iso           *Isolate
	handle        uintptr
	closed        bool
	cacheable     bool
	cacheCapacity int
}

func (u *UnboundModuleScript) check() error {
	if u == nil {
		return errors.New("gov8: nil unbound module script")
	}
	if u.closed {
		return errors.New("gov8: unbound module script used after Close")
	}
	if u.iso == nil || !u.cacheable || u.handle == 0 {
		return ErrModuleNotCacheable
	}
	if err := u.iso.check(); err != nil {
		return err
	}
	return nil
}

// ErrModuleNotCacheable is returned before reaching V8 when a caller-created
// zero value or a module without SourceTextModule provenance is used for cache
// production. Upstream CreateCodeCache requires genuine compiled provenance.
var ErrModuleNotCacheable = errors.New("gov8: module script is not cacheable")

// GetUnboundModuleScript returns the persistent context-independent script
// underlying m. The returned wrapper has independent lifetime and must close.
func (m *Module) GetUnboundModuleScript() (*UnboundModuleScript, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	sourceText, err := m.IsSourceTextModule()
	if err != nil {
		return nil, err
	}
	if !sourceText {
		return nil, ErrModuleNotCacheable
	}
	var handle uintptr
	r1, _, _ := proc("gov8_module_cache_get_unbound").Call(
		m.handle, uintptr(unsafe.Pointer(&handle)))
	if int64(r1) < 0 {
		return nil, shimError("Module.GetUnboundModuleScript", r1)
	}
	if handle == 0 {
		return nil, errors.New("gov8: GetUnboundModuleScript returned empty")
	}
	return &UnboundModuleScript{iso: m.iso, handle: handle, cacheable: true}, nil
}

// Close releases the persistent unbound-script root.
func (u *UnboundModuleScript) Close() error {
	if u == nil {
		return errors.New("gov8: nil unbound module script")
	}
	if u.iso == nil {
		return ErrModuleNotCacheable
	}
	if err := u.iso.check(); err != nil {
		return err
	}
	if u.closed {
		return errors.New("gov8: unbound module script already closed")
	}
	if err := callErr("UnboundModuleScript.Close",
		proc("gov8_module_cache_unbound_dispose"), u.handle); err != nil {
		return err
	}
	u.closed = true
	u.handle = 0
	return nil
}

func (u *UnboundModuleScript) metadata(s *Scope, field uintptr) (Value, error) {
	if err := u.check(); err != nil {
		return Value{}, err
	}
	if s == nil || s.iso != u.iso {
		return Value{}, foreignIsolate("scope")
	}
	scopeHandle, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_module_cache_unbound_metadata").Call(
		u.iso.handleAssumingCheck(), scopeHandle, u.handle, field,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("UnboundModuleScript.Metadata", r1)
	}
	return Value{iso: u.iso, sc: s, h: out}, nil
}

// SourceURL returns the value read from the sourceURL magic comment.
func (u *UnboundModuleScript) SourceURL(s *Scope) (Value, error) {
	return u.metadata(s, 0)
}

// SourceMappingURL returns the value read from the sourceMappingURL comment.
func (u *UnboundModuleScript) SourceMappingURL(s *Scope) (Value, error) {
	return u.metadata(s, 1)
}

// ScriptID returns V8's unique script identifier.
func (u *UnboundModuleScript) ScriptID() (int32, error) {
	if err := u.check(); err != nil {
		return 0, err
	}
	var out int32
	r1, _, _ := proc("gov8_module_cache_unbound_script_id").Call(
		u.handle, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("UnboundModuleScript.ScriptID", r1)
	}
	return out, nil
}

// ModuleCodeCache is an engine-produced SourceTextModule cache. Its bytes are
// intentionally opaque and unconstructible outside this package. A cache owns
// a Go copy, so it remains usable after its producer isolate closes.
type ModuleCodeCache struct {
	data       []byte
	provenance bool
}

// Len returns the opaque cache payload size.
func (c *ModuleCodeCache) Len() int {
	if c == nil {
		return 0
	}
	return len(c.data)
}

// Equal reports whether two opaque cache payloads are byte-identical without
// exposing either payload.
func (c *ModuleCodeCache) Equal(other *ModuleCodeCache) bool {
	return c != nil && other != nil && c.provenance && other.provenance &&
		bytes.Equal(c.data, other.data)
}

func validateModuleCodeCacheLength(length int) error {
	if length <= 0 {
		return errors.New("gov8: empty module code cache")
	}
	if uint64(length) > uint64(math.MaxInt32) {
		return fmt.Errorf("gov8: module code cache length %d exceeds int32", length)
	}
	return nil
}

// CreateCodeCache serializes the compiled unbound module. The provenance guard
// runs before the V8 API, and the returned cache is a process-independent copy.
func (u *UnboundModuleScript) CreateCodeCache() (*ModuleCodeCache, error) {
	if err := u.check(); err != nil {
		return nil, err
	}
	ensureModuleCacheProcs()
	cacheHandle, _, _ := syscall.Syscall(moduleCacheCreateAddr, 1, u.handle, 0, 0)
	if cacheHandle == 0 {
		return nil, shimError("UnboundModuleScript.CreateCodeCache", 0)
	}
	capacity := u.cacheCapacity
	if capacity <= 0 {
		capacity = 4096
	}
	for attempt := 0; attempt < 2; attempt++ {
		buffer := make([]byte, capacity)
		var length int64
		r1, _, _ := syscall.Syscall6(moduleCacheReadDeleteAddr, 4,
			cacheHandle, uintptr(sliceUnsafePointer(buffer)), uintptr(capacity),
			uintptr(unsafe.Pointer(&length)), 0, 0)
		runtime.KeepAlive(buffer)
		if int64(r1) == errNoMemory {
			if length <= 0 || length > math.MaxInt32 {
				return nil, shimError("UnboundModuleScript.CreateCodeCache", r1)
			}
			capacity = int(length)
			cacheHandle, _, _ = syscall.Syscall(moduleCacheCreateAddr, 1, u.handle, 0, 0)
			if cacheHandle == 0 {
				return nil, shimError("UnboundModuleScript.CreateCodeCache", 0)
			}
			continue
		}
		if int64(r1) < 0 {
			return nil, shimError("UnboundModuleScript.CreateCodeCache", r1)
		}
		if length <= 0 || length > int64(len(buffer)) {
			return nil, errors.New("gov8: invalid module code-cache length")
		}
		u.cacheCapacity = int(length)
		// buffer is already the Go-owned copy filled by the deleting native read.
		// A full slice expression prevents callers inside this package from
		// appending into unused capacity and retaining unrelated bytes.
		return &ModuleCodeCache{data: buffer[:length:length], provenance: true}, nil
	}
	return nil, errors.New("gov8: module code-cache read failed")
}

// CompileModuleCached compiles source with an optional opaque cache. rejected
// is V8's CachedData rejected bit. Rejection is non-fatal: V8 recompiles the
// supplied source and returns a usable module.
func (c *Context) CompileModuleCached(s *Scope, source string,
	options ModuleCompileOptions, cache *ModuleCodeCache,
	tc *TryCatch) (module *Module, rejected bool, err error) {
	if err = c.check(); err != nil {
		return nil, false, err
	}
	if s == nil || s.iso != c.iso {
		return nil, false, foreignIsolate("scope")
	}
	scopeHandle, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, false, err
	}
	if tc != nil {
		if tc.iso != c.iso {
			return nil, false, foreignIsolate("trycatch")
		}
		if err = tc.check(); err != nil {
			return nil, false, err
		}
	}
	if len(source) > math.MaxInt32 || len(options.ResourceName) > math.MaxInt32 {
		return nil, false, errors.New("gov8: module source or resource name exceeds int32")
	}
	var cacheBytes []byte
	consume := uintptr(0)
	if cache != nil {
		if !cache.provenance {
			return nil, false, ErrModuleNotCacheable
		}
		if err = validateModuleCodeCacheLength(len(cache.data)); err != nil {
			return nil, false, err
		}
		cacheBytes = cache.data
		consume = 1
	}
	var tryCatchHandle uintptr
	if tc != nil {
		tryCatchHandle = tc.handle
	}
	sourceBytes := []byte(source)
	nameBytes := []byte(options.ResourceName)
	var out uintptr
	var rejectedInt int32
	ensureModuleCacheProcs()
	// Syscall15 is the fixed-arity form of SyscallN on Windows. Using it here
	// avoids a heap-allocated variadic frame for this 15-argument hot path; its
	// uintptrkeepalive contract also keeps the transient Go buffers live.
	r1, _, _ := syscall.Syscall15(moduleCacheCompileAddr, 15,
		c.iso.handleAssumingCheck(), c.handle, scopeHandle, tryCatchHandle,
		bytesArg(sourceBytes), uintptr(len(sourceBytes)),
		bytesArg(nameBytes), uintptr(len(nameBytes)),
		uintptr(options.LineOffset), uintptr(options.ColumnOffset),
		bytesArg(cacheBytes), uintptr(len(cacheBytes)), consume,
		uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&rejectedInt)))
	runtime.KeepAlive(sourceBytes)
	runtime.KeepAlive(nameBytes)
	runtime.KeepAlive(cacheBytes)
	if int64(r1) < 0 {
		return nil, false, shimError("CompileModuleCached", r1)
	}
	module = &Module{iso: c.iso, ctx: c, handle: out}
	moduleRegMu.Lock()
	moduleByHandle[out] = module
	moduleRegMu.Unlock()
	return module, rejectedInt != 0, nil
}
