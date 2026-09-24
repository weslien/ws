//go:build linux

package backend

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// ---------- OverlayfsBackend (Linux only) ----------
//
// Structure under root:
//   layers/<hash>/
//   workspaces/<name>/       -- mount point (MOUNT)
//   workdirs/<name>/work    -- overlayfs work directory
//   uppers/<name>/          -- overlayfs upper (writable)
//
// Each workspace is an overlay mount: lower=layers/<hash>, upper=uppers/<name>, work=workdirs/<name>/work
//
// Forking a workspace materializes the upper into a new layer (keep) or copies it
// (first draft: keep copies upper, new ws from layer mounts overlay fresh).

type OverlayfsBackend struct {
	root string
}

func NewOverlayfsBackend() *OverlayfsBackend { return &OverlayfsBackend{} }
func (b *OverlayfsBackend) Name() string      { return "overlayfs" }

func (b *OverlayfsBackend) Init(root string) error {
	b.root = root
	for _, d := range []string{"layers", "workspaces", "workdirs", "uppers", "meta"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			return err
		}
	}
	return nil
}

func (b *OverlayfsBackend) layersDir() string      { return filepath.Join(b.root, "layers") }
func (b *OverlayfsBackend) workspacesDir() string    { return filepath.Join(b.root, "workspaces") }
func (b *OverlayfsBackend) uppersDir() string       { return filepath.Join(b.root, "uppers") }
func (b *OverlayfsBackend) workdirsDir() string     { return filepath.Join(b.root, "workdirs") }

func (b *OverlayfsBackend) Fork(srcHash string, dstName string, logger OperationLogger) error {
	upper := filepath.Join(b.uppersDir(), dstName)
	workdir := filepath.Join(b.workdirsDir(), dstName, "work")
	mountPoint := filepath.Join(b.workspacesDir(), dstName)
	lower := filepath.Join(b.layersDir(), srcHash)

	for _, d := range []string{mountPoint, upper, workdir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return b.mount(lower, upper, workdir, mountPoint)
}

func (b *OverlayfsBackend) ForkFromWorkspace(srcName string, dstName string, logger OperationLogger) (string, error) {
	// To branch from a live workspace we must snapshot it first — otherwise
	// the source's mutable upper could race with the mount we build.
	logger.Log("snapshotting workspace %s...", srcName)
	snapHash, err := b.Commit(srcName, logger)
	if err != nil {
		return "", fmt.Errorf("snapshot workspace %q: %w", srcName, err)
	}
	// Now fork from that snapshot (immutable)
	if err := b.Fork(snapHash, dstName, logger); err != nil {
		return "", err
	}
	return snapHash, nil
}

func (b *OverlayfsBackend) CloneGitRepo(repo string, ref string, logger OperationLogger) (string, string, error) {
	// Reuse the generic git clone from copy backend behavior — after clone
	// store in layers/ as a regular directory, then we can mount from it.
	cb := NewCopyBackend()
	cb.Init(b.root) // sets root on copy backend
	// Actually, copy backend doesn't have its root set by Init because we
	// changed it to take a param. Let's just do it inline.
	return b.cloneGitRepo(repo, ref, logger)
}

func (b *OverlayfsBackend) cloneGitRepo(repo, ref string, logger OperationLogger) (string, string, error) {
	tmpDir, err := os.MkdirTemp("", "ws-clone-*")
	if err != nil {
		return "", "", fmt.Errorf("temp dir: %w", err)
	}
	logger.Log("cloning %s@%s...", repo, ref)
	cmd := exec.Command("git", "clone", "--depth=1")
	if ref != "" && ref != "HEAD" {
		cmd.Args = append(cmd.Args, "--branch="+ref)
	}
	cmd.Args = append(cmd.Args, repo, tmpDir)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("git", "clone", "--depth=1", repo, tmpDir)
		cmd.Stderr = os.Stderr
		if err2 := cmd.Run(); err2 != nil {
			os.RemoveAll(tmpDir)
			return "", "", fmt.Errorf("git clone failed: %w", err)
		}
		if ref != "" && ref != "HEAD" {
			exec.Command("git", "-C", tmpDir, "checkout", ref).Run()
		}
	}
	hash := b.LayerHash(tmpDir)
	layerDir := filepath.Join(b.layersDir(), hash)
	if _, err := os.Stat(layerDir); os.IsNotExist(err) {
		if err := copyDir(tmpDir, layerDir); err != nil {
			os.RemoveAll(tmpDir)
			return "", "", err
		}
	}
	return hash, tmpDir, nil
}

func (b *OverlayfsBackend) Mount(name string, layerHash string, logger OperationLogger) error {
	return nil // Fork already mounts
}
func (b *OverlayfsBackend) Unmount(name string, logger OperationLogger) error {
	mp := filepath.Join(b.workspacesDir(), name)
	if output, err := exec.Command("umount", "-l", mp).CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "not mounted") {
			return fmt.Errorf("umount: %w: %s", err, output)
		}
	}
	return nil
}
func (b *OverlayfsBackend) Destroy(name string, logger OperationLogger) error {
	_ = b.Unmount(name, logger)
	for _, d := range []string{
		filepath.Join(b.workspacesDir(), name),
		filepath.Join(b.uppersDir(), name),
		filepath.Join(b.workdirsDir(), name),
	} {
		os.RemoveAll(d)
	}
	return nil
}

func (b *OverlayfsBackend) Commit(name string, logger OperationLogger) (string, error) {
	// Hash the FULL merged workspace state (mount point), not just the upper dir.
	// This ensures that keep on an unchanged workspace produces the same hash
	// as the base layer, and that the layer is always usable for forking.
	mountPoint := filepath.Join(b.workspacesDir(), name)
	hash := b.LayerHash(mountPoint)
	layerDir := filepath.Join(b.layersDir(), hash)
	if _, err := os.Stat(layerDir); os.IsNotExist(err) {
		// Materialize the full merged view into the layer directory.
		// We use tar to copy the mount point (which resolves the overlay)
		// rather than the upper dir alone.
		if err := os.MkdirAll(layerDir, 0755); err != nil {
			return "", err
		}
		if err := copyDir(mountPoint, layerDir); err != nil {
			return "", err
		}
	}
	return hash, nil
}

func (b *OverlayfsBackend) LayerHash(dir string) string {
	return NewCopyBackend().LayerHash(dir) // same hashing logic
}

func (b *OverlayfsBackend) Diff(wsA string, wsB string, layerB string, w io.Writer, logger OperationLogger) error {
	// Diff on overlayfs: for wsA vs layer, the effective filesystem is the mountpoint.
	// We can diff mountpoints directly.
	pa := filepath.Join(b.workspacesDir(), wsA)
	var pb string
	if wsB != "" {
		pb = filepath.Join(b.workspacesDir(), wsB)
	} else {
		pb = filepath.Join(b.layersDir(), layerB)
	}
	cmd := exec.Command("diff", "-ruN", pb, pa)
	out, _ := cmd.CombinedOutput()
	w.Write(out) // writes to the passed io.Writer
	return nil
}

// mount creates an overlayfs mount at mountPoint using the given lower, upper, and work.
func (b *OverlayfsBackend) mount(lower, upper, work, mountPoint string) error {
	// Prefer fuse-overlayfs (no root required) for user-mode overlay mounts.
	// Fusion is available on most Linux distros. If missing, the kernel mount
	// requires CAP_SYS_ADMIN and likely fails.
	opt := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	cmd := exec.Command("fuse-overlayfs", "-o", opt, mountPoint)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fuse-overlayfs mount: %w: %s", err, out)
	}
	return nil
}

// materialize recursively copies the contents of src to dst, excluding overlay
// pseudofiles (e.g. whiteouts).
func (b *OverlayfsBackend) materialize(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		dstPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		// Skip whiteouts in overlayfs: char device with 0,0
		if info.Mode()&os.ModeCharDevice != 0 {
			stat, ok := info.Sys().(*syscall.Stat_t)
			if ok && stat != nil && stat.Rdev == 0 {
				return nil // whiteout
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		os.MkdirAll(filepath.Dir(dstPath), 0755)
		return os.WriteFile(dstPath, data, info.Mode().Perm())
	})
}

// copyDirFromUpper copies just the overlayfs upper directory (changes) into a layer.
func copyDirFromUpper(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, rel)
		os.MkdirAll(filepath.Dir(dstPath), 0755)
		data, _ := os.ReadFile(path)
		return os.WriteFile(dstPath, data, info.Mode().Perm())
	})
}
