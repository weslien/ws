# Changelog

All notable changes to `ws` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- Docs: macOS story updated to match the shipped Apple `container` backend (auto-detect, per-workspace Linux VMs with real overlayfs, copy fallback). README, `ws help get/agent/concepts/platform`, and usage text previously described macOS as "directory copies only".

## [0.9.0] — 2026-09-28

### Changed
- Refactor: per-entity metadata files (one JSON per workspace, one per layer) instead of monolithic `workspaces.json`/`layers.json` indexes. Eliminates the read-modify-write race; normal operations need no locking, only GC takes the global lock. Atomic writes (temp + rename) are crash-safe.

## [0.8.0] — 2026-09-28

### Fixed
- GlobalLock moved to metadata-only section in `cmdGet` — not held during slow I/O (clone, fork, copyDir). Prevents serialization on macOS where CopyBackend.Fork is slow.
- `UpdateWorkspaces`/`UpdateLayers` helpers with atomic read-modify-write under GlobalLock.
- `ws layer files` and `ws layer cat` check filesystem first, fall back to `layers.json` for error messages. Prevents false "layer not found" when GC deletes metadata but directory exists.
- Layer directory verification after `Commit`.
- Better `CopyBackend.Commit` error messages with source/dest paths.
- t_cond retry (3 attempts, 1s/2s/3s backoff) for CopyBackend timing.
- 101/101 scenarios pass on macOS and Linux, sequentially and in parallel.

## [0.1.1] — 2026-09-22

### Added
- Linux overlayfs backend via `fuse-overlayfs` — unprivileged, no root required.
- Pluggable backend architecture: `internal/backend.Backender` interface.
- Platform auto-selection via Go build tags: `linux` → overlayfs, others → copy.
- `ForkFromWorkspace` now snapshots before branching (immutable base guarantee).
- Proper cleanup on drop: unmount overlayfs, remove uppers, workdirs, workspaces.
- Transitive closure garbage collection for layers.

### Changed
- Moved source from flat `main.go` to `cmd/ws/main.go` + `internal/` packages.
- `Backender.ForkFromWorkspace` returns `(string, error)` — the hash of the snapshot used.

### Fixed
- Linux backend properly persists cloned layers from `base:` source.
- `ws keep` updates workspace metadata so committed layers aren't GC'd.

## [0.1.0] — 2026-09-22

### Added
- Initial prototype with 7 commands: `get`, `run`, `diff`, `keep`, `drop`, `graph`, `layer`.
- Three source types: `layer:hash`, `ws:name`, `base:repo#ref`.
- Copy-based backend (macOS-compatible, O(n) fork).
- JSON metadata: `~/.ws/meta/layers.json`, `~/.ws/meta/workspaces.json`.
- Content-addressed layer hashing (SHA-256 truncated to 16 hex chars).
