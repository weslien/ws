# ws

**Fast, lightweight workspace graphs for Git repositories.**

`ws` creates **mutable workspaces** from **immutable layers** — think lightweight branching for entire working directories. Branch from another agent's workspace. Isolate to a clean base. Diff, commit, and garbage-collect — all without touching Git itself.

On Linux, workspaces are [overlayfs](https://docs.kernel.org/filesystems/overlayfs.html) mounts (instant fork, zero-copy). On macOS, they use efficient directory copies. The **CLI and graph model are identical** on both platforms.

```bash
# Start from a repository
$ ws get base:https://github.com/you/project --name=feature-a
workspace feature-a formed from a3f4d...

# Work inside it
$ ws run feature-a -- npm install

# Branch — build on another agent's work
$ ws get ws:feature-a --name=feature-b
workspace feature-b formed from 8c2e1...

# Diff against the base
$ ws diff feature-a
--- a/package.json
+++ b/package.json
@@ -1,3 +1,5 @@
 {
-  "version": "1.0.0"
+  "version": "1.1.0"
   "dependencies": {
     "express": "^4.21.0"
+    "lodash": "^4.17.21"

# Promote changes to a permanent layer
$ ws keep feature-a --message="added lodash"
kept layer 8c2e1ba7d3f4...

# See the graph
$ ws graph feature-b
ws:feature-b [active]
  └─ layer:8c2e1ba7d3f4 [added lodash]
    └─ layer:a3f4d9c2e1b5 [base:https://github.com/you/project#HEAD]
```

---

## Install

### macOS (Apple Silicon)

Requires Go 1.23+.

```bash
git clone https://github.com/weslien/ws.git
cd ws
go build -o ws ./cmd/ws
sudo mv ws /usr/local/bin/  # optional
```

By default, `ws` on macOS uses a **copy backend** — `overlayfs` is unavailable at the macOS kernel level. The copy backend is O(n) per fork but correct and dependency-free.

#### Optional: real overlayfs via `container` ⚡ EXPERIMENTAL

If you have Apple's [`container`](https://github.com/apple/container) installed (macOS 26+, Apple Silicon), `ws` automatically uses it to get real Linux VMs with native overlayfs:

```bash
# Install container first: https://github.com/apple/container#initial-install
container system start

# Now ws auto-detects 'container' and routes commands through Linux VMs
ws get base:https://github.com/you/project --name=feature-a
ws run feature-a -- ls -la             # executes inside the VM
ws diff feature-a                     # diff against the VM's overlay lower
```

This gives you fast, copy-on-write workspace forks on macOS. If `container` is not installed, `ws` falls back silently to the copy backend.

### Linux (x86_64, ARM64)

Requires Go 1.23+ and `fuse-overlayfs` (for unprivileged overlay mounts).

```bash
# Debian / Ubuntu
sudo apt install fuse-overlayfs  # or: sudo apt install fuse3

# Fedora / RHEL
sudo dnf install fuse-overlayfs

# Build
git clone https://github.com/weslien/ws.git
cd ws
go build -o ws ./cmd/ws
sudo mv ws /usr/local/bin/  # optional
```

### Verify

```bash
ws graph
# → Layers:
# → Workspaces:
```

---

## Concepts

| Term | Definition |
|------|-----------|
| **Layer** | An immutable, content-addressed directory. Stored in `~/.ws/layers/<hash>`. Once created, never changes. The unit of sharing. |
| **Workspace** | A mutable working directory. Stored in `~/.ws/workspaces/<name>`. Created from a layer (`formed_from`). You `keep` it to promote to a new layer, or `drop` it to destroy. |
| **Graph** | The provenance chain of a workspace: the sequence of `parent` → `parent` → base layers. `ws graph` prints this tree. |

### How it differs from Git worktrees

Git worktrees let you check out multiple branches into separate directories. `ws` is different:

- **Branch from uncommitted work** — `ws get ws:agent-a` captures agent-a's current dirty state, not just a committed branch.
- **Isolation is explicit** — `ws get layer:base` gives a clean slate from the immutable base, not whatever happens to be on `main` right now.
- **No Git indexing** — operations don't touch `.git/index`, so there's no risk of index corruption from concurrent agent access.
- **Content-addressed base layers** — a layer with hash `a3f4d...` is *exactly* that content, forever. No ambiguity about what "HEAD" meant at fork time.

### Source types

| Source | Meaning |
|--------|---------|
| `layer:<hash>` | Fork from an immutable layer |
| `ws:<name>` | Branch from another workspace's current state (snapshot-first) |
| `base:<repo>#<ref>` | Clone a git repo, create a layer from it, fork from that layer |

---

## Commands

| Command | Description |
|---------|-------------|
| `ws get <source> --name=<ws>` | Create a workspace from a layer, another workspace, or a git repo |
| `ws run <ws> -- <cmd...>` | Execute a command inside a workspace's filesystem |
| `ws diff <ws> [target]` | Diff workspace vs its base, or vs another workspace/layer |
| `ws keep <ws> [--message=...]` | Materialize workspace changes as an immutable layer |
| `ws drop <ws>` | Destroy workspace and all uncommitted changes |
| `ws graph [ws]` | Print the layer + workspace DAG, or a specific workspace's provenance |
| `ws layer ls` | List all layers |
| `ws layer show <hash>` | Show layer metadata |
| `ws layer gc` | Garbage-collect unreferenced layers |


### `ws get` — Create a workspace

```bash
# From a layer
ws get layer:a3f4d9c2e1b5 --name=feature-a

# From another workspace (snapshot first, branch second)
ws get ws:feature-a --name=feature-b

# From a git repo
ws get base:https://github.com/you/project#main --name=feature-c
ws get base:/path/to/local/repo --name=feature-d
```

### `ws run` — Execute inside a workspace

```bash
ws run feature-a -- cat package.json
ws run feature-a -- npm install
ws run feature-a -- go test ./...
```

On Linux, the workspace is a mounted overlayfs — all writes go to the private upper directory.

### `ws diff` — Compare changes

```bash
# Diff workspace vs the layer it was forked from
ws diff feature-a

# Diff two workspaces
ws diff feature-a feature-b

# Diff workspace vs a specific layer
ws diff feature-a layer:a3f4d9c2e1b5
```

### `ws keep` — Promote to layer

```bash
ws keep feature-a --message="installed dependencies"
# → kept layer 8c2e1ba7d3f4
```

After `keep`, the new layer is recorded in the graph. The workspace now tracks from the new layer as its base.

### `ws drop` — Destroy

```bash
ws drop feature-a
```

All uncommitted changes are lost. The workspace directory is removed.

### `ws graph` — Visualize

```bash
# All layers and workspaces
ws graph

# Provenance tree for a specific workspace
ws graph feature-b
```

### `ws layer` — Layer management

```bash
ws layer ls
ws layer show a3f4d9c2e1b5
ws layer gc        # removes layers no longer referenced by any workspace
```

---

## Worked Example

```bash
# 1. Create a test repository
mkdir /tmp/demo && cd /tmp/demo
git init
echo '{"name": "demo"}' > package.json
git add . && git commit -m "initial"

# 2. Form a workspace
ws get base:/tmp/demo --name=agent-1
# → created layer a3f4d9c2e1b5...
# → workspace agent-1 formed from a3f4d9c2e1b5

# 3. Modify
ws run agent-1 -- sh -c 'echo {"version": "1.0"} > package.json'
ws diff agent-1
# → --- package.json
# → +++ package.json
# → @@ -1 +1 @@
# → -{"name": "demo"}
# → +{"version": "1.0"}

# 4. Branch — agent-2 sees agent-1's work
ws get ws:agent-1 --name=agent-2
ws run agent-2 -- cat package.json
# → {"version": "1.0"}

# 5. Isolate — agent-3 gets the clean base
ws get layer:a3f4d9c2e1b5 --name=agent-3
ws run agent-3 -- cat package.json
# → {"name": "demo"}

# 6. Commit agent-1's changes
ws keep agent-1 --message="added version"
# → kept layer 8c2e1ba7d3f4...

# 7. See the graph
ws graph agent-2
# ws:agent-2 [active]
#   └─ layer:8c2e1ba7d3f4 [added version]
#     └─ layer:a3f4d9c2e1b5 [base:/tmp/demo#HEAD]

# 8. Clean up
ws drop agent-3
ws layer gc
```

---

## Storage Layout

```
~/.ws/
├── layers/
│   ├── a3f4d9c2e1b5/          # immutable layer content
│   └── 8c2e1ba7d3f4/
├── workspaces/
│   ├── agent-1/                # mutable workspace (overlay mount on Linux)
│   └── agent-2/
├── uppers/                     # overlayfs upper dirs (Linux only)
│   ├── agent-1/
│   └── agent-2/
├── workdirs/                   # overlayfs work dirs (Linux only)
│   ├── agent-1/work/
│   └── agent-2/work/
└── meta/
    ├── layers.json             # layer metadata (parent, message, timestamp)
    └── workspaces.json         # workspace metadata (formed_from, state)
```

Layers are content-addressed by a SHA-256 truncated to 16 hex characters.

---

## Platform Notes

### Linux

- Uses `fuse-overlayfs` for unprivileged overlay mounts — no `sudo` required.
- `fuse-overlayfs` packages: `fuse-overlayfs` (Debian/Ubuntu), `fuse3` (Alpine), or compile from [source](https://github.com/containers/fuse-overlayfs).
- Each workspace = one overlay mount. Forking snapshots the source workspace to a new layer first, then mounts the new workspace from that layer.

### macOS

- By default, uses directory copies for all fork/branch operations (no kernel overlayfs available).
- If Apple's [`container`](https://github.com/apple/container) is installed, automatically routes through lightweight Linux VMs with native overlayfs. See [Install → macOS](#macos-apple-silicon).
- No additional dependencies beyond Go (copy backend).

### Windows

- Not currently supported. Contributions welcome.

---

## Architecture

```
internal/backend/
  backend.go              — Backender interface (platform abstraction)
  copy.go                 — CopyBackend (macOS, Windows fallback)
  overlayfs_linux.go      — OverlayfsBackend (Linux, fuse-overlayfs)
  platform*.go            — Build-tag backend selection

internal/storage/
  metadata.go             — JSON metadata for layers and workspaces

cmd/ws/main.go            — CLI
```

Adding a new backend (e.g., `container` on macOS 26, APFS clone, or ZFS snapshot) requires implementing the `Backender` interface. The CLI, graph engine, and metadata layer are platform-agnostic.

---

## FAQ

**Q: Why not just use Git worktrees?**

Git worktrees are great for human developers working on committed branches. `ws` is designed for agents that may have:
- Uncommitted state that another agent wants to build on
- Concurrent access to the same repo (no index races)
- Need to isolate to a guaranteed-clean base, not "whatever main is now"

**Q: Can I use this to replace Docker volumes or dev containers?**

Not directly — `ws` is a filesystem primitive, not a runtime. But it's designed to compose cleanly with containers: a container runtime can mount a `ws` workspace as its working directory.

**Q: Does `ws` track file renames efficiently?**

Not yet — renames trigger full copy-up on the copy backend, and overlayfs handles them at the filesystem level on Linux. A Merkle-tree content-addressed backend would optimize this.

**Q: How do I clean up old layers?**

`ws layer gc` removes any layer not transitively reachable from a live workspace. Run it periodically.

---

## Contributing

1. Fork the repository
2. Create a branch: `git checkout -b feature/xyz`
3. Make your changes
4. Run `go test ./...` and `go vet ./...`
5. Submit a pull request

Please open an issue before major design changes.

---

## Acknowledgments

Inspired by:
- [Git worktrees](https://git-scm.com/docs/git-worktree) — the original multi-workspace idea
- [Docker overlayfs driver](https://docs.docker.com/storage/storagedriver/overlayfs-driver/) — the filesystem layering model
- [Nix store](https://nixos.org/manual/nix/stable/store/) — content-addressed store paths

---

## License

MIT — see [LICENSE](LICENSE).
