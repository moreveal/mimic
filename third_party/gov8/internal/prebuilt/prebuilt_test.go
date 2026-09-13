//go:build (windows || linux) && amd64

package prebuilt

import (
	"os"
	"sync"
	"testing"
)

func TestMaterializeRepairsCorruptionAndSharesVerifiedAsset(t *testing.T) {
	root := t.TempDir()
	path, err := Materialize(root)
	if err != nil {
		t.Fatal(err)
	}
	if err = Verify(path); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("truncated native library"), 0600); err != nil {
		t.Fatal(err)
	}
	if Verify(path) == nil {
		t.Fatal("accepted corrupt library")
	}
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			got, err := Materialize(root)
			if err != nil {
				t.Error(err)
				return
			}
			if got != path {
				t.Errorf("cache identity changed: %s != %s", got, path)
			}
			if err = Verify(got); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
}
