//go:build linux && amd64

package v8

import (
	"fmt"
	"golang.org/x/sys/unix"
	"unsafe"
)

func currentThreadID() uint32 { return uint32(unix.Gettid()) }

// Linux inherits the caller's scheduling policy; the Windows power hint has no equivalent.
func configurePageThreadPolicy() func() error { return nil }

type processorProbe struct{}

var diagnosticProcessor processorProbe

func (processorProbe) Call() (uintptr, uintptr, error) {
	return linuxProcessor()
}
func linuxProcessor() (uintptr, uintptr, error) {
	var cpu uint32
	_, _, err := unix.RawSyscall(unix.SYS_GETCPU, uintptr(unsafe.Pointer(&cpu)), 0, 0)
	if err != 0 {
		return 0, 0, err
	}
	return uintptr(cpu), 0, nil
}

func diagnosticThreadCPU(tid uint32) map[string]any {
	result := map[string]any{"thread_id": tid}
	if cpu, _, err := linuxProcessor(); err == nil {
		result["processor"] = cpu
	}
	var usage unix.Rusage
	if unix.Getrusage(unix.RUSAGE_THREAD, &usage) == nil {
		result["kernel_100ns"] = uint64(usage.Stime.Nano() / 100)
		result["user_100ns"] = uint64(usage.Utime.Nano() / 100)
	}
	return result
}
func (a *adapter) ProfileActiveQoS() (func(), error) {
	return nil, fmt.Errorf("Windows thread power QoS is unavailable on Linux")
}
