//go:build windows && amd64

package prebuilt

import (
	"os"

	"golang.org/x/sys/windows"
)

func lockCache(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	var overlapped windows.Overlapped
	if err = windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &overlapped); err != nil {
		file.Close()
		return nil, err
	}
	return func() { windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped); file.Close() }, nil
}
