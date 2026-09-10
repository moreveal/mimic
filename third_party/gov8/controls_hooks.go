//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// Process/isolate controls, V8 flags, and hooks — the Go port of the pinned
// crate's V8 flag/entropy/fatal surface and the Isolate controls & hooks
// (request_garbage_collection_for_testing, clear_kept_objects, memory
// pressure, low memory, atomics-wait toggle, idle, timezone notifications,
// background-task polling, promise hooks and reject callbacks,
// prepare-stack-trace, use counter, modify-code-generation-from-strings,
// message listeners, and OOM / near-heap-limit handlers).
//
// Semantic notes mirrored from the pinned oracle (v8 =152.2.0, V8
// 15.2.124.1-rusty):
//
//   - Flags must be set BEFORE Initialize. After initialization the engine's
//     flag set is frozen: a value-CHANGING SetFlagsFromString fails a fatal
//     CHECK ("Check failed: !IsFrozen().") and aborts the process. This
//     module does not guard that path — it is engine-fatal by design and is
//     characterized out-of-process.
//   - The entropy source pins per-isolate PRNG seeding: the source installed
//     before Initialize seeds every fresh isolate identically; replacing it
//     after Initialize still affects isolates created afterwards. A source
//     that returns false declines to fill the buffer and the engine falls
//     back to its default randomness.
//   - RequestGarbageCollectionForTesting requires --expose-gc (set pre-init);
//     without it the engine fails a fatal CHECK and aborts. The fatal
//     handler registered via SetFatalErrorHandler is NOT invoked at that
//     site (site-specific engine behavior, pinned by the oracle).
//   - Fatal/OOM/near-heap-limit handlers have process-fatal semantics: the
//     Go handler only OBSERVES; returning lets the engine abort the process
//     (STATUS_BREAKPOINT). There is deliberately no recovery path. The
//     near-heap-limit callback may raise or shrink the limit; only the most
//     recently added callback is invoked.
//   - The modify-code-generation-from-strings callback is consulted only
//     when the context disallows code generation from strings OR the eval
//     source is not a string; it can block, rewrite the source, or pass
//     through.
//
// Ownership and safety (same rules as the rest of the module):
//   - No Go pointer ever crosses into the engine. Every hook dispatch is
//     routed through ONE shim trampoline family and an integer-safe registry
//     keyed by (hook kind, engine isolate); the engine only ever sees static
//     trampolines.
//   - Hook callbacks run synchronously inside engine execution on the
//     isolate's owning thread (the process-global entropy and fatal handlers
//     may run on ANY engine thread, including background ones). A panic in a
//     hook is recovered and converted into the process fail-fast abort
//     documented for native callbacks — unwinding a Go panic through engine
//     C++ frames is unsupported.
//   - Values delivered to hook callbacks are scope-local wires valid only
//     for the duration of the callback; retaining them past the callback is
//     a use-after-free. The entropy callback's buffer is engine-owned
//     scratch memory: fill it, never retain it.

// ---------------------------------------------------------------------------
// Process-level: flags, entropy, fatal handler
// ---------------------------------------------------------------------------

// EnableWebAssemblyTrapHandler activates V8's trap-based WebAssembly bounds
// checks. Call it before Initialize. If useV8SignalHandler is true, V8 installs
// its own signal handler; otherwise the embedder is responsible for routing
// faults to V8. The result reports whether trap handling is available in the
// pinned engine build.
func EnableWebAssemblyTrapHandler(useV8SignalHandler bool) (bool, error) {
	if err := loadShim(); err != nil {
		return false, err
	}
	flag := uintptr(0)
	if useV8SignalHandler {
		flag = 1
	}
	r1, _, _ := proc("gov8_ch_enable_wasm_trap_handler").Call(flag)
	if int64(r1) < 0 {
		return false, shimError("EnableWebAssemblyTrapHandler", r1)
	}
	return r1 == 1, nil
}

// SetFlagsFromCommandLine passes args to the engine BEFORE Initialize.
// Recognized flags are consumed; the args the engine did not understand are
// returned in order (including the program name at args[0]). The engine
// exits the process on --help; do not pass it.
func SetFlagsFromCommandLine(args []string) ([]string, error) {
	return setFlagsFromCommandLine(args, nil)
}

// SetFlagsFromCommandLineWithUsage is the usage-bearing form of
// SetFlagsFromCommandLine. V8 prints usage followed by its flag catalogue when
// args requests help. As in rusty_v8, usage must not contain an embedded NUL.
func SetFlagsFromCommandLineWithUsage(args []string, usage string) ([]string, error) {
	return setFlagsFromCommandLine(args, &usage)
}

func setFlagsFromCommandLine(args []string, usage *string) ([]string, error) {
	if err := loadShim(); err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, errors.New("gov8: command line requires argv[0]")
	}
	cstrs := make([][]byte, len(args))
	ptrs := make([]uintptr, len(args))
	origin := make(map[uintptr]string, len(args))
	for i, a := range args {
		if strings.IndexByte(a, 0) >= 0 {
			return nil, fmt.Errorf("gov8: command-line argument %d contains NUL", i)
		}
		b := make([]byte, len(a)+1)
		copy(b, a)
		cstrs[i] = b
		ptrs[i] = uintptr(unsafe.Pointer(&b[0]))
		origin[ptrs[i]] = a
	}
	// The engine reads and shrinks argc in place and compacts argv by
	// shuffling the pointer array (recognized flags are removed); the
	// leftover pointers still address the Go-owned strings, which stay
	// alive via cstrs for the call.
	argc := int32(len(args))
	var argvPtr uintptr
	if len(ptrs) > 0 {
		argvPtr = uintptr(unsafe.Pointer(&ptrs[0]))
	}
	var usageBytes []byte
	var usagePtr uintptr
	procName := "gov8_ch_set_flags_from_command_line"
	if usage != nil {
		if strings.IndexByte(*usage, 0) >= 0 {
			return nil, errors.New("gov8: usage contains NUL")
		}
		usageBytes = make([]byte, len(*usage)+1)
		copy(usageBytes, *usage)
		usagePtr = uintptr(unsafe.Pointer(&usageBytes[0]))
		procName = "gov8_ch_set_flags_from_command_line_with_usage"
	}
	r1, _, _ := proc(procName).Call(uintptr(unsafe.Pointer(&argc)), argvPtr, usagePtr)
	runtime.KeepAlive(cstrs)
	runtime.KeepAlive(usageBytes)
	if int64(r1) < 0 {
		return nil, shimError(procName, r1)
	}
	if argc < 0 || int(argc) > len(args) {
		return nil, fmt.Errorf("gov8: SetFlagsFromCommandLine reported %d leftover args", argc)
	}
	leftover := make([]string, 0, argc)
	for i := 0; i < int(argc); i++ {
		s, ok := origin[ptrs[i]]
		if !ok {
			return nil, fmt.Errorf("gov8: SetFlagsFromCommandLine produced an unknown leftover pointer")
		}
		leftover = append(leftover, s)
	}
	return leftover, nil
}

// SetFlagsFromString sets V8 flags from a whitespace-separated string. Must
// run before Initialize for deterministic behavior: after initialization the
// flag set is frozen and a value-changing write is engine-fatal (documented
// above, characterized out-of-process). Unknown flags are reported to
// stderr by the engine and otherwise ignored; recognized flags in the same
// string still take effect.
func SetFlagsFromString(flags string) error {
	if err := loadShim(); err != nil {
		return err
	}
	b := []byte(flags)
	var p uintptr
	if len(b) > 0 {
		p = uintptr(unsafe.Pointer(&b[0]))
	}
	r1, _, _ := proc("gov8_ch_set_flags_from_string").Call(p, uintptr(len(b)))
	if int64(r1) < 0 {
		return shimError("SetFlagsFromString", r1)
	}
	return nil
}

// EntropySource fills buf with entropy bytes and reports whether it did.
// Returning false declines; the engine then uses its default randomness
// source. It runs on engine threads (any thread, including background
// compilation workers) and must not re-enter the engine or retain buf.
type EntropySource func(buf []byte) bool

var (
	chEntropyOnce sync.Once
	chEntropyErr  error
)

// SetEntropySource installs src as the process entropy source. Installed
// before Initialize it pins every fresh isolate's PRNG identically; called
// again (before or after Initialize) it replaces the previous source and
// still affects isolates created afterwards. A nil source is rejected.
func SetEntropySource(src EntropySource) error {
	if src == nil {
		return errors.New("gov8: entropy source required")
	}
	if err := loadShim(); err != nil {
		return err
	}
	chRegistry.mu.Lock()
	chRegistry.entries[chKey{kind: chKindEntropy, engine: 0}] = &chEntry{entropy: src}
	chRegistry.mu.Unlock()
	chEntropyOnce.Do(func() {
		chEntropyErr = ensureCHDispatcher()
		if chEntropyErr == nil {
			chEntropyErr = callErr("SetEntropySource", proc("gov8_ch_set_entropy_source"))
		}
	})
	return chEntropyErr
}

// FatalErrorHandler observes engine fatal CHECK failures (file is "" and
// line 0 in official builds). It may run on any engine thread in a broken
// process state: it must not re-enter the engine. When it returns, the
// engine aborts the process — this handler only observes.
type FatalErrorHandler func(file string, line int32, message string)

var (
	chFatalOnce sync.Once
	chFatalErr  error
)

// SetFatalErrorHandler installs h as the process fatal-error handler. The
// handler is site-specific in the pinned build: it fires for the
// flags-freeze CHECK and the post-OOM abort, not for every fatal site
// (e.g. the "Must use --expose-gc" CHECK does not call it).
func SetFatalErrorHandler(h FatalErrorHandler) error {
	if h == nil {
		return errors.New("gov8: fatal error handler required")
	}
	if err := loadShim(); err != nil {
		return err
	}
	chRegistry.mu.Lock()
	chRegistry.entries[chKey{kind: chKindFatal, engine: 0}] = &chEntry{fatal: h}
	chRegistry.mu.Unlock()
	chFatalOnce.Do(func() {
		chFatalErr = ensureCHDispatcher()
		if chFatalErr == nil {
			chFatalErr = callErr("SetFatalErrorHandler", proc("gov8_ch_set_fatal_error_handler"))
		}
	})
	return chFatalErr
}

// NewIsolateWithLimits creates a fresh isolate with explicit heap limits
// (CreateParams::heap_limits): ConfigureDefaultsFromHeapSize(initial, max).
// The ceiling makes heap-pressure workloads end in the intended, bounded
// fatal OOM instead of uncontrolled process growth. Creation follows the
// same lifecycle rules as NewIsolate (serialized against Dispose, OS-thread
// pinned, registered as live).
func NewIsolateWithLimits(initialHeapBytes, maxHeapBytes uint64) (*Isolate, error) {
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	tid := currentThreadID()
	if err := beginIsolateCreate(); err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}
	h, err := callHandle("Isolate.NewWithLimits", proc("gov8_ch_isolate_new_heap_limits"),
		uintptr(initialHeapBytes), uintptr(maxHeapBytes))
	if err != nil {
		abandonIsolateCreate()
		runtime.UnlockOSThread()
		return nil, err
	}
	iso := &Isolate{handle: h, tid: tid}
	finishIsolateCreate(iso)
	return iso, nil
}

// ---------------------------------------------------------------------------
// Isolate controls
// ---------------------------------------------------------------------------

// GarbageCollectionType mirrors v8::Isolate::GarbageCollectionType.
type GarbageCollectionType uint32

const (
	// GcFull is a full garbage collection.
	GcFull GarbageCollectionType = 0
	// GcMinor is a minor (young-generation) collection.
	GcMinor GarbageCollectionType = 1
)

// RequestGarbageCollectionForTesting requests a collection of the given
// type. Requires --expose-gc set before Initialize: without it the engine
// fails a fatal CHECK and aborts the process (pinned, engine-fatal; not
// guarded away).
func (i *Isolate) RequestGarbageCollectionForTesting(t GarbageCollectionType) error {
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	return callErr("RequestGarbageCollectionForTesting", proc("gov8_ch_request_gc"), ih, uintptr(t))
}

// ClearKeptObjects drops the engine's kept-object set: WeakRef targets kept
// alive only by that set become collectible at the next full collection.
func (i *Isolate) ClearKeptObjects() error {
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	return callErr("ClearKeptObjects", proc("gov8_ch_clear_kept_objects"), ih)
}

// MemoryPressureLevel mirrors v8::MemoryPressureLevel.
type MemoryPressureLevel uint32

const (
	MemoryPressureNone     MemoryPressureLevel = 0
	MemoryPressureModerate MemoryPressureLevel = 1
	MemoryPressureCritical MemoryPressureLevel = 2
)

// MemoryPressureNotification signals the given pressure level to the
// isolate. All three levels are accepted back-to-back; the isolate stays
// fully usable.
func (i *Isolate) MemoryPressureNotification(l MemoryPressureLevel) error {
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	return callErr("MemoryPressureNotification", proc("gov8_ch_memory_pressure"), ih, uintptr(l))
}

// LowMemoryNotification and the promise-reject callback live in buffer.go
// and promise.go respectively; this slice adds everything else on the
// controls/hooks surface.

// ---------------------------------------------------------------------------
// Hook registry (integer-safe dispatch; no Go pointers cross the boundary)
// ---------------------------------------------------------------------------

// SetAllowAtomicsWait toggles whether Atomics.wait may block on this
// isolate. When disallowed, Atomics.wait throws a TypeError before any
// blocking. The toggle can be flipped repeatedly on a live isolate.
func (i *Isolate) SetAllowAtomicsWait(allow bool) error {
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	a := uintptr(0)
	if allow {
		a = 1
	}
	return callErr("SetAllowAtomicsWait", proc("gov8_ch_set_allow_atomics_wait"), ih, a)
}

// SetIdle marks the isolate as idle (or not). It must be called on the
// isolate's thread while no JS is executing; the flag has no synchronous
// observable effect.
func (i *Isolate) SetIdle(idle bool) error {
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	v := uintptr(0)
	if idle {
		v = 1
	}
	return callErr("SetIdle", proc("gov8_ch_set_idle"), ih, v)
}

// HasPendingBackgroundTasks reports whether the isolate still has
// background work (in this build true is reachable only via background Wasm
// compilation, which is out of scope).
func (i *Isolate) HasPendingBackgroundTasks() (bool, error) {
	ih, err := i.handleChecked()
	if err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_ch_has_pending_background_tasks").Call(ih)
	if int64(r1) < 0 {
		return false, shimError("HasPendingBackgroundTasks", r1)
	}
	return r1 == 1, nil
}

// TimeZoneDetection mirrors v8::Isolate::TimeZoneDetection.
type TimeZoneDetection uint32

const (
	// TZSkip does not redetect the host time zone.
	TZSkip TimeZoneDetection = 0
	// TZRedetect redetects the host time zone and uses it as the default.
	TZRedetect TimeZoneDetection = 1
)

// DateTimeConfigurationChangeNotification tells the engine that date/time
// configuration changed, resetting cached values. Neither mode changes UTC
// date math.
func (i *Isolate) DateTimeConfigurationChangeNotification(d TimeZoneDetection) error {
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	return callErr("DateTimeConfigurationChangeNotification",
		proc("gov8_ch_date_time_config_change"), ih, uintptr(d))
}

// ---------------------------------------------------------------------------
// Hook registry (integer-safe dispatch; no Go pointers cross the boundary)
// ---------------------------------------------------------------------------

// Hook dispatch kinds; keep in sync with the shim's chKind* constants
// (internal/shim/features/controls_hooks.inc).
const (
	chKindEntropy = iota
	chKindFatal
	chKindOOM
	chKindNearHeapLimit
	chKindPromiseHook
	chKindPrepareStackTrace
	chKindUseCounter
	chKindCodegen
	chKindMessageListener
)

// chKey routes a dispatch to its registration: hook kind plus the engine
// isolate pointer the hook was registered for (0 for the process-global
// entropy/fatal slots). A fresh engine isolate starts with every hook slot
// empty, so a recycled isolate address can only ever dispatch a hook that
// was re-registered for it — stale entries cannot fire.
type chKey struct {
	kind   int
	engine uintptr
}

// chEntry holds the Go callbacks for one registered hook slot. Exactly one
// family field is set, except listeners which accumulate (the engine calls
// the trampoline once per registered listener).
type chEntry struct {
	iso *Isolate // nil for process-global kinds

	entropy           EntropySource
	fatal             FatalErrorHandler
	oom               OOMErrorCallback
	nearHeapLimit     NearHeapLimitCallback
	promiseHook       PromiseHook
	prepareStackTrace PrepareStackTraceCallback
	useCounter        UseCounterCallback
	codegen           ModifyCodeGenerationFromStringsCallback
	listeners         []MessageListenerCallback
	listenerLevels    []uint32
}

var chRegistry = struct {
	mu      sync.Mutex
	entries map[chKey]*chEntry
}{entries: make(map[chKey]*chEntry)}

var (
	chDispatcherOnce sync.Once
	chDispatcherErr  error
)

func ensureCHDispatcher() error {
	chDispatcherOnce.Do(func() {
		chDispatcherErr = callErr("SetCHDispatcher", proc("gov8_ch_set_dispatcher"), goCHDispatchPtr)
	})
	return chDispatcherErr
}

// goCHDispatchPtr is the engine-facing callback address for goCHDispatch
// (syscall.NewCallback, exactly like the other dispatch families).
var goCHDispatchPtr = syscall.NewCallback(goCHDispatch)

// chFrame mirrors the shim's ChFrame field-for-field (MSVC x64 layout:
// int32+int32, then pointer-size words). It lives on the trampoline's C++
// stack and is valid only for the synchronous dispatch.
type chFrame struct {
	kind   int32
	pad    int32
	engine uintptr
	a      uintptr
	b      uintptr
	c      uintptr
	scope  unsafe.Pointer
	out    unsafe.Pointer
}

// goCHDispatch is the single Go entry point for every controls/hooks
// trampoline. It runs synchronously inside engine execution. The frame
// arrives as a raw pointer argument (syscall callbacks deliver each
// argument verbatim), so no uintptr→unsafe.Pointer conversion exists on
// this path.
func goCHDispatch(frame *chFrame) uintptr {
	chRegistry.mu.Lock()
	entry := chRegistry.entries[chKey{kind: int(frame.kind), engine: frame.engine}]
	chRegistry.mu.Unlock()
	if entry == nil {
		fatalHostMisuse("gov8: controls/hooks dispatch for unregistered kind %d (engine %#x)", frame.kind, frame.engine)
		return 0
	}
	// Affinity proof for isolate-bound kinds. Process-global entropy/fatal
	// handlers and the isolate OOM observer may legally run on an engine
	// background thread. OOM carries no handles and is process-fatal, so it
	// must not be rejected merely because Go entered its callback elsewhere.
	if entry.iso != nil && frame.kind != chKindOOM {
		if err := entry.iso.check(); err != nil {
			fatalHostMisuse("gov8: hook invoked off the owning thread: %v", err)
			return 0
		}
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in controls/hooks callback: %v\n", r)
			proc("gov8_host_panic_abort").Call()
			// abort() does not return; unreachable.
			panic(r)
		}
	}()
	switch frame.kind {
	case chKindEntropy:
		// frame.a/b: engine-owned buffer + length. Filled by the callback,
		// never retained; the view dies with this call.
		buf := unsafe.Slice((*byte)(wordToPtr(frame.a)), int(frame.b))
		if entry.entropy(buf) {
			return 1
		}
		return 0
	case chKindFatal:
		entry.fatal(cStr(frame.a), int32(frame.c), cStr(frame.b))
		return 0
	case chKindOOM:
		entry.oom(cStr(frame.a), cStr(frame.b), frame.c != 0)
		return 0
	case chKindNearHeapLimit:
		// a/b: current and initial heap limit; the returned word is the new
		// limit (may raise or shrink; passed to the engine verbatim).
		return uintptr(entry.nearHeapLimit(uint64(frame.a), uint64(frame.b)))
	case chKindPromiseHook:
		scope, err := entry.iso.NewScope()
		if err != nil {
			fatalHostMisuse("gov8: promise hook scope: %v", err)
			return 0
		}
		defer func() { _ = scope.Close() }()
		entry.promiseHook(PromiseHookType(frame.a),
			Value{iso: entry.iso, sc: scope, h: frame.b},
			Value{iso: entry.iso, sc: scope, h: frame.c})
		return 0
	case chKindPrepareStackTrace:
		// The trampoline handed us its own scope: values created through it
		// stay valid until the trampoline returns (the returned wire is the
		// result).
		scope := &Scope{iso: entry.iso, handle: uintptr(frame.scope), borrowed: true}
		result, ok := entry.prepareStackTrace(scope,
			Value{iso: entry.iso, sc: scope, h: frame.b},
			Value{iso: entry.iso, sc: scope, h: frame.c})
		if !ok {
			fatalHostMisuse("invalid prepare stack trace callback result: callback returned no value")
			return 0
		}
		if err := validatePrepareStackTraceResult(entry.iso, scope, result); err != nil {
			fatalHostMisuse("invalid prepare stack trace callback result: %v", err)
			return 0
		}
		// frame.out is the address of the trampoline's returned Local;
		// writing the wire (slot address) there re-slots it.
		*(*uintptr)(frame.out) = result.h
		return 0
	case chKindUseCounter:
		entry.useCounter(uint32(frame.a))
		return 0
	case chKindCodegen:
		scope := &Scope{iso: entry.iso, handle: uintptr(frame.scope), borrowed: true}
		allowed, modified := entry.codegen(
			Value{iso: entry.iso, sc: scope, h: frame.b}, frame.c != 0)
		// Verdict protocol with the trampoline: 0 = block, 1 = allow
		// unchanged, 2 = allow with the modified-source wire written to
		// *frame.out (the address of the trampoline's Local; the C++ side
		// escapes the slot into the engine's scope before returning).
		if !allowed {
			return 0
		}
		if modified == nil {
			return 1
		}
		s, err := scope.NewString(*modified)
		if err != nil {
			fatalHostMisuse("gov8: codegen modified source: %v", err)
			return 0
		}
		*(*uintptr)(frame.out) = s.h
		return 2
	case chKindMessageListener:
		scope, err := entry.iso.NewScope()
		if err != nil {
			fatalHostMisuse("gov8: message listener scope: %v", err)
			return 0
		}
		defer func() { _ = scope.Close() }()
		msg := &CallbackMessage{iso: entry.iso, sc: scope, ctxWire: frame.c, h: frame.a}
		exception := Value{iso: entry.iso, sc: scope, h: frame.b}
		// The engine holds ONE all-levels listener per isolate; the
		// per-registration level filtering happens here (the engine's
		// add-message-listener API cannot carry a Go discriminator).
		level, lerr := msg.ErrorLevel()
		if lerr != nil {
			fatalHostMisuse("gov8: message listener level: %v", lerr)
			return 0
		}
		for i, l := range entry.listeners {
			if entry.listenerLevels[i]&uint32(level) == 0 {
				continue
			}
			l(msg, exception)
		}
		return 0
	}
	fatalHostMisuse("gov8: unknown controls/hooks dispatch kind %d", frame.kind)
	return 0
}

// validatePrepareStackTraceResult enforces the lifetime relationship that
// rusty_v8 expresses through Local<'s, Value>: a successful callback result
// must be a non-empty local allocated in this exact callback invocation's
// scope. Go callbacks can capture arbitrary Values, so accepting a local from
// another scope (even on the same isolate) would hand V8 a slot whose lifetime
// is not owned by the trampoline that escapes it.
func validatePrepareStackTraceResult(iso *Isolate, scope *Scope, result Value) error {
	if result.h == 0 {
		return errors.New("empty value")
	}
	if result.iso != iso {
		return foreignIsolate("prepare stack trace result")
	}
	if result.sc != scope {
		return errors.New("value is not owned by the callback scope")
	}
	if err := result.check(); err != nil {
		return err
	}
	return nil
}

// cStr reads a NUL-terminated engine string into a Go string. The pointer is
// transient engine memory; the read is bounded and nothing is retained.
func cStr(p uintptr) string {
	if p == 0 {
		return ""
	}
	// One well-defined conversion of the engine pointer into a bounded view;
	// all further access goes through the Go slice.
	b := unsafe.Slice((*byte)(wordToPtr(p)), maxFatalStrLen)
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	// Unterminated (engine contract violation): take the bounded prefix.
	return string(b)
}

// maxFatalStrLen bounds cStr reads against engine contract violations.
const maxFatalStrLen = 1 << 16

// wordToPtr converts an integer-sized engine word to a pointer using the
// module-wide vet-clean idiom (see abiWordToPtr in callback.go).
func wordToPtr(w uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&w))
}

// replaceCHSlot installs a single-slot hook registration (every Set/Add of
// a single-slot kind replaces the previous registration wholesale, matching
// the engine's one-slot-per-hook model). No state is carried over: two
// sequentially created engine isolates can reuse the same address, so a
// stale entry must never leak into a new registration.
func replaceCHSlot(i *Isolate, kind int, e *chEntry) {
	chRegistry.mu.Lock()
	defer chRegistry.mu.Unlock()
	e.iso = i
	chRegistry.entries[chKey{kind: kind, engine: i.handle}] = e
}

// releaseCHIsolateEntries drops every Go callback retained for i. Match on
// isolate identity, not only the native address: V8 may reuse a disposed
// isolate's address, and releasing stale state must never remove a newer
// isolate's registration at that address. Process-global entries have no
// isolate and are deliberately preserved.
func releaseCHIsolateEntries(i *Isolate) {
	chRegistry.mu.Lock()
	defer chRegistry.mu.Unlock()
	for key, entry := range chRegistry.entries {
		if entry != nil && entry.iso == i {
			delete(chRegistry.entries, key)
		}
	}
}

// registerCHListener appends a message-listener registration. The engine
// holds ONE all-levels listener per isolate (registered on the first Go
// registration); per-registration level filtering happens at dispatch, which
// reproduces the engine's per-registration filtering exactly (the engine
// fires the trampoline once per message; Go fans out to the listeners whose
// level bits match the message level, in registration order).
func registerCHListener(i *Isolate, cb MessageListenerCallback, level uint32) (bool, error) {
	chRegistry.mu.Lock()
	key := chKey{kind: chKindMessageListener, engine: i.handle}
	e := chRegistry.entries[key]
	if e == nil || e.iso != i {
		e = &chEntry{iso: i}
		chRegistry.entries[key] = e
	}
	first := len(e.listeners) == 0
	e.listeners = append(e.listeners, cb)
	e.listenerLevels = append(e.listenerLevels, level)
	chRegistry.mu.Unlock()
	if !first {
		return true, nil // already registered with the engine
	}
	return true, callErr("AddMessageListener(All)",
		proc("gov8_ch_add_message_listener_wl"), i.handle, uintptr(MsgAll))
}

// ---------------------------------------------------------------------------
// Promise hooks
// ---------------------------------------------------------------------------

// PromiseHookType mirrors v8::PromiseHookType.
type PromiseHookType uint32

const (
	PromiseHookInit    PromiseHookType = 0
	PromiseHookResolve PromiseHookType = 1
	PromiseHookBefore  PromiseHookType = 2
	PromiseHookAfter   PromiseHookType = 3
)

// PromiseHook observes promise lifecycle events. Init/Resolve fire
// synchronously at creation/resolution; Before/After bracket the reaction
// job at the microtask checkpoint. promise and parent are scope-local
// values valid only during the callback (parent is undefined for
// non-derived promises).
type PromiseHook func(t PromiseHookType, promise, parent Value)

// SetPromiseHook installs h on the isolate (replacing any previous hook —
// the engine keeps one slot). A nil hook is rejected; there is no unset in
// the pinned surface.
func (i *Isolate) SetPromiseHook(h PromiseHook) error {
	if h == nil {
		return errors.New("gov8: promise hook required")
	}
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	if err := ensureCHDispatcher(); err != nil {
		return err
	}
	replaceCHSlot(i, chKindPromiseHook, &chEntry{promiseHook: h})
	return callErr("SetPromiseHook", proc("gov8_ch_set_promise_hook"), ih)
}

// The promise-reject callback (PromiseRejectEvent, PromiseRejectMessage,
// SetPromiseRejectCallback) is owned by promise.go; the dispatch kinds below
// cover the rest of the hook surface.

// ---------------------------------------------------------------------------
// Prepare stack trace
// ---------------------------------------------------------------------------

// PrepareStackTraceCallback replaces the `stack` VALUE for every error
// whose stack is first accessed. It receives the error and the CallSite
// array; the returned Value becomes the stack value. The ok result remains
// for API compatibility, but false is invalid and fails fast: pinned V8
// asserts on an empty MaybeLocal, and rusty_v8 therefore requires a Local.
// Installing the hook
// disables the JS Error.prepareStackTrace hook entirely. The callback runs
// once per distinct error; the scope handed to it is owned by the engine
// trampoline and values created through it remain valid until the callback
// returns to the engine.
type PrepareStackTraceCallback func(s *Scope, errorValue, sites Value) (result Value, ok bool)

// SetPrepareStackTraceCallback installs cb on the isolate (one slot).
func (i *Isolate) SetPrepareStackTraceCallback(cb PrepareStackTraceCallback) error {
	if cb == nil {
		return errors.New("gov8: prepare stack trace callback required")
	}
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	if err := ensureCHDispatcher(); err != nil {
		return err
	}
	replaceCHSlot(i, chKindPrepareStackTrace, &chEntry{prepareStackTrace: cb})
	return callErr("SetPrepareStackTrace", proc("gov8_ch_set_prepare_stack_trace"), ih)
}

// ExceptionMessageText formats err through Exception::CreateMessage and
// Message::Get — the "Uncaught Error: ..." text the engine would produce.
// Useful inside a PrepareStackTraceCallback for the formatted message.
func ExceptionMessageText(s *Scope, errValue Value) (string, error) {
	if err := errValue.check(); err != nil {
		return "", err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return "", err
	}
	if errValue.iso != s.iso {
		return "", foreignIsolate("value")
	}
	return callTextFn("ExceptionMessageText", func(buf *byte, cap int, outLen *int64) uintptr {
		r, _, _ := proc("gov8_ch_exception_message_text_utf8").Call(
			s.iso.handleAssumingCheck(), sh, errValue.h,
			uintptr(unsafe.Pointer(buf)), uintptr(cap), uintptr(unsafe.Pointer(outLen)))
		return r
	})
}

// ArrayLength returns the length of a callback-delivered array value (e.g.
// the CallSite array of a PrepareStackTraceCallback).
func ArrayLength(s *Scope, v Value) (int, error) {
	if err := v.check(); err != nil {
		return 0, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return 0, err
	}
	if v.iso != s.iso {
		return 0, foreignIsolate("value")
	}
	r1, _, _ := proc("gov8_ch_array_length").Call(s.iso.handleAssumingCheck(), sh, v.h)
	if int64(r1) < 0 {
		return 0, shimError("ArrayLength", r1)
	}
	return int(r1), nil
}

// ---------------------------------------------------------------------------
// Use counter
// ---------------------------------------------------------------------------

// UseCounterCallback receives the engine's UseCounterFeature discriminant
// (a stable engine-assigned number; e.g. 9 = strict mode directive in the
// pinned build). It runs during compilation/execution on the isolate's
// thread and must not re-enter the engine.
type UseCounterCallback func(feature uint32)

// SetUseCounterCallback installs cb on the isolate (one slot).
func (i *Isolate) SetUseCounterCallback(cb UseCounterCallback) error {
	if cb == nil {
		return errors.New("gov8: use counter callback required")
	}
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	if err := ensureCHDispatcher(); err != nil {
		return err
	}
	replaceCHSlot(i, chKindUseCounter, &chEntry{useCounter: cb})
	return callErr("SetUseCounterCallback", proc("gov8_ch_set_use_counter"), ih)
}

// ---------------------------------------------------------------------------
// Modify code generation from strings
// ---------------------------------------------------------------------------

// ModifyCodeGenerationFromStringsCallback decides whether code generation
// from the given source value is allowed in a context that disallows it
// (or for a non-string eval source). Return allowed=false to block (the
// engine throws EvalError); allowed=true with modified=nil passes the
// source through unchanged; allowed=true with a non-nil rewritten source
// compiles the replacement string instead. The callback runs on the
// isolate's thread during eval/Function/new Function.
type ModifyCodeGenerationFromStringsCallback func(source Value, isCodeLike bool) (allowed bool, modified *string)

// SetModifyCodeGenerationFromStringsCallback installs cb on the isolate
// (one slot). The engine consults it ONLY when the context disallows code
// generation from strings or the eval source is not a string; plain evals
// in an allowed context skip it entirely.
func (i *Isolate) SetModifyCodeGenerationFromStringsCallback(cb ModifyCodeGenerationFromStringsCallback) error {
	if cb == nil {
		return errors.New("gov8: code generation callback required")
	}
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	if err := ensureCHDispatcher(); err != nil {
		return err
	}
	replaceCHSlot(i, chKindCodegen, &chEntry{codegen: cb})
	return callErr("SetModifyCodeGenerationFromStringsCallback",
		proc("gov8_ch_set_codegen_callback"), ih)
}

// AllowCodeGenerationFromStrings toggles codegen-from-strings for the
// context (Context::AllowCodeGenerationFromStrings).
func (c *Context) AllowCodeGenerationFromStrings(allow bool) error {
	if err := c.check(); err != nil {
		return err
	}
	a := uintptr(0)
	if allow {
		a = 1
	}
	return callErr("AllowCodeGenerationFromStrings",
		proc("gov8_ch_context_allow_codegen_from_strings"), c.handle, a)
}

// IsCodeGenerationFromStringsAllowed reports the context's current
// codegen-from-strings setting (allowed by default).
func (c *Context) IsCodeGenerationFromStringsAllowed() (bool, error) {
	if err := c.check(); err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_ch_context_codegen_allowed").Call(c.handle)
	if int64(r1) < 0 {
		return false, shimError("IsCodeGenerationFromStringsAllowed", r1)
	}
	return r1 == 1, nil
}

// ---------------------------------------------------------------------------
// Message listeners
// ---------------------------------------------------------------------------

// RunUncaught runs the script with NO TryCatch active, so an exception
// escapes to the isolate's message listeners — the pinned oracle's
// `script.run` shape (the default Run installs an internal fallback
// TryCatch, which would swallow the message). The completion value is a
// scope-local Value; a runtime exception returns the error.
func (sc *Script) RunUncaught(s *Scope) (Value, error) {
	if err := sc.check(); err != nil {
		return Value{}, err
	}
	if s.iso != sc.iso {
		return Value{}, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_ch_script_run_uncaught").Call(
		sc.iso.handleAssumingCheck(), sc.ctx.handle, sh, sc.handle,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("RunUncaught", r1)
	}
	return Value{iso: sc.iso, sc: s, h: out}, nil
}

// MessageErrorLevel bits mirror v8::Isolate::MessageErrorLevel.
const (
	MsgLog     uint32 = 1 << 0
	MsgDebug   uint32 = 1 << 1
	MsgInfo    uint32 = 1 << 2
	MsgError   uint32 = 1 << 3
	MsgWarning uint32 = 1 << 4
	MsgAll     uint32 = MsgLog | MsgDebug | MsgInfo | MsgError | MsgWarning
)

// MessageListenerCallback observes a message produced by an exception that
// escaped every TryCatch (uncaught only — TryCatch-caused exceptions are
// never reported). msg is valid only during the callback; exception is the
// thrown value (scope-local, same lifetime). The same listener registered
// twice is called twice per message.
type MessageListenerCallback func(msg *CallbackMessage, exception Value)

// CallbackMessage is a message delivered to a MessageListenerCallback. It
// is bound to the dispatch scope and the engine's entered-or-microtask
// context captured at dispatch time.
type CallbackMessage struct {
	iso     *Isolate
	sc      *Scope
	ctxWire uintptr
	h       uintptr
}

// Text returns the Message::Get text (carries the "Uncaught " prefix).
func (m *CallbackMessage) Text() (string, error) {
	if err := m.check(); err != nil {
		return "", err
	}
	return callTextFn("CallbackMessage.Text", func(buf *byte, cap int, outLen *int64) uintptr {
		r, _, _ := proc("gov8_ch_message_text_utf8").Call(
			m.iso.handleAssumingCheck(), m.sc.handle, m.h,
			uintptr(unsafe.Pointer(buf)), uintptr(cap), uintptr(unsafe.Pointer(outLen)))
		return r
	})
}

// LineNumber returns the 1-based line of the error; ok=false when absent.
func (m *CallbackMessage) LineNumber() (line int32, ok bool, err error) {
	if err := m.check(); err != nil {
		return 0, false, err
	}
	var out, okv int32
	r1, _, _ := proc("gov8_ch_message_line_number").Call(
		m.iso.handleAssumingCheck(), m.ctxWire, m.sc.handle, m.h,
		uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&okv)))
	if int64(r1) < 0 {
		return 0, false, shimError("CallbackMessage.LineNumber", r1)
	}
	return out, okv == 1, nil
}

// ErrorLevel returns the MessageErrorLevel bits of the message.
func (m *CallbackMessage) ErrorLevel() (int64, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	r1, _, _ := proc("gov8_ch_message_error_level").Call(
		m.iso.handleAssumingCheck(), m.sc.handle, m.h)
	if int64(r1) < 0 {
		return 0, shimError("CallbackMessage.ErrorLevel", r1)
	}
	return int64(r1), nil
}

// ValueText converts the exception delivered alongside this message to its
// ECMAScript ToString text, using the engine context captured by the
// dispatch (the entered-or-microtask context the engine had when the
// listener fired).
func (m *CallbackMessage) ValueText(exception Value) (string, error) {
	if err := m.check(); err != nil {
		return "", err
	}
	if err := exception.check(); err != nil {
		return "", err
	}
	if exception.iso != m.iso {
		return "", foreignIsolate("value")
	}
	return callTextFn("CallbackMessage.ValueText", func(buf *byte, cap int, outLen *int64) uintptr {
		r, _, _ := proc("gov8_ch_value_text_utf8").Call(
			m.iso.handleAssumingCheck(), m.ctxWire, m.sc.handle, exception.h,
			uintptr(unsafe.Pointer(buf)), uintptr(cap), uintptr(unsafe.Pointer(outLen)))
		return r
	})
}

// StartPosition returns the 0-based character offset where the error
// region starts.
func (m *CallbackMessage) StartPosition() (int64, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	r1, _, _ := proc("gov8_ch_message_start_position").Call(
		m.iso.handleAssumingCheck(), m.sc.handle, m.h)
	if int64(r1) < 0 {
		return 0, shimError("CallbackMessage.StartPosition", r1)
	}
	return int64(r1), nil
}

// EndPosition returns the exclusive end offset of the error region.
func (m *CallbackMessage) EndPosition() (int64, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	r1, _, _ := proc("gov8_ch_message_end_position").Call(
		m.iso.handleAssumingCheck(), m.sc.handle, m.h)
	if int64(r1) < 0 {
		return 0, shimError("CallbackMessage.EndPosition", r1)
	}
	return int64(r1), nil
}

func (m *CallbackMessage) check() error {
	if m == nil || m.h == 0 {
		return fmt.Errorf("gov8: nil or empty callback message")
	}
	return m.sc.check()
}

// AddMessageListener registers cb for ERROR-level messages. Listeners are
// append-only (no removal API in the pinned surface); the same listener
// registered twice is delivered twice.
func (i *Isolate) AddMessageListener(cb MessageListenerCallback) (bool, error) {
	if cb == nil {
		return false, errors.New("gov8: message listener required")
	}
	if _, err := i.handleChecked(); err != nil {
		return false, err
	}
	if err := ensureCHDispatcher(); err != nil {
		return false, err
	}
	ok, err := registerCHListener(i, cb, MsgError)
	if err != nil {
		return false, err
	}
	return ok, nil
}

// AddMessageListenerWithErrorLevel registers cb filtered to the given
// MessageErrorLevel bits; it only observes messages whose level matches
// (e.g. a WARNING-filtered listener never sees ERROR-level throws).
func (i *Isolate) AddMessageListenerWithErrorLevel(cb MessageListenerCallback, errorLevel uint32) (bool, error) {
	if cb == nil {
		return false, errors.New("gov8: message listener required")
	}
	if errorLevel == 0 || errorLevel > MsgAll {
		return false, fmt.Errorf("gov8: invalid message error level %#x", errorLevel)
	}
	if _, err := i.handleChecked(); err != nil {
		return false, err
	}
	if err := ensureCHDispatcher(); err != nil {
		return false, err
	}
	ok, err := registerCHListener(i, cb, errorLevel)
	if err != nil {
		return false, err
	}
	return ok, nil
}

// ---------------------------------------------------------------------------
// OOM and near-heap-limit handlers
// ---------------------------------------------------------------------------

// OOMErrorCallback observes a fatal out-of-memory event: location names the
// site ("Reached heap limit" for heap OOM), detail is the engine's detail
// string (empty on the heap-OOM path in this build), isHeapOOM distinguishes
// heap OOM from other OOMs. When the handler returns, the engine aborts the
// process; the handler only observes.
type OOMErrorCallback func(location, detail string, isHeapOOM bool)

// SetOOMErrorHandler installs cb on the isolate (one slot). Pair it with a
// heap-limited isolate (NewIsolateWithLimits) so OOM paths stay bounded.
func (i *Isolate) SetOOMErrorHandler(cb OOMErrorCallback) error {
	if cb == nil {
		return errors.New("gov8: OOM error handler required")
	}
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	if err := ensureCHDispatcher(); err != nil {
		return err
	}
	replaceCHSlot(i, chKindOOM, &chEntry{oom: cb})
	return callErr("SetOOMErrorHandler", proc("gov8_ch_set_oom_error_handler"), ih)
}

// NearHeapLimitCallback is consulted when the heap approaches its limit.
// It receives the current and initial heap limits (bytes) and returns the
// new limit to install — raising it grants more budget (return current*2 to
// double), shrinking it forces the intended, controlled fatal OOM. The
// engine keeps ONE slot: only the most recently added callback is invoked.
// The callback runs on the isolate's thread inside GC and must not
// re-enter the engine.
type NearHeapLimitCallback func(currentHeapLimit, initialHeapLimit uint64) uint64

// AddNearHeapLimitCallback installs cb (replacing any previous callback —
// the engine invokes only the most recently added callback; the replaced
// registration never fires again).
func (i *Isolate) AddNearHeapLimitCallback(cb NearHeapLimitCallback) error {
	if cb == nil {
		return errors.New("gov8: near-heap-limit callback required")
	}
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	if err := ensureCHDispatcher(); err != nil {
		return err
	}
	chRegistry.mu.Lock()
	existing := chRegistry.entries[chKey{kind: chKindNearHeapLimit, engine: ih}]
	// A previous registration only counts when it belongs to THIS live Go
	// isolate: engine isolate addresses are recycled, so an entry left over
	// from a closed isolate at the same address must be treated as absent
	// (its engine slot died with the isolate).
	hadPrevious := existing != nil && existing.iso == i && !i.closed
	chRegistry.entries[chKey{kind: chKindNearHeapLimit, engine: ih}] = &chEntry{iso: i, nearHeapLimit: cb}
	chRegistry.mu.Unlock()
	had := uintptr(0)
	if hadPrevious {
		had = 1
	}
	return callErr("AddNearHeapLimitCallback", proc("gov8_ch_add_near_heap_limit"), ih, had)
}

// RemoveNearHeapLimitCallback removes the active callback and restores the
// given heap limit (RemoveNearHeapLimitCallback's heap_limit). A stale
// registration left over from a closed isolate at the same engine address
// is treated as absent (calling the engine's remove in that case would hit
// its UNREACHABLE guard).
func (i *Isolate) RemoveNearHeapLimitCallback(heapLimit uint64) error {
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	chRegistry.mu.Lock()
	existing := chRegistry.entries[chKey{kind: chKindNearHeapLimit, engine: ih}]
	registered := existing != nil && existing.iso == i && !i.closed
	delete(chRegistry.entries, chKey{kind: chKindNearHeapLimit, engine: ih})
	chRegistry.mu.Unlock()
	if !registered {
		return nil
	}
	return callErr("RemoveNearHeapLimitCallback", proc("gov8_ch_remove_near_heap_limit"),
		ih, uintptr(heapLimit))
}
