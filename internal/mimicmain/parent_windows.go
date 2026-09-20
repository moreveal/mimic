//go:build windows

package mimicmain

import (
	"os"

	"golang.org/x/sys/windows"
)

func parentExitSignal() <-chan struct{} {
	done := make(chan struct{})
	parent, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(os.Getppid()))
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
