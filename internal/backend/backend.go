package backend

import "io"

// OperationLogger receives human-readable progress messages.
type OperationLogger interface {
	Log(format string, args ...any)
	Error(format string, args ...any)
}

// Workspace describes a mutable workspace backed by a specific backend.
type Workspace struct {
	Name       string
	FormedFrom string        // layer hash this ws was forked from
	Source     string        // e.g. layer:abc or ws:other
	CreatedAt  string        // RFC3339
}

// Layer describes an immutable, content-addressed directory.
type Layer struct {
	Hash        string
	Parent      string
	Basis       []string      // multi-lower for future unions
	Message     string
	CreatedAt   string        // RFC3339
	CommittedBy string
}

// Backender is the interface each platform backend implements.
// The storage package (JSON metadata) is shared; the backend
// handles filesystem operations and lifecycle.
type Backender interface {
	// Name returns the backend kind (e.g. "copy", "overlayfs").
	Name() string

	// Init ensures root dirs exist and performs one-time setup.
	Init(root string) error

	// Fork creates a new workspace named dst from a layer source (content-addressed hash).
	// Returns the layer hash used as the new workspace's base.
	Fork(srcHash string, dstName string, logger OperationLogger) error

	// ForkFromWorkspace forks from an existing workspace (mutable upper).
	// Returns the hash of the snapshot that was used as the new workspace's base.
	// For copy backend this returns the hash of the source workspace's current state.
	ForkFromWorkspace(srcName string, dstName string, logger OperationLogger) (string, error)

	// CloneGitRepo retrieves a git repository into a temporary directory,
	// hashes it, and stores it as a new layer. Returns the layer hash and
	// the path to the copy (caller manages cleanup).
	CloneGitRepo(repo string, ref string, logger OperationLogger) (string, string, error)

	// Mount makes the workspace active for execution. For copy backend
	// this is a no-op. For overlayfs this creates the overlay mount.
	Mount(name string, layerHash string, logger OperationLogger) error

	// Unmount tears down the workspace mount. For copy backend this
	// is a no-op.
	Unmount(name string, logger OperationLogger) error

	// Diff computes the difference between a workspace and a layer or
	// between two workspaces. It writes unified diff to w.
	Diff(wsA string, wsB string, layerB string, w io.Writer, logger OperationLogger) error

	// Commit materializes a workspace's current state into a new
	// immutable layer, returning its hash.
	Commit(name string, logger OperationLogger) (string, error)

	// Destroy removes a workspace entirely.
	Destroy(name string, logger OperationLogger) error

	// LayerHash computes the content-addressed hash of a directory.
	LayerHash(dir string) string
}
