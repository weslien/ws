# Changelog

All notable changes to `ws` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- macOS container backend (Apple `container`): **in-VM execution and overlayfs** (issue #1). New workspaces default to overlay mode — the layer is mounted read-only inside the workspace's Linux VM (fork = machine boot, no content copy); `ws run` executes inside the VM against the merged view with TTY forwarding and exit-code propagation (`WS_VM_ROOT=1` for root). `ws status` and `ws export` read VM-side content via new optional backend capabilities (`WorkspaceHasher`, `Exporter`, `Execer`). On mount failure workspaces transparently degrade to shared-dir mode (layer staged host-side, no data loss). `WS_CONTAINER_MODE=shared` forces shared-dir forks for host-side readability. Overlay upper/work dirs are chowned to the probed default in-VM user (`id -u`/`id -g` via `machine run`, no `--root`), and a write test after mounting falls back to shared-dir mode when the merged view is not writable (e.g. id-mapped virtiofs) instead of keeping a broken overlay.

### Fixed
- Progress logs now write to stderr; `--json` output on stdout is pure JSON (previously lines like `overlayfs active in machine …` broke `ws keep --json | jq`).
- Installer: resolve "latest" from releases (the tags API is not semver-ordered, and tags exist for releases whose pipeline failed — v0.7.0–v0.9.0); verify prebuilt tarballs against the published `.sha256` asset; build from source at the release tag (never stamp `main` with a release version); report the installed binary's version by path, not PATH-resolved `ws`. wget fallback; README install section matches behavior.

## [0.9.1] — 2026-10-01

### Fixed
- **macOS container backend: `container machine run` joins positional argv into one command line**, breaking every in-VM shell script (the Fork copy failed with a BusyBox tar usage dump + "tar: short read"). One-shot `argvJoinProbe` detects the exec semantics; in-VM scripts are POSIX-quoted (`quotePOSIX`) so they arrive as a single `sh -c` argument under both semantics. All in-VM invocations use the documented `--` separator and print the first machine-run command line to stderr (`WS_DEBUG=1` traces all). Verified end-to-end on macOS (Apple Container VM): fork → host-side `ws run` write → `ws keep` → `ws layer cat` round-trips.
- **`ws keep` on the container backend silently produced empty layers**: Commit staged exports under `/tmp`, which is not visible inside the machine (only the host home is virtiofs-mounted). Staging moved to `~/.ws/tmp`.
- **Windows builds broken** since the per-entity metadata refactor (and with them every Release run since v0.6.4): `syscall.Flock` does not exist on Windows. `GlobalLock` split into `lockFileExclusive` — flock on Unix, `LockFileEx` via `golang.org/x/sys/windows` on Windows. x/sys pinned to v0.30.0 (compatible with Go 1.23.4, no toolchain bump). No release published between v0.6.4 and v0.9.1 as a result.
- Removed the never-functioning in-VM overlay mount from Fork. With quoting fixed it would have succeeded — and silently hidden host-side `ws run` writes from `ws keep` (execution runs on the Mac). In-VM overlayfs returns with in-VM execution routing (#1).

### Changed
- Docs: macOS story matches the shipped backend — per-workspace Linux VMs via Apple `container` (auto-detected), honest fork cost (VM boot + in-VM staging), in-VM overlayfs documented as planned with rationale (#1). Windows platform section corrected (release binaries, no "cross-compiles cleanly" claim); `dir:<path>` added to the README source-type table; 0.9.0 CHANGELOG entry backfilled.

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
