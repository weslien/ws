//go:build darwin

package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// In-VM execution + overlayfs for the Apple container backend (issue #1).
//
// Modes (recorded in ~/.ws/meta/container/<name>.mode):
//
//	overlay  fork = machine boot only (no layer copy). The overlay is
//	         mounted lazily inside the VM on first use: lower = the layer
//	         (virtiofs, read-only), upper/work = VM-local disk
//	         (/var/lib/ws), merged view at the virtiofs workspace path.
//	         Execution, keep, diff and status all operate in-VM against
//	         the merged view. The HOST never reads workspace content.
//	shared   fallback (also used for pre-overlay workspaces): the
//	         workspace directory is the content itself, visible on both
//	         host and VM. Fork stages the layer with an in-VM tar copy.
//
// If the overlay mount fails (or the VM lacks overlayfs), the workspace
// transparently degrades: the layer is staged host-side (fast, no VM
// needed) and the mode file is rewritten to 'shared' — never a data
// loss, just slower forks.
//
// All in-VM scripts go through runScript, which handles the argv-joining
// behavior of 'container machine run' (see container_darwin.go).

const (
	vmStateDir = "/var/lib/ws" // VM-local persistent state (upper/work)
)

// modeFile is the backend-owned per-workspace mode record.
func (b *ContainerBackend) modeFile(name string) string {
	return filepath.Join(b.root, "meta", "container", name+".mode")
}

// readMode returns ("overlay", layerHash) or ("shared", "") — the empty
// hash means "content is the workspace dir itself; layer is formed-from".
func (b *ContainerBackend) readMode(name string) (string, string) {
	data, err := os.ReadFile(b.modeFile(name))
	if err != nil {
		return "shared", "" // pre-overlay workspaces and unknown = shared
	}
	fields := strings.Fields(string(data))
	if len(fields) == 2 && fields[0] == "overlay" {
		return "overlay", fields[1]
	}
	return "shared", ""
}

func (b *ContainerBackend) writeMode(name, mode, layerHash string) error {
	dir := filepath.Dir(b.modeFile(name))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(b.modeFile(name), []byte(mode+" "+layerHash+"\n"), 0644)
}

// ensureMachine creates the workspace's machine if it is missing (e.g.
// after a host reboot or a container-runtime reset).
func (b *ContainerBackend) ensureMachine(name string, logger OperationLogger) error {
	mName := b.machineName(name)
	if b.machineExists(mName) {
		return nil
	}
	if logger != nil {
		logger.Log("(re)creating container machine %s...", mName)
	}
	out, err := exec.Command("container", "machine", "create", "alpine:latest", "--name", mName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("machine create %s: %w\n%s", mName, err, out)
	}
	return nil
}

// mountedInVM reports whether the workspace path is a mountpoint in the VM.
func (b *ContainerBackend) mountedInVM(mName, wsInVM string) bool {
	check := fmt.Sprintf("grep -qs ' %s ' /proc/mounts", quotePOSIX(wsInVM))
	out, err := b.machineRunOutput(mName, "sh", "-c", b.scriptArg(mName, check))
	_ = out
	return err == nil
}

// ensureOverlay makes the workspace's merged view available in-VM. For
// shared mode it is a no-op. For overlay mode it mounts the overlay if
// needed, falling back to shared (host-side staging) when mounting fails.
func (b *ContainerBackend) ensureOverlay(name string, logger OperationLogger) error {
	mode, layerHash := b.readMode(name)
	if mode != "overlay" {
		return nil
	}
	mName := b.machineName(name)
	wsInVM := b.hostPath(filepath.Join(b.workspacesDir(), name))

	if err := b.ensureMachine(name, logger); err != nil {
		return b.fallbackToShared(name, layerHash, err, logger)
	}
	if b.mountedInVM(mName, wsInVM) {
		return nil // already mounted
	}

	layerInVM := b.hostPath(filepath.Join(b.layersDir(), layerHash))
	upper := filepath.Join(vmStateDir, name, "upper")
	work := filepath.Join(vmStateDir, name, "work")
	// The merged view's root takes ownership from upperdir, and everything
	// in it is written by the user `machine run` uses (the host-matching
	// user) — so upper/work must be owned by that user, not root. Derive
	// the uid:gid from the virtiofs workspace dir (the identity host files
	// already present inside the VM) instead of guessing the host uid.
	mountCmd := fmt.Sprintf(
		"mkdir -p %s %s %s && owner=$(stat -c '%%u:%%g' %s) && chown -R $owner %s %s && mount -t overlay overlay -o lowerdir=%s,upperdir=%s,workdir=%s %s",
		quotePOSIX(upper), quotePOSIX(work), quotePOSIX(wsInVM),
		quotePOSIX(wsInVM),
		quotePOSIX(upper), quotePOSIX(work),
		quotePOSIX(layerInVM), quotePOSIX(upper), quotePOSIX(work), quotePOSIX(wsInVM))
	if err := b.runScript(mName, true, mountCmd, nil); err != nil {
		return b.fallbackToShared(name, layerHash, err, logger)
	}
	if logger != nil {
		logger.Log("overlayfs active in machine %s", mName)
	}
	return nil
}

// fallbackToShared degrades an overlay workspace to shared-dir mode:
// stage the layer host-side (no VM required) and record the mode switch.
func (b *ContainerBackend) fallbackToShared(name, layerHash string, cause error, logger OperationLogger) error {
	if logger != nil {
		logger.Log("overlay unavailable for %s (%v) — falling back to shared-dir mode", name, cause)
	}
	layerDir := filepath.Join(b.layersDir(), layerHash)
	wsDir := filepath.Join(b.workspacesDir(), name)
	if err := copyDir(layerDir, wsDir); err != nil {
		return fmt.Errorf("fallback staging for %s: %w", name, err)
	}
	if err := b.writeMode(name, "shared", ""); err != nil {
		return err
	}
	return nil
}

// Exec routes a command inside the workspace's container machine (the
// optional Execer interface used by 'ws run'). Both modes execute at the
// same in-VM path — overlay mode sees the merged view, shared mode sees
// the shared directory.
func (b *ContainerBackend) Exec(name string, argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("exec: empty command")
	}
	if err := b.ensureOverlay(name, nil); err != nil {
		return err
	}
	mName := b.machineName(name)
	wsInVM := b.hostPath(filepath.Join(b.workspacesDir(), name))

	quoted := make([]string, len(argv))
	for i, a := range argv {
		quoted[i] = quotePOSIX(a)
	}
	script := fmt.Sprintf("cd %s && exec %s", quotePOSIX(wsInVM), strings.Join(quoted, " "))

	// runScriptInteractive: TTY forwarding when interactive; WS_VM_ROOT=1
	// runs as root; propagates the in-VM exit code.
	return b.runScriptInteractive(mName, script)
}

func (b *ContainerBackend) runScriptInteractive(mName string, script string) error {
	args := []string{"sh", "-c", script}
	if b.argvJoinProbe(mName) {
		args = []string{"sh", "-c", quotePOSIX(script)}
	}
	base := []string{"machine", "run", "-n", mName}
	if os.Getenv("WS_VM_ROOT") == "1" {
		base = append(base, "--root")
	}
	if isCharDevice(os.Stdin) && isCharDevice(os.Stdout) {
		base = append(base, "-t")
	}
	base = append(base, "--")
	full := append(append([]string{}, base...), args...)
	if os.Getenv("WS_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[ws debug] container %s\n", strings.Join(full, " "))
	}
	cmd := exec.Command("container", full...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func isCharDevice(f *os.File) bool {
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

// HashWorkspace produces a hash of the workspace CONTENT comparable to
// LayerHash. The workspace is exported in-VM to a host staging dir and
// hashed with the standard algorithm. Falls back to the shared-dir host
// hash when no machine interaction is possible.
func (b *ContainerBackend) HashWorkspace(name string) string {
	mode, _ := b.readMode(name)
	if mode != "overlay" {
		// Shared mode: the host sees the content directly.
		return NewCopyBackend().LayerHash(filepath.Join(b.workspacesDir(), name))
	}
	if err := b.ensureOverlay(name, nil); err != nil {
		return "?"
	}
	mName := b.machineName(name)
	wsInVM := b.hostPath(filepath.Join(b.workspacesDir(), name))
	stage, err := os.MkdirTemp(b.tmpDir(), "hash-*")
	if err != nil {
		return "?"
	}
	defer os.RemoveAll(stage)
	export := fmt.Sprintf("tar -C %s -cf - . | tar -C %s -xf -",
		quotePOSIX(wsInVM), quotePOSIX(b.hostPath(stage)))
	if err := b.runScript(mName, false, export, nil); err != nil {
		return "?"
	}
	return NewCopyBackend().LayerHash(stage)
}

// ExportWorkspace copies the workspace content to an external directory
// (the optional Exporter interface used by 'ws export'). Overlay mode
// exports the merged view via staging; shared mode is a host-side copy.
func (b *ContainerBackend) ExportWorkspace(name string, dest string, logger OperationLogger) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	mode, _ := b.readMode(name)
	if mode != "overlay" {
		return copyDir(filepath.Join(b.workspacesDir(), name), dest)
	}
	if err := b.ensureOverlay(name, logger); err != nil {
		return err
	}
	mName := b.machineName(name)
	wsInVM := b.hostPath(filepath.Join(b.workspacesDir(), name))
	stage, err := os.MkdirTemp(b.tmpDir(), "export-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	export := fmt.Sprintf("tar -C %s -cf - . | tar -C %s -xf -",
		quotePOSIX(wsInVM), quotePOSIX(b.hostPath(stage)))
	if err := b.runScript(mName, false, export, nil); err != nil {
		return err
	}
	return copyDir(stage, dest)
}
