//go:build darwin

package backend

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ContainerBackend uses Apple's 'container' CLI (github.com/apple/container)
// to run Linux VMs with real overlayfs on macOS.
//
// Architecture:
//   - Each workspace = one persistent 'container machine'.
//   - The machine auto-mounts the host home directory (default: rw), so ~/.ws
//     is visible inside the VM at /Users/<user>/.ws.
//   - On Fork the layer is copied into the machine's workspace area, then
//     an overlayfs mount is attempted at Mount() time (layer=lower; upper on
//     the shared ~/.ws so host writes and VM reads agree).
//   - If the overlayfs mount fails the workspace silently stays a plain
//     copy — degraded, but host and VM remain consistent.
//
// Exec semantics: 'container machine run' does not necessarily preserve
// argv. Observed on container CLI 1.x: it joins the positional arguments
// into one command line which the machine's shell then re-parses. A naive
// ["sh","-c",script] therefore arrives as `sh -c tar ...` — the inner sh -c
// receives only the first word, reproducing the BusyBox-usage-dump +
// "tar: short read" failure reported on macOS. runScript probes the
// semantics once and passes quotePOSIX(script) when joined, so the outer
// shell parse yields exactly: sh -c '<script>'
//
// CLI reference: https://github.com/apple/container/blob/main/docs/command-reference.md
type ContainerBackend struct {
	root string
	user string // host username, needed for paths inside the VM

	probeMu    sync.Mutex
	probeDone  bool
	argvJoined bool // true = machine run joins positionals into one shell line
}

func NewContainerBackend() *ContainerBackend {
	u := os.Getenv("USER")
	if u == "" {
		u = "user"
	}
	return &ContainerBackend{user: u}
}

func (b *ContainerBackend) Name() string { return "container" }

func (b *ContainerBackend) Init(root string) error {
	if !hasContainerCLI() {
		return fmt.Errorf("'container' CLI not found in PATH; install from https://github.com/apple/container")
	}
	b.root = root
	for _, d := range []string{"layers", "workspaces", "uppers", "workdirs", "meta", "tmp"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			return err
		}
	}
	return nil
}

func (b *ContainerBackend) layersDir() string     { return filepath.Join(b.root, "layers") }
func (b *ContainerBackend) workspacesDir() string { return filepath.Join(b.root, "workspaces") }
func (b *ContainerBackend) uppersDir() string     { return filepath.Join(b.root, "uppers") }
func (b *ContainerBackend) workdirsDir() string   { return filepath.Join(b.root, "workdirs") }
func (b *ContainerBackend) tmpDir() string        { return filepath.Join(b.root, "tmp") }

func (b *ContainerBackend) machineName(ws string) string { return "ws-" + sanitizeContainerName(ws) }

// hostPath converts a host absolute path to the path visible inside the
// container machine (virtiofs home mount). Paths outside the host user's
// home are NOT visible inside the VM — always stage under ~/.ws, not /tmp.
func (b *ContainerBackend) hostPath(p string) string {
	return filepath.Join("/Users", b.user, strings.TrimPrefix(p, os.Getenv("HOME")))
}

// machineDelete removes a container machine by name.
// Uses 'container machine delete' (the real API) — NOT 'rm -f'.
// Stops the machine first if it's running.
func (b *ContainerBackend) machineDelete(mName string) {
	exec.Command("container", "machine", "stop", mName).Run()
	time.Sleep(500 * time.Millisecond)
	exec.Command("container", "machine", "delete", mName).Run()
}

// argvJoinProbe distinguishes the two possible 'machine run' exec semantics:
//
//	argv preserved:  ['sh','-c','printf %s wsargvok'] -> prints "wsargvok"
//	argv joined:    the machine runs -> sh -c printf %s wsargvok
//	                i.e. `printf` with $0=%s $1=wsargvok -> prints nothing
//
// The result is cached for the lifetime of the process.
func (b *ContainerBackend) argvJoinProbe(mName string) bool {
	b.probeMu.Lock()
	defer b.probeMu.Unlock()
	if !b.probeDone {
		out, err := b.machineRunOutput(mName, "sh", "-c", "printf %s wsargvok")
		b.probeDone = true
		b.argvJoined = !(err == nil && strings.TrimSpace(out) == "wsargvok")
	}
	return b.argvJoined
}

// runScript executes script inside the machine as one shell script, adapting
// to the exec semantics detected by argvJoinProbe:
//
//	argv joined: pass ['sh','-c', quotePOSIX(script)] — after the CLI joins
//	             and the machine shell re-parses, inner sh -c gets <script>
//	             as ONE argument.
//	argv kept:   pass ['sh','-c', script] directly.
//
// The '--' separator ends the CLI's flag parsing, so a script that begins
// with '-' can never be eaten as an option. WS_DEBUG=1 prints the full
// machine-run invocation for troubleshooting.
func (b *ContainerBackend) runScript(mName string, root bool, script string, stdin *os.File) error {
	args := []string{"sh", "-c", script}
	if b.argvJoinProbe(mName) {
		args = []string{"sh", "-c", quotePOSIX(script)}
	}
	base := []string{"machine", "run", "-n", mName}
	if root {
		base = append(base, "--root")
	}
	base = append(base, "--")
	full := append(append([]string{}, base...), args...)
	if os.Getenv("WS_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[ws debug] container %s\n", strings.Join(full, " "))
	} else {
		traceFirstRun.Do(func() {
			fmt.Fprintf(os.Stderr, "[ws debug] container %s\n", strings.Join(full, " "))
		})
	}
	cmd := exec.Command("container", full...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil && os.Getenv("WS_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[ws debug] machine run failed (exit=%v). Re-run manually:\n  container %s\n", err, strings.Join(full, " "))
	}
	return err
}

// traceFirstRun prints the FIRST machine-run invocation of the process to
// stderr, always — so a failure trace (like the original BusyBox usage
// dump) always shows at least one full command line without the user
// needing to know about WS_DEBUG. WS_DEBUG=1 traces every invocation.
var traceFirstRun sync.Once

// machineRunOutput runs a command capturing combined output. Used by the
// argv probe; user-facing commands stream via runScript.
func (b *ContainerBackend) machineRunOutput(mName string, args ...string) (string, error) {
	cmdArgs := append([]string{"machine", "run", "-n", mName}, args...)
	cmd := exec.Command("container", cmdArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// machineExists reports whether the workspace's machine is present.
func (b *ContainerBackend) machineExists(mName string) bool {
	out, err := exec.Command("container", "machine", "list", "--format", "json").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "\""+mName+"\"")
}

// Fork creates a new workspace by instantiating a container machine.
//
// New workspaces default to OVERLAY mode (issue #1): no content staging
// at all — the layer is mounted read-only as the overlay lowerdir lazily
// on first use; writes go to VM-local disk; ws run executes INSIDE the
// machine against the merged view. Fork cost = machine boot.
//
// If the overlay mount fails on first use, the workspace transparently
// degrades to shared-dir mode: the layer is staged host-side (no VM) and
// everything behaves like the old backend, with the mode recorded in
// ~/.ws/meta/container/<name>.mode. Pre-overlay workspaces read as
// shared and keep working unchanged.
//
//	Default mode: overlay. WS_CONTAINER_MODE=shared forces legacy
//	shared-dir forks (content staged in-VM; the host can read the
//	workspace directory directly).
func (b *ContainerBackend) Fork(srcHash string, dstName string, logger OperationLogger) error {
	wsDir := filepath.Join(b.workspacesDir(), dstName)
	upperDir := filepath.Join(b.uppersDir(), dstName)
	workDir := filepath.Join(b.workdirsDir(), dstName)

	for _, d := range []string{wsDir, upperDir, workDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	mName := b.machineName(dstName)

	// Clean up any previous machine with this name.
	b.machineDelete(mName)
	time.Sleep(1 * time.Second)

	// Create the machine from a lightweight image.
	// alpine:latest has tar, mount, and a real Linux kernel.
	logger.Log("creating container machine %s...", mName)
	cmd := exec.Command("container", "machine", "create", "alpine:latest", "--name", mName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "already exists") {
			logger.Log("machine %s still exists, retrying cleanup...", mName)
			b.machineDelete(mName)
			time.Sleep(2 * time.Second)
			cmd = exec.Command("container", "machine", "create", "alpine:latest", "--name", mName)
			out, err = cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("container machine create (retry): %w\nstderr: %s\n\nTo manually clean up: container machine delete %s", err, out, mName)
			}
		} else {
			return fmt.Errorf("container machine create: %w\nstderr: %s", err, out)
		}
	}

	if os.Getenv("WS_CONTAINER_MODE") == "shared" {
		// Legacy shared-dir fork: stage the content in-VM. The host can
		// read the workspace directory directly (compatibility mode).
		if err := b.writeMode(dstName, "shared", ""); err != nil {
			return err
		}
		return b.forkShared(mName, srcHash, dstName, logger)
	}

	// Overlay mode: record and boot; no content copied. The overlay mount
	// happens lazily at first use (ensureOverlay) so a mount failure only
	// costs a host-side copyDir — not a failed fork.
	if err := b.writeMode(dstName, "overlay", srcHash); err != nil {
		return err
	}
	logger.Log("workspace %s in overlay mode (layer %s mounted read-only in-VM)", dstName, shortHash(srcHash))
	return nil
}

// forkShared stages the layer content into the shared workspace directory
// via an in-VM copy (legacy behavior).
func (b *ContainerBackend) forkShared(mName, srcHash, dstName string, logger OperationLogger) error {
	layerInVM := b.hostPath(filepath.Join(b.layersDir(), srcHash))
	wsInVM := b.hostPath(filepath.Join(b.workspacesDir(), dstName))
	logger.Log("copying layer into workspace %s...", dstName)
	setup := fmt.Sprintf("tar -C %s -cf - . | tar -C %s -xf -",
		quotePOSIX(layerInVM), quotePOSIX(wsInVM))
	if err := b.runScript(mName, false, setup, nil); err != nil {
		b.machineDelete(mName)
		return fmt.Errorf("initial copy into workspace: %w", err)
	}
	return nil
}

func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}

// ForkFromWorkspace snapshots the source workspace state into a new layer,
// then forks from that layer.
func (b *ContainerBackend) ForkFromWorkspace(srcName string, dstName string, logger OperationLogger) (string, error) {
	logger.Log("snapshotting workspace %s...", srcName)
	snapHash, err := b.Commit(srcName, logger)
	if err != nil {
		return "", fmt.Errorf("snapshot workspace %q: %w", srcName, err)
	}
	if err := b.Fork(snapHash, dstName, logger); err != nil {
		return "", err
	}
	return snapHash, nil
}

// CloneGitRepo clones a repo into a temporary directory, hashes it, stores
// the layer on the host, and returns the hash.
func (b *ContainerBackend) CloneGitRepo(repo string, ref string, logger OperationLogger) (string, string, error) {
	cb := NewCopyBackend()
	cb.Init(b.root)
	return cb.CloneGitRepo(repo, ref, logger)
}

// Mount is a no-op. The workspace is a shared directory (visible to both
// host and VM); mounting an overlay VM-side would shadow host writes and
// make ws keep silently drop them (see Fork). Overlay support returns with
// in-VM execution routing.
func (b *ContainerBackend) Mount(string, string, OperationLogger) error { return nil }

// Unmount is a no-op (nothing is mounted). The machine's lifecycle is
// managed by Destroy.
func (b *ContainerBackend) Unmount(string, OperationLogger) error { return nil }

// Destroy removes the workspace's machine and its host-side directories,
// including the mode record.
func (b *ContainerBackend) Destroy(name string, log OperationLogger) error {
	b.Unmount(name, log)
	b.machineDelete(b.machineName(name))
	for _, d := range []string{b.workspacesDir(), b.uppersDir(), b.workdirsDir()} {
		os.RemoveAll(filepath.Join(d, name))
	}
	os.Remove(b.modeFile(name))
	return nil
}

// Commit hashes the workspace content and copies it to a host layer.
// Overlay workspaces are mounted first (an unused overlay workspace has an
// empty virtiofs view; the merged view only exists once mounted in-VM).
// The export goes through a staging directory under ~/.ws/tmp, which IS
// visible inside the VM via the home mount (the historical /tmp staging
// silently produced empty layers — see CHANGELOG 0.9.1).
func (b *ContainerBackend) Commit(name string, logger OperationLogger) (string, error) {
	if err := b.ensureOverlay(name, logger); err != nil {
		return "", err
	}
	mName := b.machineName(name)
	wsInVM := b.hostPath(filepath.Join(b.workspacesDir(), name))

	stage, err := os.MkdirTemp(b.tmpDir(), "commit-*")
	if err != nil {
		return "", fmt.Errorf("staging dir: %w", err)
	}
	defer os.RemoveAll(stage)

	stageInVM := b.hostPath(stage)
	logger.Log("exporting workspace %s from machine %s...", name, mName)
	export := fmt.Sprintf("tar -C %s -cf - . | tar -C %s -xf -",
		quotePOSIX(wsInVM), quotePOSIX(stageInVM))
	if err := b.runScript(mName, false, export, nil); err != nil {
		return "", fmt.Errorf("export workspace from machine: %w", err)
	}

	hash := b.LayerHash(stage)
	layerDir := filepath.Join(b.layersDir(), hash)
	if _, err := os.Stat(layerDir); os.IsNotExist(err) {
		if err := copyDir(stage, layerDir); err != nil {
			return "", err
		}
	}
	return hash, nil
}

func (b *ContainerBackend) LayerHash(dir string) string {
	return NewCopyBackend().LayerHash(dir)
}

// Diff writes the unified diff between the workspace and a target (another
// workspace or a layer) by running diff inside the machine.
func (b *ContainerBackend) Diff(wsA string, wsB string, layerB string, w io.Writer, _ OperationLogger) error {
	mName := b.machineName(wsA)
	wsAInVM := b.hostPath(filepath.Join(b.workspacesDir(), wsA))

	var targetInVM string
	if wsB != "" {
		targetInVM = b.hostPath(filepath.Join(b.workspacesDir(), wsB))
	} else {
		targetInVM = b.hostPath(filepath.Join(b.layersDir(), layerB))
	}

	script := fmt.Sprintf("diff -ruN %s %s", quotePOSIX(targetInVM), quotePOSIX(wsAInVM))
	out, err := b.machineRunOutput(mName, "sh", "-c", b.scriptArg(mName, script))
	if err != nil && len(out) == 0 {
		return fmt.Errorf("diff in machine %s: %w", mName, err)
	}
	w.Write([]byte(out))
	return nil
}

// scriptArg returns the inner sh -c argument adapted to the machine's exec
// semantics: the raw script if argv is preserved, quotePOSIX(script) if the
// CLI joins positionals (see runScript).
func (b *ContainerBackend) scriptArg(mName string, script string) string {
	if b.argvJoinProbe(mName) {
		return quotePOSIX(script)
	}
	return script
}

// sanitizeContainerName makes a string safe for use as a container machine
// name (alphanumeric, hyphens, underscores only).
func sanitizeContainerName(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
}
