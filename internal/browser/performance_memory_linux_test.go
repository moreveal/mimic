package browser

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func init() { profileProcessMemory = processMemory }

// Linux private resident memory is not Windows private commit. Diagnostic
// consumers must retain the accounting label when comparing OS measurements.
func processMemory(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile("/proc/self/smaps_rollup")
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[2] != "kB" {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		values[strings.TrimSuffix(fields[0], ":")] = value * 1024
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	return map[string]any{"rss": values["Rss"], "private": values["Private_Clean"] + values["Private_Dirty"] + values["Private_Hugetlb"], "private_accounting": "resident", "go_heap": memory.HeapAlloc, "go_sys": memory.Sys, "go_released": memory.HeapReleased, "goroutines": runtime.NumGoroutine()}
}
