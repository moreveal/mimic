//go:build windows && amd64

package gov8

import (
	"sync"
	"syscall"
)

// currentThreadID returns the Win32 thread id used for isolate affinity
// checks. Windows keeps the ID in the amd64 TEB, so the steady-state path reads
// it directly instead of crossing the syscall trampoline for every wrapper
// validation. The first call verifies that layout against GetCurrentThreadId;
// an unexpected platform layout falls back to the API without weakening the
// affinity check. The leaf assembly reads the ID of the thread executing it;
// isolate operations retain their existing LockOSThread ownership invariant.
func currentThreadID() uint32 {
	tidOnce.Do(resolveThreadIDProc)
	if tidFast {
		return currentThreadIDFast()
	}
	r1, _, _ := syscall.SyscallN(tidProcAddr)
	return uint32(r1)
}

// currentThreadIDFast is implemented in thread_id_windows_amd64.s.
//
//go:noescape
func currentThreadIDFast() uint32

var (
	tidOnce     sync.Once
	tidProcAddr uintptr
	tidFast     bool
)

func resolveThreadIDProc() {
	dll, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		panic("gov8: loading kernel32.dll: " + err.Error())
	}
	p, err := dll.FindProc("GetCurrentThreadId")
	if err != nil {
		panic("gov8: kernel32.dll export GetCurrentThreadId missing: " + err.Error())
	}
	tidProcAddr = p.Addr()
	win32ID, _, _ := syscall.SyscallN(tidProcAddr)
	fastID := currentThreadIDFast()
	tidFast = fastID != 0 && fastID == uint32(win32ID)
}
