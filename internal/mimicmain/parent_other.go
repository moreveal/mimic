//go:build !windows

package mimicmain

func parentExitSignal() <-chan struct{} { return nil }
