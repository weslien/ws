# Changelog

All notable changes to `ws` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
