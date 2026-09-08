//go:build windows && amd64

package v8

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type threadPowerPolicy struct{ Version, Control, State uint32 }

var threadInformation = windows.NewLazySystemDLL("kernel32.dll")
var getThreadPolicy = threadInformation.NewProc("GetThreadInformation")
var setThreadPolicy = threadInformation.NewProc("SetThreadInformation")

func readThreadPowerPolicy() (threadPowerPolicy, error) {
	policy := threadPowerPolicy{Version: 1}
	if err := getThreadPolicy.Find(); err != nil {
		return policy, err
	}
	ok, _, err := getThreadPolicy.Call(uintptr(windows.CurrentThread()), 3, uintptr(unsafe.Pointer(&policy)), unsafe.Sizeof(policy))
	if ok == 0 {
		return policy, err
	}
	return policy, nil
}
func writeThreadPowerPolicy(policy threadPowerPolicy) error {
	if err := setThreadPolicy.Find(); err != nil {
		return err
	}
	ok, _, err := setThreadPolicy.Call(uintptr(windows.CurrentThread()), 3, uintptr(unsafe.Pointer(&policy)), unsafe.Sizeof(policy))
	if ok == 0 {
		return err
	}
	return nil
}

// A Page owner executes synchronous, latency-sensitive JavaScript. Leaving its
// execution-speed QoS unmanaged lets Windows demote long-running background
// browser processes to efficiency cores even with an active client waiting.
// This is a scheduling hint, not an affinity mask or priority increase. Honor
// an explicit policy inherited from the caller, and restore before V8 unlocks
// the OS thread so a later Go goroutine does not inherit our hint.
func configurePageThreadPolicy() func() error {
	previous, err := readThreadPowerPolicy()
	if err != nil || previous.Control&1 != 0 {
		return nil
	}
	active := previous
	active.Control |= 1
	active.State &^= 1
	if err = writeThreadPowerPolicy(active); err != nil {
		return nil
	}
	return func() error { return writeThreadPowerPolicy(previous) }
}
