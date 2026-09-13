//go:build windows && amd64

// Package native supplies pointer-word calls using the host C ABI.
package native

import "syscall"

type Proc = syscall.Proc
type DLL = syscall.DLL
type Errno = syscall.Errno

var NewCallback = syscall.NewCallback

//go:uintptrescapes
func Syscall(trap, nargs, a1, a2, a3 uintptr) (uintptr, uintptr, Errno) {
	return syscall.Syscall(trap, nargs, a1, a2, a3)
}

//go:uintptrescapes
func Syscall6(trap, nargs, a1, a2, a3, a4, a5, a6 uintptr) (uintptr, uintptr, Errno) {
	return syscall.Syscall6(trap, nargs, a1, a2, a3, a4, a5, a6)
}

//go:uintptrescapes
func Syscall9(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9 uintptr) (uintptr, uintptr, Errno) {
	return syscall.Syscall9(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9)
}

//go:uintptrescapes
func Syscall12(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12 uintptr) (uintptr, uintptr, Errno) {
	return syscall.Syscall12(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12)
}

//go:uintptrescapes
func Syscall15(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15 uintptr) (uintptr, uintptr, Errno) {
	return syscall.Syscall15(trap, nargs, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, a13, a14, a15)
}

//go:uintptrescapes
func SyscallN(trap uintptr, args ...uintptr) (uintptr, uintptr, Errno) {
	return syscall.SyscallN(trap, args...)
}
