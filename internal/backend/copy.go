package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sort"
)

// ---------- CopyBackend (generic, macOS-safe) ----------

type CopyBackend struct {
	root string
}

func NewCopyBackend() *CopyBackend { return &CopyBackend{} }
func (b *CopyBackend) Name() string  { return "copy" }

func (b *CopyBackend) Init(root string) error {
	b.root = root
	for _, d := range []string{"layers", "workspaces", "meta"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			return err
		}
	}
	return nil
}

func (b *CopyBackend) rootDir() string        { return b.root }
func (b *CopyBackend) layersDir() string       { return filepath.Join(b.root, "layers") }
func (b *CopyBackend) workspacesDir() string  { return filepath.Join(b.root, "workspaces") }
func (b *CopyBackend) metaDir() string         { return filepath.Join(b.root, "meta") }

func (b *CopyBackend) Fork(srcHash string, dstName string, _ OperationLogger) error {
	return copyDir(filepath.Join(b.layersDir(), srcHash), filepath.Join(b.workspacesDir(), dstName))
}

func (b *CopyBackend) ForkFromWorkspace(srcName string, dstName string, _ OperationLogger) (string, error) {
	if err := copyDir(filepath.Join(b.workspacesDir(), srcName), filepath.Join(b.workspacesDir(), dstName)); err != nil {
		return "", err
	}
	return b.LayerHash(filepath.Join(b.workspacesDir(), srcName)), nil
}

func (b *CopyBackend) CloneGitRepo(repo string, ref string, logger OperationLogger) (string, string, error) {
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

func (b *CopyBackend) Mount(string, string, OperationLogger) error   { return nil }
func (b *CopyBackend) Unmount(string, OperationLogger) error        { return nil }
func (b *CopyBackend) Destroy(name string, _ OperationLogger) error   { return os.RemoveAll(filepath.Join(b.workspacesDir(), name)) }

func (b *CopyBackend) Commit(name string, _ OperationLogger) (string, error) {
	wsDir := filepath.Join(b.workspacesDir(), name)
	hash := b.LayerHash(wsDir)
	layerDir := filepath.Join(b.layersDir(), hash)
	if _, err := os.Stat(layerDir); os.IsNotExist(err) {
		if err := copyDir(wsDir, layerDir); err != nil {
			return "", err
		}
	}
	return hash, nil
}

func (b *CopyBackend) LayerHash(dir string) string {
	h := sha256.New()
	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		// Exclude .git from hash — two clones of the same repo
		// should produce the same layer hash for dedup.
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			return nil
		}
		// Skip symlinks — they're metadata, not content
		if info.Mode()&os.ModeSymlink != 0 {
			files = append(files, rel+" -> "+info.Name())
			return nil
		}
		files = append(files, rel)
		return nil
	})
	sort.Strings(files)
	for _, f := range files {
		h.Write([]byte(f))
		data, _ := os.ReadFile(filepath.Join(dir, f))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (b *CopyBackend) Diff(wsA string, wsB string, layerB string, w io.Writer, _ OperationLogger) error {
	pa := filepath.Join(b.workspacesDir(), wsA)
	var pb string
	if wsB != "" {
		pb = filepath.Join(b.workspacesDir(), wsB)
	} else {
		pb = filepath.Join(b.layersDir(), layerB)
	}
	cmd := exec.Command("diff", "-ruN", pb, pa)
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Run()
	return nil
}

// copyDir recursively copies src to dst, excluding .git directories.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dstPath := filepath.Join(dst, rel)

		// Handle symlinks: copy the link, not the target
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			os.MkdirAll(filepath.Dir(dstPath), 0755)
			return os.Symlink(target, dstPath)
		}

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()
		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()
		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return err
		}
		return os.Chmod(dstPath, info.Mode())
	})
}
