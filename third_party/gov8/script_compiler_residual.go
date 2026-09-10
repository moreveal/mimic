//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"unsafe"
)

// Additional CompileOptions exposed by rusty_v8 152.2.0. CompileOptions is a
// bit set: unknown bits are deliberately preserved and forwarded because the
// pinned engine ignores them for classic compilation.
const (
	OptProduceCompileHints                       CompileOptions = 1 << 2
	OptConsumeCompileHints                       CompileOptions = 1 << 3
	OptFollowCompileHintsMagicComment            CompileOptions = 1 << 4
	OptFollowCompileHintsPerFunctionMagicComment CompileOptions = 1 << 5
)

// NoCacheReason reports why an embedder is not requesting or supplying a code
// cache. It is metadata for V8's compilation accounting and does not alter the
// result of a classic compile.
type NoCacheReason int32

const (
	NoCacheNoReason NoCacheReason = iota
	NoCacheBecauseCachingDisabled
	NoCacheBecauseNoResource
	NoCacheBecauseInlineScript
	NoCacheBecauseModule
	NoCacheBecauseStreamingSource
	NoCacheBecauseInspector
	NoCacheBecauseScriptTooSmall
	NoCacheBecauseCacheTooCold
	NoCacheBecauseV8Extension
	NoCacheBecauseExtensionModule
	NoCacheBecausePacScript
	NoCacheBecauseInDocumentWrite
	NoCacheBecauseResourceWithNoCacheHandler
	NoCacheBecauseDeferredProduceCodeCache
)

// ScriptCompilerOrigin is the scope-local origin accepted by
// ScriptCompilerSource. ResourceName may be any JavaScript Value, including
// undefined or an object. SourceMapURL is nil for no source-map value and may
// otherwise point to any Value. HostDefinedOptions is restricted to the exact
// PrimitiveArray type used by the pinned public API.
type ScriptCompilerOrigin struct {
	ResourceName        Value
	LineOffset          int32
	ColumnOffset        int32
	IsSharedCrossOrigin bool
	ScriptID            int32
	SourceMapURL        *Value
	IsOpaque            bool
	IsWasm              bool
	HostDefinedOptions  *PrimitiveArray
}

func (o *ScriptCompilerOrigin) flags() uintptr {
	var flags uintptr
	if o.IsSharedCrossOrigin {
		flags |= 1
	}
	if o.IsOpaque {
		flags |= 2
	}
	if o.IsWasm {
		flags |= 4
	}
	return flags
}

// ScriptCompilerCachedData is a snapshot of the cached-data state retained by
// a ScriptCompilerSource. Bytes is a copy and remains unchanged when V8 marks
// the data rejected.
type ScriptCompilerCachedData struct {
	Present  bool
	Rejected bool
	Bytes    []byte
}

// ScriptCompilerSource owns source text, optional scope-local origin handles,
// and optional copied cache bytes. It must only be compiled while all Values
// in Origin are live. The source object itself owns no native allocation.
type ScriptCompilerSource struct {
	mu            sync.Mutex
	text          string
	origin        *ScriptCompilerOrigin
	cache         []byte
	cachePresent  bool
	cacheRejected bool
}

// NewScriptCompilerSource creates a source without cached data.
func NewScriptCompilerSource(source string, origin *ScriptCompilerOrigin) *ScriptCompilerSource {
	return &ScriptCompilerSource{text: source, origin: origin}
}

// NewScriptCompilerSourceWithCachedData creates a source with caller-provided
// cached data. An empty slice is still present cached data, matching
// CachedData::new(&[]) and its graceful rejection during compilation.
func NewScriptCompilerSourceWithCachedData(source string, origin *ScriptCompilerOrigin, cache []byte) (*ScriptCompilerSource, error) {
	if uint64(len(cache)) > math.MaxInt32 {
		return nil, fmt.Errorf("gov8: cached data length %d exceeds int32", len(cache))
	}
	return &ScriptCompilerSource{
		text:         source,
		origin:       origin,
		cache:        append([]byte(nil), cache...),
		cachePresent: true,
	}, nil
}

// CachedData returns a copy of the source's current cached-data state.
func (s *ScriptCompilerSource) CachedData() ScriptCompilerCachedData {
	if s == nil {
		return ScriptCompilerCachedData{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.cachePresent {
		return ScriptCompilerCachedData{}
	}
	return ScriptCompilerCachedData{
		Present:  true,
		Rejected: s.cacheRejected,
		Bytes:    append([]byte(nil), s.cache...),
	}
}

// Text returns the immutable source text.
func (s *ScriptCompilerSource) Text() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.text
}

func validateScriptCompilerOrigin(c *Context, s *Scope, origin *ScriptCompilerOrigin) (resource, sourceMap, host uintptr, err error) {
	if origin == nil {
		return 0, 0, 0, nil
	}
	if err := origin.ResourceName.check(); err != nil {
		return 0, 0, 0, fmt.Errorf("gov8: script origin resource name: %w", err)
	}
	if origin.ResourceName.iso != c.iso {
		return 0, 0, 0, foreignIsolate("script origin resource name")
	}
	resource = origin.ResourceName.h
	if origin.SourceMapURL != nil {
		if err := origin.SourceMapURL.check(); err != nil {
			return 0, 0, 0, fmt.Errorf("gov8: script origin source map: %w", err)
		}
		if origin.SourceMapURL.iso != c.iso {
			return 0, 0, 0, foreignIsolate("script origin source map")
		}
		sourceMap = origin.SourceMapURL.h
	}
	if origin.HostDefinedOptions != nil {
		if err := origin.HostDefinedOptions.check(); err != nil {
			return 0, 0, 0, fmt.Errorf("gov8: script origin host-defined options: %w", err)
		}
		if origin.HostDefinedOptions.iso != c.iso {
			return 0, 0, 0, foreignIsolate("script origin host-defined options")
		}
		host = origin.HostDefinedOptions.h
	}
	return resource, sourceMap, host, nil
}

// CompileScriptCompilerSource compiles a classic ScriptCompilerSource in the
// context. ConsumeCodeCache without present cached data is rejected before FFI
// because the pinned native call access-violates. Syntax failures are routed
// through tc exactly like Context.Compile.
func (c *Context) CompileScriptCompilerSource(scope *Scope, source *ScriptCompilerSource, options CompileOptions, reason NoCacheReason, tc *TryCatch) (*Script, error) {
	if source == nil {
		return nil, errors.New("gov8: nil script compiler source")
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	if options&OptConsumeCodeCache != 0 && !source.cachePresent {
		return nil, errors.New("gov8: ConsumeCodeCache requires cached data")
	}
	if reason < NoCacheNoReason || reason > NoCacheBecauseDeferredProduceCodeCache {
		return nil, fmt.Errorf("gov8: unknown no-cache reason %d", reason)
	}
	if uint64(len(source.text)) > math.MaxInt32 {
		return nil, fmt.Errorf("gov8: source length %d exceeds int32", len(source.text))
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	if scope == nil || scope.iso != c.iso {
		return nil, foreignIsolate("scope")
	}
	sh, err := scope.checkedHandleAssumingIsolate()
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
	resource, sourceMap, host, err := validateScriptCompilerOrigin(c, scope, source.origin)
	if err != nil {
		return nil, err
	}
	src := []byte(source.text)
	var originPresent, tcv, cachePresent uintptr
	var line, column, scriptID int32
	var flags uintptr
	if source.origin != nil {
		originPresent = 1
		line = source.origin.LineOffset
		column = source.origin.ColumnOffset
		scriptID = source.origin.ScriptID
		flags = source.origin.flags()
	}
	if tc != nil {
		tcv = tc.handle
	}
	if source.cachePresent {
		cachePresent = 1
	}
	var out uintptr
	var rejected int32
	r1, _, _ := proc("gov8_scr_compile").Call(
		c.iso.handleAssumingCheck(), c.handle, sh, tcv,
		bytesArg(src), uintptr(len(src)), originPresent, resource,
		uintptr(line), uintptr(column), uintptr(scriptID), sourceMap, host, flags,
		uintptr(uint32(options)), uintptr(int32(reason)), cachePresent,
		bytesArg(source.cache), uintptr(len(source.cache)),
		uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&rejected)))
	runtime.KeepAlive(src)
	runtime.KeepAlive(source.cache)
	if source.cachePresent {
		source.cacheRejected = rejected != 0
	}
	if int64(r1) < 0 {
		return nil, shimError("CompileScriptCompilerSource", r1)
	}
	return &Script{iso: c.iso, ctx: c, handle: out}, nil
}

// CurrentHostDefinedOptions returns the PrimitiveArray attached to the
// currently executing script origin. Outside script execution it reports
// present=false.
func (i *Isolate) CurrentHostDefinedOptions(scope *Scope) (*PrimitiveArray, bool, error) {
	if i == nil {
		return nil, false, errors.New("gov8: nil isolate")
	}
	if err := i.check(); err != nil {
		return nil, false, err
	}
	if scope == nil || scope.iso != i {
		return nil, false, foreignIsolate("scope")
	}
	sh, err := scope.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, false, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_scr_current_host_options").Call(
		i.handleAssumingCheck(), sh, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, false, shimError("Isolate.CurrentHostDefinedOptions", r1)
	}
	if out == 0 {
		return nil, false, nil
	}
	return &PrimitiveArray{Data: Data{iso: i, sc: scope, h: out}}, true, nil
}
