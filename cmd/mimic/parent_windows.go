//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// parentExitSignal ties the foreground server to the process which launched it.
// Windows does not deliver a signal when that process exits, so a server started
// by a short-lived automation runner would otherwise keep its port and browser
// state forever. Holding a process handle also avoids PID-reuse races.
func parentExitSignal() <-chan struct{} {
	return processExitSignal(uint32(os.Getppid()))
}

func processExitSignal(pid uint32) <-chan struct{} {
	done := make(chan struct{})
	parent, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if err != nil {
		if err == windows.ERROR_INVALID_PARAMETER {
			close(done)
			return done
		}
		return nil
	}
	go func() {
		_, _ = windows.WaitForSingleObject(parent, windows.INFINITE)
		_ = windows.CloseHandle(parent)
		close(done)
	}()
	return done
}
