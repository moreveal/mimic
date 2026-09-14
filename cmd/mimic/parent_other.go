//go:build !windows

package main

// Unix-like systems already signal or re-parent foreground children according
// to their launcher and terminal lifecycle. Preserve that established behavior.
func parentExitSignal() <-chan struct{} { return nil }
