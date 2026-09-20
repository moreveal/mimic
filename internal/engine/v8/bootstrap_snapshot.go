//go:build (windows || linux) && amd64

package v8

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	gov8 "github.com/maclof/gov8"
	"github.com/moreveal/mimic/internal/engine"
)

func (Factory) BootstrapSnapshotsEnabled() bool {
	return os.Getenv("MIMIC_DIAGNOSTICS") != "1" || os.Getenv("MIMIC_PROFILE_HOSTS") == "1"
}

func (Factory) BootstrapSnapshotIdentity() (string, error) {
	build, err := gov8.VersionString()
	if err != nil {
		return "", err
	}
	runtimeVersion, err := gov8.RuntimeVersionString()
	if err != nil {
		return "", err
	}
	return build + "\x00" + runtimeVersion, nil
}

func (Factory) LoadBootstrapSnapshot(data []byte) (engine.BootstrapSnapshot, error) {
	if len(data) == 0 {
		return nil, errors.New("empty bootstrap snapshot")
	}
	if _, err := initialize(); err != nil {
		return nil, err
	}
	blob := gov8.StartupDataFromBytes(data)
	// Validate through an actual consumer creation here, before the artifact is
	// admitted to the Browser cache. This turns corrupt cache files into a
	// recoverable rebuild instead of exposing them to a Page.
	consumer, err := blob.ShareImmutableBytes()
	if err != nil {
		_ = blob.Release()
		return nil, err
	}
	owner, err := newRuntime(consumer)
	if err != nil {
		_ = consumer.Release()
		_ = blob.Release()
		return nil, err
	}
	adapter, err := newAdapter(owner, nil)
	if err != nil {
		_ = owner.Dispose()
		_ = blob.Release()
		return nil, err
	}
	if err := adapter.Close(); err != nil {
		_ = blob.Release()
		return nil, err
	}
	return &bootstrapSnapshot{blob: blob, size: len(data)}, nil
}

type bootstrapSnapshot struct {
	mu   sync.Mutex
	blob *gov8.StartupData
	size int
}

// BuildBootstrapSnapshot serializes complete pure-JS initialization. Native
// host bindings belong to each restored runtime and must be installed later.
func (Factory) BuildBootstrapSnapshot(ctx context.Context, sources ...string) (engine.BootstrapSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := initialize(); err != nil {
		return nil, err
	}
	type result struct {
		blob *gov8.StartupData
		err  error
	}
	done := make(chan result, 1)
	// The creator owns/locks this goroutine's OS thread. The caller can be an
	// existing Page owner; creating the snapshot must not change its V8 stack.
	go func() { blob, err := buildBootstrapSnapshot(ctx, sources...); done <- result{blob, err} }()
	value := <-done
	if value.err != nil {
		return nil, value.err
	}
	return &bootstrapSnapshot{blob: value.blob, size: len(value.blob.Bytes())}, nil
}

func buildBootstrapSnapshot(ctx context.Context, sources ...string) (*gov8.StartupData, error) {
	creator, err := gov8.NewSnapshotCreator()
	if err != nil {
		return nil, err
	}
	consumed := false
	defer func() {
		if !consumed {
			_ = creator.Close()
		}
	}()
	iso := creator.Isolate()
	if err := iso.SetMicrotasksPolicy(gov8.PolicyExplicit); err != nil {
		return nil, err
	}
	realm, err := iso.NewContext()
	if err != nil {
		return nil, err
	}
	scope, err := iso.NewScope()
	if err != nil {
		_ = realm.Close()
		return nil, err
	}
	for _, source := range sources {
		err = runSnapshotSeed(ctx, iso, realm, scope, source)
		if err != nil {
			break
		}
	}
	if err == nil {
		err = creator.SetDefaultContext(realm)
	}
	err = errors.Join(err, scope.Close(), realm.Close())
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	// Serialization is a bounded native phase. Cancellation is checked again
	// after it finishes; never terminate the serializer midway through cleanup.
	blob, err := creator.CreateBlob(gov8.FunctionCodeKeep)
	consumed = true // CreateBlob consumes its creator, including native failures.
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		_ = blob.Release()
		return nil, err
	}
	return blob, nil
}

func runSnapshotSeed(ctx context.Context, iso *gov8.Isolate, realm *gov8.Context, scope *gov8.Scope, source string) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ctx.Done() != nil {
		finished, joined := make(chan struct{}), make(chan struct{})
		handle := iso.ThreadSafeHandle()
		go func() {
			defer close(joined)
			select {
			case <-ctx.Done():
				handle.TerminateExecution()
			case <-finished:
			}
		}()
		defer func() {
			close(finished)
			<-joined
			if ctx.Err() != nil {
				_ = iso.CancelTerminateExecution()
				err = ctx.Err()
			}
		}()
	}
	catcher, err := iso.NewTryCatch()
	if err != nil {
		return err
	}
	defer catcher.Close()
	script, err := realm.Compile(scope, source, catcher)
	if err != nil {
		return exceptionError(catcher, scope, realm, "bootstrap snapshot", err)
	}
	defer script.Close()
	if _, err = script.Run(scope, catcher); err != nil {
		return exceptionError(catcher, scope, realm, "bootstrap snapshot", err)
	}
	return nil
}

func (s *bootstrapSnapshot) NewRuntime() (engine.Runtime, error) {
	s.mu.Lock()
	if s.blob == nil {
		s.mu.Unlock()
		return nil, errors.New("bootstrap snapshot is closed")
	}
	// Every runtime owns its native consumer records, so teardown releases its
	// native blob even while other Pages live. The serialized Go bytes are
	// immutable and can share their backing storage with the cache and siblings.
	consumer, err := s.blob.ShareImmutableBytes()
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	profile := newDiagnostics()
	started := time.Now()
	owner, err := newRuntime(consumer)
	if err != nil {
		_ = consumer.Release()
		return nil, err
	}
	if profile != nil {
		profile.Costs["factory:isolate"] = diagnosticCost{Count: 1, Nanoseconds: time.Since(started).Nanoseconds()}
	}
	return newAdapter(owner, profile)
}

func (s *bootstrapSnapshot) SizeBytes() int { return s.size }
func (s *bootstrapSnapshot) BootstrapSnapshotBytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.blob == nil {
		return nil
	}
	data := s.blob.Bytes()
	return append([]byte(nil), data...)
}
func (s *bootstrapSnapshot) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.blob == nil {
		return nil
	}
	err := s.blob.Release()
	if err == nil {
		s.blob = nil
	}
	return err
}

var _ engine.BootstrapSnapshotFactory = Factory{}
var _ engine.PersistentBootstrapSnapshotFactory = Factory{}
var _ engine.BootstrapSnapshot = (*bootstrapSnapshot)(nil)
var _ engine.PersistentBootstrapSnapshot = (*bootstrapSnapshot)(nil)
