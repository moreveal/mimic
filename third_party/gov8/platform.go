//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"math"
	"unsafe"
)

// PlatformKind selects one of the default platform implementations exposed by
// the pinned rusty_v8 release. Custom PlatformImpl implementations are a
// separate API family and are not represented here.
type PlatformKind uint8

const (
	PlatformDefault PlatformKind = iota
	PlatformUnprotected
	PlatformSingleThreaded
)

// PlatformOptions configures the process-global platform installed by
// Initialize. ThreadPoolSize is clamped to 16, exactly like rusty_v8; zero asks
// V8 to choose a worker count. It is ignored for PlatformSingleThreaded and
// must be zero there.
//
// SingleThreadedFlag is an explicit safety acknowledgement required for
// PlatformSingleThreaded. ConfigurePlatform applies V8's --single-threaded
// flag when it is true. The pinned native API accepts a single-threaded
// platform without that flag, but later asynchronous Wasm compilation access
// violates; Go rejects that configuration before engine entry instead.
type PlatformOptions struct {
	Kind               PlatformKind
	ThreadPoolSize     uint32
	IdleTaskSupport    bool
	SingleThreadedFlag bool
}

// ErrSingleThreadedPlatformFlagRequired is returned instead of admitting the
// pinned engine's fatal single-threaded-platform-without-flag configuration.
var ErrSingleThreadedPlatformFlagRequired = errors.New("gov8: single-threaded platform requires the --single-threaded V8 flag")

type platformConfiguration struct {
	kind           PlatformKind
	threadPoolSize uint32
	idleTasks      bool
}

var selectedPlatform *platformConfiguration

// ConfigurePlatform selects the platform that the next Initialize call will
// install. It is process-global, may be called exactly once, and must run
// before Initialize. If it is never called, Initialize retains its historical
// behavior: default platform, automatic worker count, idle tasks disabled.
func ConfigurePlatform(options PlatformOptions) error {
	if err := loadShim(); err != nil {
		return err
	}
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	if loadPlatform() != stateUninitialized {
		return fmt.Errorf("gov8: ConfigurePlatform must be called before Initialize")
	}
	if selectedPlatform != nil || selectedCustomPlatform != nil {
		return fmt.Errorf("gov8: platform already configured")
	}
	if options.Kind > PlatformSingleThreaded {
		return fmt.Errorf("gov8: unknown platform kind %d", options.Kind)
	}
	if options.Kind == PlatformSingleThreaded {
		if options.ThreadPoolSize != 0 {
			return fmt.Errorf("gov8: single-threaded platform does not accept a thread pool size")
		}
		if !options.SingleThreadedFlag {
			return ErrSingleThreadedPlatformFlagRequired
		}
		// Apply the required flag while initialization is still impossible under
		// lifecycleMu. This closes the fatal native configuration out of the safe
		// Go API rather than relying on callers to order a separate flags call.
		flags := []byte("--single-threaded")
		r1, _, _ := proc("gov8_ch_set_flags_from_string").Call(
			uintptr(unsafe.Pointer(&flags[0])), uintptr(len(flags)))
		if int64(r1) < 0 {
			return shimError("ConfigurePlatform", r1)
		}
	} else if options.SingleThreadedFlag {
		return fmt.Errorf("gov8: SingleThreadedFlag is only valid for PlatformSingleThreaded")
	}
	size := options.ThreadPoolSize
	if size > 16 {
		size = 16
	}
	selectedPlatform = &platformConfiguration{
		kind:           options.Kind,
		threadPoolSize: size,
		idleTasks:      options.IdleTaskSupport,
	}
	return nil
}

// initializeSelectedPlatform is called by Initialize with lifecycleMu held.
func initializeSelectedPlatform() error {
	if configured, err := initializeCustomPlatformIfConfigured(); configured {
		return err
	}
	if selectedPlatform == nil {
		return callErr("Initialize", proc("gov8_initialize_platform"))
	}
	var idle uintptr
	if selectedPlatform.idleTasks {
		idle = 1
	}
	return callErr("Initialize", proc("gov8_platform_initialize_configured"),
		uintptr(selectedPlatform.kind), uintptr(selectedPlatform.threadPoolSize), idle)
}

// RunIdleTasks runs pending idle tasks for at most idleTimeInSeconds. As in
// rusty_v8, all finite and non-finite float64 values are forwarded unchanged;
// the default platform treats negative/NaN/infinite boundaries safely.
func (i *Isolate) RunIdleTasks(idleTimeInSeconds float64) error {
	if i == nil {
		return errors.New("gov8: nil isolate")
	}
	if err := i.check(); err != nil {
		return err
	}
	r1, _, _ := proc("gov8_platform_run_idle_tasks").Call(
		i.handleAssumingCheck(), uintptr(math.Float64bits(idleTimeInSeconds)))
	if int64(r1) < 0 {
		return shimError("Isolate.RunIdleTasks", r1)
	}
	return nil
}
