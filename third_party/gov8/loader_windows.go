//go:build windows && amd64

package gov8

import (
	"fmt"
	"syscall"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var loadLibraryExW = kernel32.NewProc("LoadLibraryExW")

func loadShimDLL(path string) (*syscall.DLL, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	const (
		loadLibrarySearchDLLLoadDir = 0x00000100
		loadLibrarySearchSystem32   = 0x00000800
	)
	handle, _, callErr := loadLibraryExW.Call(
		uintptr(unsafe.Pointer(name)),
		0,
		loadLibrarySearchDLLLoadDir|loadLibrarySearchSystem32,
	)
	if handle == 0 {
		return nil, fmt.Errorf("LoadLibraryExW: %w", callErr)
	}
	return &syscall.DLL{Name: path, Handle: syscall.Handle(handle)}, nil
}
