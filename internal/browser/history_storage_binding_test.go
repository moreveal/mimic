//go:build (windows || linux) && amd64

package browser

import "testing"

func TestHistoryStorageBindingsMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "history_storage_binding")
}
