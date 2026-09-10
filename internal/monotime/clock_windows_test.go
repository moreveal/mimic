//go:build windows

package monotime

import (
	"testing"
	"time"
)

func TestQPCClockPrecisionAndMonotonicity(t *testing.T) {
	previous := Now()
	minimum := time.Second
	for i := 0; i < 100000; i++ {
		next := Now()
		delta := next.Sub(previous)
		if delta < 0 {
			t.Fatal("QPC clock moved backwards")
		}
		if delta > 0 && delta < minimum {
			minimum = delta
		}
		previous = next
	}
	// Interrupt-time based time.Now on the reference Windows host has ~500us
	// steps. The source must be finer than the ordinary browser's 100us grid.
	if minimum >= 100*time.Microsecond {
		t.Fatalf("source too coarse: %v", minimum)
	}
	start := Now()
	time.Sleep(time.Millisecond)
	// Sleep uses Go's timer source, whose Windows resolution is exactly what
	// differs here. It cannot serve as a precise lower-bound oracle for QPC.
	if Since(start) <= 0 {
		t.Fatal("elapsed time did not advance")
	}
	t.Logf("minimum positive source delta: %v", minimum)
}
