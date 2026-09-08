//go:build windows && amd64

package v8

import "syscall"

var getCurrentThreadID = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")

func currentThreadID() uint32 {
	id, _, _ := getCurrentThreadID.Call()
	return uint32(id)
}

