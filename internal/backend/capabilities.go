package backend

// Optional capabilities implemented by some backends. Commands probe for
// these with type assertions so the core Backender interface stays small.

// Execer is a backend that can execute a command inside the workspace's
// own execution environment (e.g. the Apple container machine on macOS),
// with full TTY/stdin/stdout/stderr forwarding and exit-code propagation.
// When not implemented, commands run host-side as before.
//
// IMPORTANT: backends implementing Execer MUST make host-side reads of
// workspace content correct (or impossible) — e.g. by mounting content in
// the VM only, never re-deriving it in a way the host can't see. The
// container backend documents its guarantee in container_overlay_darwin.go.
type Execer interface {
	Exec(name string, argv []string) error
}

// WorkspaceHasher is a backend whose workspace content may not be
// directly hashable from the host (e.g. content exists only behind a VM
// overlay). HashWorkspace returns a content hash comparable to
// LayerHash(dir), or "?" when hashing is impossible (never "" — that
// would alias the empty-layer hash).
type WorkspaceHasher interface {
	HashWorkspace(name string) string
}

// Exporter is a backend whose workspace content may not be directly
// copyable from the host. ExportWorkspace copies the full merged
// workspace content to dest (which is created if needed).
type Exporter interface {
	ExportWorkspace(name string, dest string, logger OperationLogger) error
}
