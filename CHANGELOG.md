# Changelog

## v1.1 — 2026-09-22
- Fixed `ws keep` + `ws layer gc` bug: committed layers were garbage collected because the workspace's `formedFrom` wasn't updated after commit. Now `keep` updates workspace metadata to point to the new layer, making it GC-rooted.

## v1.0 — 2026-09-22
- Initial prototype with 7 commands: `get`, `run`, `diff`, `keep`, `drop`, `graph`, `layer`
- Three source types: `layer:hash`, `ws:name`, `base:repo#ref`
- Copy-based backend on macOS (no overlayfs)
- JSON metadata: `~/.ws/meta/layers.json`, `~/.ws/meta/workspaces.json`
- Transitive closure GC for layers
