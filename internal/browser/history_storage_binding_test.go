//go:build (windows || linux) && amd64

package browser

import "testing"

func TestHistoryStorageBindingsMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "history_storage_binding")
}
