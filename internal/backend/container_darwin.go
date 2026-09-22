//go:build darwin

package backend

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ContainerBackend uses Apple's 'container' CLI (github.com/apple/container)
// to run Linux VMs with real overlayfs on macOS.
//
// Architecture:
//   - Each workspace = one persistent 'container machine'.
//   - The machine auto-mounts the host home directory, so ~/.ws is visible inside.
//   - On Fork the layer is copied into the machine's workspace area, then
//     an overlayfs mount is attempted (layer=lower, machine-local upper).
//   - If overlayfs mount fails the backend falls back to a plain directory copy.
//
// EXPERIMENTAL: Verified against 'container' CLI documentation only. Live
// behaviour on macOS may differ. Failed attempts report the exact command
// and stderr so issues can be filed with precise reproduction.
type ContainerBackend struct {
	root string
	user string // host username, needed for paths inside the VM
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
	for _, d := range []string{"layers", "workspaces", "uppers", "workdirs", "meta"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			return err
		}
	}
	return nil
}

func (b *ContainerBackend) layersDir() string      { return filepath.Join(b.root, "layers") }
func (b *ContainerBackend) workspacesDir() string   { return filepath.Join(b.root, "workspaces") }
func (b *ContainerBackend) uppersDir() string       { return filepath.Join(b.root, "uppers") }
func (b *ContainerBackend) workdirsDir() string     { return filepath.Join(b.root, "workdirs") }

func (b *ContainerBackend) machineName(ws string) string { return "ws-" + sanitizeContainerName(ws) }

// hostPath converts a host absolute path to the path visible inside the
// container machine (virtiofs home mount).
func (b *ContainerBackend) hostPath(p string) string {
	return filepath.Join("/Users", b.user, strings.TrimPrefix(p, os.Getenv("HOME")))
}

// Fork creates a new workspace by instantiating a container machine and
// setting up overlayfs inside it.
func (b *ContainerBackend) Fork(srcHash string, dstName string, logger OperationLogger) error {
	layerDir := filepath.Join(b.layersDir(), srcHash)
	wsDir := filepath.Join(b.workspacesDir(), dstName)
	upperDir := filepath.Join(b.uppersDir(), dstName)
	workDir := filepath.Join(b.workdirsDir(), dstName)

	for _, d := range []string{wsDir, upperDir, workDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	mName := b.machineName(dstName)

	// Remove any previous machine with this name.
	// 'container machine rm -f' often needs a moment for background cleanup.
	exec.Command("container", "machine", "rm", "-f", mName).Run()
	time.Sleep(2 * time.Second)

	// Create the machine from a lightweight image.
	// The image must have 'mount' and a real kernel (any Linux image works).
	logger.Log("creating container machine %s...", mName)
	cmd := exec.Command("container", "machine", "create", "alpine:latest", "--name", mName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Stale machine from interrupted prior run: try cleanup + retry once.
		if strings.Contains(string(out), "already exists") {
			logger.Log("machine %s still exists, retrying cleanup...", mName)
			exec.Command("container", "machine", "rm", "-f", mName).Run()
			time.Sleep(3 * time.Second)
			cmd = exec.Command("container", "machine", "create", "alpine:latest", "--name", mName)
			out, err = cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("container machine create (retry): %w\nstderr: %s\n\nTo manually clean up: container machine rm -f %s", err, out, mName)
			}
		} else {
			return fmt.Errorf("container machine create: %w\nstderr: %s", err, out)
		}
	}

	// Paths inside the VM (home directory is auto-mounted by container).
	layerInVM := b.hostPath(layerDir)
	wsInVM := b.hostPath(wsDir)
	upperInVM := b.hostPath(upperDir)
	workInVM := b.hostPath(workDir)

	// Stage 1: copy the layer into the workspace area inside the VM.
	// We do this first so we have a fallback if overlay mount fails.
	logger.Log("copying layer into workspace %s...", dstName)
	setup := fmt.Sprintf("tar -C %s -cf - . | tar -C %s -xf -", layerInVM, wsInVM)
	if err := b.machineRun(mName, "sh", "-c", setup); err != nil {
		exec.Command("container", "machine", "rm", "-f", mName).Run()
		return fmt.Errorf("initial copy into workspace: %w", err)
	}

	// Stage 2: attempt overlayfs mount.
	// This requires root inside the VM. container machines run as the
	// matching host user by default, so we try 'sudo mount' first.
	logger.Log("attempting overlayfs mount in %s...", mName)
	mountCmd := fmt.Sprintf("mkdir -p %s %s && sudo mount -t overlay overlay -o lowerdir=%s,upperdir=%s,workdir=%s %s",
		upperInVM, workInVM, layerInVM, upperInVM, workInVM, wsInVM)
	if err := b.machineRun(mName, "sh", "-c", mountCmd); err != nil {
		logger.Log("overlay mount failed: %v (falling back to plain copy)", err)
		// Fallback: workspace is already the copied directory from Stage 1.
		return nil
	}

	logger.Log("overlayfs active in machine %s", mName)
	return nil
}

// ForkFromWorkspace snapshots the source workspace state into a new layer,
// then Forks from that layer.
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

// Mount is a no-op because the machine itself is the mounted environment.
func (b *ContainerBackend) Mount(string, string, OperationLogger) error { return nil }

// Unmount destroys the container machine.
func (b *ContainerBackend) Unmount(name string, _ OperationLogger) error {
	mName := b.machineName(name)
	exec.Command("container", "machine", "rm", "-f", mName).Run()
	return nil
}

// Destroy removes the workspace's machine, upper, work, and mount dirs.
func (b *ContainerBackend) Destroy(name string, log OperationLogger) error {
	b.Unmount(name, log)
	for _, d := range []string{b.workspacesDir(), b.uppersDir(), b.workdirsDir()} {
		os.RemoveAll(filepath.Join(d, name))
	}
	return nil
}

// Commit hashes the workspace content inside the VM and copies it to a host
// layer directory.
func (b *ContainerBackend) Commit(name string, logger OperationLogger) (string, error) {
	mName := b.machineName(name)
	wsInVM := b.hostPath(filepath.Join(b.workspacesDir(), name))

	// We compute the hash by copying the workspace out to a temp dir on the
	// host and using the standard hashing.  In future this could be done
	// entirely inside the VM with a shell script.
	tmpDir, err := os.MkdirTemp("", "ws-commit-*")
	if err != nil {
		return "", fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	logger.Log("exporting workspace %s from machine %s...", name, mName)
	if err := b.machineRun(mName, "sh", "-c", fmt.Sprintf("tar -C %s -cf - . | tar -C %s -xf -", wsInVM, b.hostPath(tmpDir))); err != nil {
		return "", fmt.Errorf("export workspace from machine: %w", err)
	}

	hash := b.LayerHash(tmpDir)
	layerDir := filepath.Join(b.layersDir(), hash)
	if _, err := os.Stat(layerDir); os.IsNotExist(err) {
		if err := copyDir(tmpDir, layerDir); err != nil {
			return "", err
		}
	}
	return hash, nil
}

func (b *ContainerBackend) LayerHash(dir string) string {
	return NewCopyBackend().LayerHash(dir)
}

// Diff runs diff inside the machine via a single shell command and writes
// output to w.
func (b *ContainerBackend) Diff(wsA string, wsB string, layerB string, w io.Writer, _ OperationLogger) error {
	mName := b.machineName(wsA)
	wsAInVM := b.hostPath(filepath.Join(b.workspacesDir(), wsA))

	var targetInVM string
	if wsB != "" {
		targetInVM = b.hostPath(filepath.Join(b.workspacesDir(), wsB))
	} else {
		targetInVM = b.hostPath(filepath.Join(b.layersDir(), layerB))
	}

	cmd := exec.Command("container", "machine", "run", "-n", mName, "diff", "-ruN", targetInVM, wsAInVM)
	out, err := cmd.CombinedOutput()
	w.Write(out)
	if err != nil && len(out) == 0 {
		return fmt.Errorf("diff in machine %s: %w", mName, err)
	}
	return nil
}

// machineRun is a helper that runs a command inside a container machine.
func (b *ContainerBackend) machineRun(name string, args ...string) error {
	cmdArgs := append([]string{"machine", "run", "-n", name}, args...)
	cmd := exec.Command("container", cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
