//go:build linux && amd64

package gov8

import "syscall"

func currentThreadID() uint32 { return uint32(syscall.Gettid()) }
