//go:build windows && amd64

package v8

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

var diagnosticProcessor = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCurrentProcessorNumber")

var diagnosticThreadTimes = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetThreadTimes")

func diagnosticThreadCPU(tid uint32) map[string]any {
	processor, _, _ := diagnosticProcessor.Call()
	var created, exited, kernel, user windows.Filetime
	threadOK, _, _ := diagnosticThreadTimes.Call(uintptr(windows.CurrentThread()), uintptr(unsafe.Pointer(&created)), uintptr(unsafe.Pointer(&exited)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	threadCPU := map[string]any{"processor": processor, "thread_id": tid}
	if threadOK != 0 {
		threadCPU["kernel_100ns"] = uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)
		threadCPU["user_100ns"] = uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime)
	}
	return threadCPU
}

// ProfileActiveQoS is an opt-in OS scheduling experiment, not benchmark policy.
func (a *adapter) ProfileActiveQoS() (func(), error) {
	get := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetThreadInformation")
	set := windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadInformation")
	var previous struct{ Version, Control, State uint32 }
	previous.Version = 1
	_, err := a.owner.execute(func(*state) response {
		ok, _, e := get.Call(uintptr(windows.CurrentThread()), 3, uintptr(unsafe.Pointer(&previous)), unsafe.Sizeof(previous))
		if ok == 0 {
			return response{err: e}
		}
		active := previous
		active.Control |= 1
		active.State &^= 1
		ok, _, e = set.Call(uintptr(windows.CurrentThread()), 3, uintptr(unsafe.Pointer(&active)), unsafe.Sizeof(active))
		if ok == 0 {
			return response{err: e}
		}
		return response{}
	})
	if err != nil {
		return nil, err
	}
	return func() {
		_, _ = a.owner.execute(func(*state) response {
			set.Call(uintptr(windows.CurrentThread()), 3, uintptr(unsafe.Pointer(&previous)), unsafe.Sizeof(previous))
			return response{}
		})
	}, nil
}
