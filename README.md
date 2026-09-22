# ws — workspace graph CLI (v1, macOS copy backend)

A CLI tool for managing **workspace graphs** using immutable layers and mutable workspaces. Designed for multi-agent code isolation, where sometimes you want to share state (branch from another agent's workspace) and sometimes you want to isolate (fork from a base layer).

On macOS (no native `overlayfs`), the backend uses full directory copies. On Linux, the backend swaps copies for `overlayfs` mounts — the CLI and graph model stay identical.

---

## Quick Start

```bash
# Build
cd ~/workspace/ws-cli && go build -o ws .

# 1. Form a workspace from a git repo
./ws get base:/path/to/repo --name=agent-a

# 2. Run commands inside it
./ws run agent-a -- cat package.json

# 3. Branch it — agent-b sees agent-a's current state
./ws get ws:agent-a --name=agent-b

# 4. Or isolate — agent-c starts from the original base
./ws get layer:4f76d0bd87b40443 --name=agent-c

# 5. Diff against fork point or cross-diff
./ws diff agent-a              # vs original base layer
./ws diff agent-a agent-b      # two workspaces head-to-head

# 6. Commit a workspace to a permanent, content-addressed layer
./ws keep agent-a --message="installed dependencies"

# 7. Drop a workspace when done (changes lost unless kept)
./ws drop agent-a

# 8. View the graph
./ws graph                   # all layers + workspaces
./ws graph agent-b           # provenance tree for a workspace

# 9. Layer management
./ws layer ls                # list all layers
./ws layer show <hash>      # layer metadata
./ws layer gc               # garbage-collect unreferenced layers
```

---

## Commands

| Command | Args | What it does |
|---------|------|-------------|
| `get` | `<source> --name=<ws>` | Create a workspace from a layer, another workspace, or a git repo |
| `run` | `<ws> -- <cmd>` | Execute a command inside a workspace's filesystem |
| `diff` | `<ws> [ws-B \| layer:hash]` | Diff workspace vs its base, or vs another workspace/layer |
| `keep` | `<ws> [--message=<msg>]` | Materialize workspace changes as an immutable layer |
| `drop` | `<ws>` | Destroy workspace (uncommitted changes are lost) |
| `graph` | `[ws]` | Print the layer + workspace DAG |
| `layer` | `ls \| show <hash> \| gc` | Layer operations |

### Source types for `get`

- `layer:<hash>` — fork from an immutable layer
- `ws:<name>` — branch from another workspace's current state (live copy)
- `base:<repo>#<ref>` — clone a git repo, auto-layerize it, fork from the result

---

## Concepts

### Layer (immutable)
Content-addressed directory, stored in `~/.ws/layers/<hash>`. Once created, never changes. The unit of sharing.

### Workspace (mutable)
Named working directory in `~/.ws/workspaces/<name>`. Each workspace was formed from a layer (`formed_from`). You can `keep` it to promote it to a new layer, or `drop` it to destroy.

### Graph
A workspace or layer has provenance: the chain of `parent` → `parent` → base. `graph` prints this tree.

### Merge semantics
There is **no `merge` command**. Overlayfs doesn't merge — it overlays. True merge (conflict resolution) is policy, not storage. Do it in a workspace with standard tools (git, diff3), then `keep` the result.

---

## macOS Backend (v1)

macOS lacks `overlayfs`. Current backend:
- `get` → `cp -R` the source to the workspace directory
- `keep` → `cp -R` workspace to a new layer directory; compute hash
- `run` → `cd` into workspace, execute
- `diff` → `diff -ruN`

This is **correct but O(n)** per fork. A 100MB repo = 100MB copy each time. Acceptable for a prototype used on small repos.

### Path to Linux `overlayfs` Backend

Replace four implementation functions, keep everything else:

| Operation | macOS (now) | Linux (target) |
|-----------|-------------|----------------|
| Fork workspace | `cp -R` | `mount -t overlay lowerdir=<layer>,upperdir=<new>,workdir=<work>` |
| Branch workspace | `cp -R` | `mount -t overlay lowerdir=<ws>:<base>,upperdir=<new>,workdir=<work>` |
| Keep layer | `cp -R` + hash | `sync` upperdir + `umount` |
| Drop workspace | `rm -rf` | `umount` + `rm -rf` upperdir |

The CLI, graph logic, metadata, and `diff`/`graph`/`layer` commands are unchanged.

---

## Data Model

```yaml
Layer:
  hash: sha256:abc...
  parent: sha256:def...       # single parent for linear history
  basis: [sha256:def...]     # multi-lower for unions (future)
  message: "installed deps"
  created_at: 2026-09-22T...
  committed_by: agent-a

Workspace:
  name: agent-a
  source: layer:abc...       # what it was created from
  formed_from: abc...       # the layer hash (for provenance)
  state: active
  created_at: 2026-09-22T...
```

Metadata is JSON: `~/.ws/meta/layers.json`, `~/.ws/meta/workspaces.json`.

---

## Known Issues / Limitations

1. **Local git clone with `--depth=1` is ignored for `file://` repos.** Go's `os/exec` passes `--depth=1` but Git silences it for local paths. This is harmless — the full history is cloned. For remote repos, `--depth=1` works correctly.
2. **Branching from a workspace copies its ENTIRE current state** (not just committed layers). If you branch `ws:agent-a` and then `agent-a` continues working, `agent-b` won't see new changes. To share new work, `keep` agent-a first.
3. **No directory rename optimization.** Large directory moves trigger full copy-up. Acceptable for V1.
4. **No concurrent workspace access safety.** Two `run` commands in the same workspace race on the filesystem. Kata containers solve this in practice (each agent in its own VM).
5. **hashDir() is naive.** It reads every file and hashes content sequentially. Fast enough for small repos (sub-second), not for GB-scale.

---

## Tested Scenario (from actual run)

```bash
# Create a test repo
mkdir /tmp/test-repo && cd /tmp/test-repo
git init && echo '{"name": "myproject"}' > package.json
git add . && git commit -m "initial"

# Form workspace from it
./ws get base:/tmp/test-repo --name=agent-1
# → created layer 4f76d0bd87b40443
# → workspace agent-1 formed from 4f76d0bd87b40443

# Mutate
./ws run agent-1 -- sh -c 'echo {"version": "1.0"} > package.json'
./ws diff agent-1
# → shows package.json changed

# Branch: agent-2 sees agent-1's mutated state (copy backend)
./ws get ws:agent-1 --name=agent-2
./ws run agent-2 -- cat package.json
# → {version: 1.0}

# Isolate: agent-3 gets the original base
./ws get layer:4f76d0bd87b40443 --name=agent-3
./ws run agent-3 -- cat package.json
# → {"name": "myproject"}

# Commit agent-1's changes
./ws keep agent-1 --message="added version"
# → kept layer 52fafb35bf49e28f

# Cross-diff: agent-2 vs agent-3
./ws diff agent-2 agent-3
# → diff shows the version change

# Graph
./ws graph agent-2
# ws:agent-2 [active]
#   └─ layer:4f76d0bd87b40443 [base:/tmp/test-repo#HEAD]

# Drop a workspace
./ws drop agent-3

# GC removes layers nobody references
./ws layer gc
# → gc: removed 1 unreferenced layers

# Still works
./ws run agent-1 -- cat package.json
# → {version: 1.0}
```

---

## Files

- `~/workspace/ws-cli/ws` — built binary
- `~/workspace/ws-cli/main.go` — source
- `~/.ws/layers/` — immutable layers
- `~/.ws/workspaces/` — mutable workspaces
- `~/.ws/meta/` — JSON metadata

---

## Next Steps

1. **Linux `overlayfs` backend** — swap `copyDir` for `mount` + `umount`
2. **Merkle-tree hashing** — `hashDir()` is O(n) per file; Merkle tree enables partial diff
3. **Daemon mode (`wsd`)** — mount lifecycle management, garbage collection background, API for Kata/fa-serve integration
4. **Union layers** — `ws layer union a b` creates an ordered overlay (lowerdir=b:a:base)
5. **Workspace handoff** — `ws hand agent-a --to=agent-b` (ownership transfer)
6. **Merge workspace** — special workspace that mounts two layers + a conflict-view tool

---

## License

Internal prototype for evroc. Not open-sourced.
