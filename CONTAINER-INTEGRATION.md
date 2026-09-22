# Apple `container` Integration Analysis
## How `container` (Apple) Maps to `ws` Workspace Graph

**Date**: 2026-09-22  
**Tool**: https://github.com/apple/container — container on macOS via lightweight Linux VMs  
**Status**: macOS 26+, Apple Silicon only, Apache 2.0

---

## What `container` Gives Us

| Feature | What It Means for `ws` |
|---------|----------------------|
| **Lightweight Linux VMs** | Real kernel inside. `overlayfs` is available natively. |
| **OCI image layer graph** | Images are already DAGs of tar.gz diffs — content-addressed by design. |
| **Named volumes** | Persistent `ext4` storage that outlives containers. Fast I/O. |
| **Bind mounts (virtiofs)** | Mac host directories mapped into the VM, live. |
| **Container machines** | Persistent VMs based on OCI images. Home dir auto-mounted. Init system. |
| **Nested virt** | M3+ / macOS 15+ can expose `/dev/kvm` inside the VM. Path to Kata. |
| **Swift CLI** | Native Mac UX, no Docker daemon, signed packages. |

The critical insight: `container`'s image model is ALREADY a layer graph (OCI spec). Its VM rootfs is a writable overlay on top of the image's read-only layers. The gap between what `container` does and what `ws` needs is bridged at both the storage layer AND the execution layer.

---

## Three Integration Models

### Model A: `container` as Execution Target Only (Minimal)

`ws` keeps its current layer graph (directories on Mac). `ws run` dispatches into a `container` VM.

```
Mac host: ~/.ws/layers/      ~/.ws/workspaces/
         ↓ bind mount         ↓ bind mount
  container machine: /layers (RO) + /workspace (RW)
         → overlayfs mount inside VM
         → execute
```

Pros:
- Minimal change to `ws` codebase
- Gets real Linux execution for builds (`npm install` with native deps)
- Natural per-agent VM isolation

Cons:
- Still O(n) copies on Mac for forks (no CoW)
- Bind-mounts over virtiofs are slower than native disk
- Two filesystems to keep in sync

### Model B: `container` Images as Layers (Natural Fit)

Store layers as OCI image layers or local image tarballs. Workspaces are container machines.

```
ws get layer:abc123 --name=agent-a
# → build OCI image from layer:abc123, instantiate container machine "agent-a"
# → machine rootfs is the writable overlay on top of image layers

ws keep agent-a
# → container commit agent-a → new image tag → extract as `ws` layer hash
```

Pros:
- OCI layer graph IS the workspace graph — no impedance mismatch
- `container`'s storage backend handles CoW, dedup, and GC
- `container images` are portable: push to registry, pull on Linux, identical graph

Cons:
- OCI layer format is tar.gz diffs (slower than overlayfs upperdirs for interactive mutation)
- `container commit` requires a full container machine cycle (boot → snapshot → create image) — minutes, not milliseconds
- Each workspace = a container machine = a running/stopped VM. Thousands of workspaces = VM sprawl.

### Model C: Named Volumes as Workspace Upperdirs (Sweet Spot)

Use `container`'s named volume system as the RW storage for workspaces, with OCI images or host directories as the RO base.

```
# 1. Create base image with the repo content
container build -t ws-base:abc123 -f Dockerfile.repo .
#    Dockerfile.repo: FROM scratch; COPY repo-content /workspace

# 2. Form a workspace = create container machine from base image + new named volume
#    The volume starts empty, mounted at /workspace (on top of the base)
#    Built-in overlay is already happening at the VM storage layer
container machine create ws-base:abc123 --name agent-a \
  --volume ws-vol-agent-a:/workspace

# 3. Inside the VM, /workspace is the writable mount where the agent works
#    The base /workspace (from the image) is shadowed by the volume

# 4. Branch from workspace
#    Create new volume copying the source volume's data, then mount as /workspace
container volume create ws-vol-agent-b --from=ws-vol-agent-a
container machine create ws-base:abc123 --name agent-b \
  --volume ws-vol-agent-b:/workspace

# 5. Keep = export volume diffs as new layer
#    Snapshot volume, create image from: base + volume-diff
```

Pros:
- Named volumes are native `ext4`, fast
- Volume copy is a disk-level snapshot (potentially CoW at the hypervisor layer)
- Container machines can be stopped/started, not destroyed
- Natural init system support for long-running processes
- No custom code for VM lifecycle — `container` handles it

Cons:
- Volume copy API may still be O(n) until Apple implements CoW snapshots
- Workspace count = VM count if each workspace is a separate machine. But machines can share a base VM with different volumes mounted.
- `container volume create --from` doesn't exist yet — would need feature request or workaround

---

## Detailed Capability Audit

### Can `container` mount real overlayfs?

YES — inside the Linux VM. The VM kernel is real Linux. `mount -t overlay` works.

What `container` does NOT expose is the ability for the USER to specify overlayfs mounts as a container option. Docker supports `--mount type=overlay` (or does it? actually Docker doesn't either, you need to diy). Standard Docker run only supports bind mounts and named volumes.

So overlayfs would be managed **inside** the VM by a helper daemon, not by `container`'s CLI directly.

### Can `container` do volume snapshots / clone?

Currently unclear from docs. Volume operations shown:
- `container volume create foo`
- `container volume list`
- `container volume delete foo`
- `container volume prune`
- `container volume inspect foo` → shows backing image path

NO `clone` or `snapshot` command visible. This is the critical missing piece for Model C.

But! The backing image is a `.img` file on disk. `APFS` (Apple's filesystem) supports **CoW cloning** via `cp -c`. If the volume directory is on APFS, then `cp -c` the `.img` is effectively an O(1) snapshot.

```bash
# Assuming volumes stored at:
# /Users/<user>/Library/Application Support/com.apple.container/volumes/
# (This is APFS by definition — the system volume is APFS)

cp -c /path/to/vol-a.img /path/to/vol-b.img  # O(1) reference copy on APFS
```

So volume cloning is possible TODAY via APFS copy-on-write + `container`'s file layout. We just need to discover the volume image path from `container volume inspect`.

### Can `container` do VM template / warm pool?

Container machines are the closest thing. They're OCI-image + VM. When stopped, the VM freezes (or at minimum releases CPU). Starting a stopped machine is faster than creating from scratch.

For a warm pool:
1. Create N container machines from base image, don't start them
2. When agent needs workspace, pick one, start it, bind the volume, execute
3. After agent done, stop machine, optionally snapshot volume and clone

This doesn't match fa-serve's 44ms warm start (container machines likely take seconds to boot), but it's still faster than cold VM create.

---

## Integration Architecture: Model C + APFS Cloning

```
                        ┌─────────────┐
                        │   Mac host  │
                        │  (APFS fs)  │
                        └──────┬──────┘
                               │
    ┌──────────────┬───────────┼───────────┬──────────────┐
    │              │           │           │              │
┌───┴───┐    ┌───┴────┐ ┌───┴───┐ ┌────┴───┐  ┌────────┴────┐
│ Image │    │ Volume │ │Volume │ │ Volume │  │ Workspace   │
│ base  │    │ upper-a │ │upper-b│ │upper-c │  │ meta        │
│.img    │    │.img     │ │.img   │ │.img    │  │ (JSON)      │
└───┬───┘    └───┬────┘ └───┬───┘ └────┬───┘  └─────────────┘
    │              │          │          │
    │         ┌────┴────┐ ┌─┴────┐ ┌──┴────┐
    │         │ Machine │ │Machine│ │Machine│
    │         │ agent-a │ │agent-b│ │agent-c│
    │         │ stopped │ │running│ │stopped│
    │         └────┬────┘ └───┬──┘ └───┬───┘
    │              │          │        │
    └──────────────┴──────────┴────────┘
                  overlay inside VM
                  /workspace = upper (volume)
                    on top of /workspace-base (image)
```

Layer graph managed by `ws` (JSON metadata). Files stored by `container` (images + volumes). Fast clone via APFS.

---

## Implementation Path (if we pursue this)

### Phase 1: V1 using `container` as execution backend (Model A-lite)
- `ws` keeps current directory-based storage
- `ws run <ws> -- <cmd>` starts a `container` VM, bind-mounts workspace, runs command
- Gets real Linux execution on Mac
- Still O(n) copies, but correct behavior

### Phase 2: Discover volume cloning
- Parse `container volume inspect` output to find `.img` paths
- Implement fast workspace fork via `cp -c` on APFS
- Move workspace upperdirs into named volumes
- Layer metadata still in `ws`, files now in `container` volumes

### Phase 3: Container machines as WS backends
- Each workspace = a container machine (or a pooled machine + hot-swapped volume)
- OCI images as first-class `ws` layers
- `ws keep` exports to OCI image; `ws get` pulls from registry or local
- Full graph portability: push/pull between Mac and Linux

### Phase 4: Kata bridge
- Nested virtualization on M3+/macOS 15
- Container machine with custom kernel + CONFIG_KVM=y
- Kata images built and tested inside `container`
- Same workspace graph now deployable to both Mac dev and Linux prod

---

## Open Questions for Apple / `container` Community

1. **Will `container volume create --from=<vol>` be added?** This is the single feature that makes Model C seamless.
2. **Where exactly are volume `.img` files stored?** Docs show `/Users/fido/Library/Application Support/...` but is this stable across versions?
3. **Does APFS sparse clone (`cp -c`) work on the volume format?** `.img` files are typically single large files — perfect for APFS CoW.
4. **Can a container machine mount ANOTHER machine's volume (read-only)?** Needed for branch-from-workspace semantics.
5. **Is there a machine warm-pool API?** Or do we need to manage N pre-createdstopped machines?
6. **OCI layer export performance** — `container commit` to create a new image from a machine: how long for a 100MB workspace?

---

## Verdict

**For the immediate v1 `ws` CLI on macOS**: `container` is compelling as an execution backend (Model A) but doesn't solve the fundamental O(n) copy problem for workspace forks. The copy backend we have now is simpler and sufficient for V1.

**For the medium-term** (Phase 2-3): If we can implement fast volume cloning on APFS, `container` becomes THE right platform for `ws` on macOS. It gives us:
- Real overlayfs semantics (inside VMs)
- Content-addressed OCI layer graph (portable)
- Named volume snapshots (fast fork/branch)
- A migration path to Linux (same OCI images)

**For the Kata/think roadmap**: `container`'s nested virt is the missing link. Being able to test Kata sandbox images on a Mac before pushing to the think cluster is a huge dev velocity win.

**Recommended next action**: Build a 50-line test script that creates a `container` volume, finds the `.img` path, attempts `cp -c` clone, and verifies the cloned volume mounts. This answers the critical Phase 2 feasibility question in 10 minutes.

---

*This analysis document is part of the `ws` workspace graph project. See README.md for CLI usage and CHANGELOG.md for version history.*
