package browser

import "testing"

// These table-driven oracles use a fresh Context, Page and HTTP server for
// every engine and case. Bound their aggregate concurrency independently of
// GOMAXPROCS: native isolates and Goja bootstraps have substantial peak memory.
// Do not use this helper for tests changing process environment or globals.
var oracleSlots = make(chan struct{}, 2)

func parallelOracle(t *testing.T) {
	t.Helper()
	t.Parallel()
	oracleSlots <- struct{}{}
	// Registered before case resources so the slot is released after teardown.
	t.Cleanup(func() { <-oracleSlots })
}
