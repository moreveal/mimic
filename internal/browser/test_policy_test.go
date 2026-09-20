package browser

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Twelve slots are the measured throughput knee on the Windows reference host:
// 24 heavyweight V8 tests took 28.25s at 8, 19.15s at 12, and 18.83s at 16.
// Keep the default below the flat part of the curve to limit peak isolate memory.
const defaultBrowserTestParallelism = 12

var (
	browserTestSlotsOnce sync.Once
	browserTestSlots     chan struct{}
)

// parallelBrowserTest marks a root test as safe to run with other independent
// Browser/Context/Page tests. The extra semaphore keeps native V8 isolates and
// their bootstrap memory bounded independently of testing's -parallel flag.
func parallelBrowserTest(t *testing.T) {
	t.Helper()
	parallelBrowserTestWithSlots(t, browserParallelSlots())
}

func parallelBrowserTestWithSlots(t *testing.T, slots chan struct{}) {
	t.Helper()
	t.Parallel()
	slots <- struct{}{}
	t.Cleanup(func() { <-slots })
}

// serialBrowserTest is an explicit policy marker. Tests which mutate process
// state, exercise global caches, or validate concurrency/lifecycle behavior
// stay in testing's serial lane.
func serialBrowserTest(t *testing.T) {
	t.Helper()
}

func browserParallelSlots() chan struct{} {
	browserTestSlotsOnce.Do(func() {
		browserTestSlots = make(chan struct{}, browserTestParallelism(os.Getenv("MIMIC_TEST_PARALLELISM"), runtime.GOMAXPROCS(0)))
	})
	return browserTestSlots
}

func browserTestParallelism(value string, gomaxprocs int) int {
	if value != "" {
		parallelism, err := strconv.Atoi(value)
		if err != nil || parallelism < 1 {
			panic(fmt.Sprintf("MIMIC_TEST_PARALLELISM must be a positive integer, got %q", value))
		}
		return parallelism
	}
	if gomaxprocs < 1 {
		return 1
	}
	if gomaxprocs < defaultBrowserTestParallelism {
		return gomaxprocs
	}
	return defaultBrowserTestParallelism
}

func TestBrowserTestParallelism(t *testing.T) {
	serialBrowserTest(t)
	for _, test := range []struct {
		value       string
		gomaxprocs  int
		parallelism int
	}{
		{"", 1, 1},
		{"", 2, 2},
		{"", 8, 8},
		{"", 16, defaultBrowserTestParallelism},
		{"1", 16, 1},
		{"7", 2, 7},
	} {
		if actual := browserTestParallelism(test.value, test.gomaxprocs); actual != test.parallelism {
			t.Errorf("browserTestParallelism(%q, %d) = %d, want %d", test.value, test.gomaxprocs, actual, test.parallelism)
		}
	}
}

func TestBrowserTestParallelismRejectsInvalidOverride(t *testing.T) {
	serialBrowserTest(t)
	for _, value := range []string{"0", "-1", "many"} {
		t.Run(value, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
			}()
			browserTestParallelism(value, 4)
		})
	}
}

func TestParallelBrowserTestBoundsConcurrencyAndReleasesSlots(t *testing.T) {
	serialBrowserTest(t)
	slots := make(chan struct{}, 2)
	var active atomic.Int32
	var maximum atomic.Int32
	for index := 0; index < 6; index++ {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			parallelBrowserTestWithSlots(t, slots)
			current := active.Add(1)
			defer active.Add(-1)
			for observed := maximum.Load(); current > observed && !maximum.CompareAndSwap(observed, current); observed = maximum.Load() {
			}
			time.Sleep(10 * time.Millisecond)
		})
	}
	t.Cleanup(func() {
		if maximum.Load() != 2 {
			t.Errorf("maximum active tests = %d, want 2", maximum.Load())
		}
		if len(slots) != 0 {
			t.Errorf("retained slots = %d, want 0", len(slots))
		}
	})
}
