package browser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/engine"
)

type diskTestFactory struct{}

func (diskTestFactory) New() engine.Runtime                        { return nil }
func (diskTestFactory) BootstrapSnapshotsEnabled() bool            { return true }
func (diskTestFactory) BootstrapSnapshotIdentity() (string, error) { return "test-engine", nil }
func (diskTestFactory) BuildBootstrapSnapshot(context.Context, ...string) (engine.BootstrapSnapshot, error) {
	return &diskTestSnapshot{data: []byte("built")}, nil
}
func (diskTestFactory) LoadBootstrapSnapshot(data []byte) (engine.BootstrapSnapshot, error) {
	return &diskTestSnapshot{data: append([]byte(nil), data...)}, nil
}

type diskTestSnapshot struct{ data []byte }

func (*diskTestSnapshot) NewRuntime() (engine.Runtime, error) { return nil, nil }
func (*diskTestSnapshot) Close() error                        { return nil }
func (s *diskTestSnapshot) SizeBytes() int                    { return len(s.data) }
func (s *diskTestSnapshot) BootstrapSnapshotBytes() []byte    { return append([]byte(nil), s.data...) }

func TestBootstrapDiskRoundTripRejectsCorruptionAndCleansOwnedFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := newBootstrapDiskStore(dir, diskTestFactory{})
	if err != nil {
		t.Fatal(err)
	}
	key := [32]byte{1, 2, 3}
	want := &diskTestSnapshot{data: []byte("immutable snapshot")}
	if err := store.save(key, want); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.load(key)
	if err != nil || string(loaded.(*diskTestSnapshot).data) != string(want.data) {
		t.Fatalf("round trip: %v %v", loaded, err)
	}
	path := store.path(key)
	data, _ := os.ReadFile(path)
	data[len(data)-1] ^= 0xff
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if loaded, err = store.load(key); loaded != nil || err == nil {
		t.Fatal("corrupt artifact was admitted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("corrupt artifact was not removed")
	}
	foreign := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(foreign, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dir, bootstrapDiskSchema+"-stale.blob")
	if err := os.WriteFile(stale, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-bootstrapDiskMaxAge - time.Hour)
	_ = os.Chtimes(stale, old, old)
	store.cleanup(time.Now())
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale owned artifact survived")
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatal("cleanup touched a foreign file")
	}
}
