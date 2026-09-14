//go:build linux && amd64

package v8

/*
#include <malloc.h>

// mallopt must run before Go or V8 causes glibc to create worker-thread arenas.
// Calling it from normal Go initialization is already too late on Linux.
__attribute__((constructor)) static void mimic_configure_malloc_arenas(void) {
	mallopt(M_ARENA_MAX, 4);
}
*/
import "C"

import (
	"sync"
	"time"
)

var (
	nativeHeapTrimOnce sync.Once
	nativeHeapTrimWake = make(chan struct{}, 1)
)

// requestNativeHeapTrim coalesces concurrent isolate teardown into one glibc
// arena trim. V8 has already disposed the isolate and released its pages at
// this point. Without the trim, Linux keeps those free native pages resident
// across Page waves, so a long-lived process approaches twice its live Page
// footprint after a large batch is closed.
func requestNativeHeapTrim() {
	nativeHeapTrimOnce.Do(func() { go nativeHeapTrimLoop() })
	select {
	case nativeHeapTrimWake <- struct{}{}:
	default:
	}
}

func nativeHeapTrimLoop() {
	for range nativeHeapTrimWake {
		// A Page batch closes isolates concurrently. Wait for the burst to end
		// so allocator maintenance runs once rather than once per Page.
		timer := time.NewTimer(10 * time.Millisecond)
	settle:
		for {
			select {
			case <-nativeHeapTrimWake:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(10 * time.Millisecond)
			case <-timer.C:
				break settle
			}
		}
		C.malloc_trim(0)
	}
}
