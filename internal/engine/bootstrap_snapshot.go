package engine

import "context"

// BootstrapSnapshotFactory optionally prepares an immutable, engine-specific
// snapshot from complete, quiescent JavaScript initialization. The source must
// not depend on embedder callbacks or contain live external resources.
type BootstrapSnapshotFactory interface {
	// BootstrapSnapshotsEnabled permits backend diagnostics to request ordinary
	// execution rather than omit the source intervals they intend to measure.
	BootstrapSnapshotsEnabled() bool
	// Sources execute in order in the same seed realm. Separate scripts allow
	// temporary initialization data to be collected independently of closures.
	BuildBootstrapSnapshot(context.Context, ...string) (BootstrapSnapshot, error)
}

// BootstrapSnapshot creates independent runtimes with separately owned realm
// objects. Close prevents subsequent creation without invalidating existing
// runtimes. Implementations support concurrent creation and Close.
type BootstrapSnapshot interface {
	NewRuntime() (Runtime, error)
	Close() error
	// SizeBytes reports the immutable serialized size, including after Close.
	SizeBytes() int
}
