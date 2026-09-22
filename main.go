package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ---------- Config ----------

var wsRoot string

func init() {
	home, _ := os.UserHomeDir()
	wsRoot = filepath.Join(home, ".ws")
}

func layersDir() string  { return filepath.Join(wsRoot, "layers") }
func workspacesDir() string { return filepath.Join(wsRoot, "workspaces") }
func metaDir() string    { return filepath.Join(wsRoot, "meta") }

// ---------- Types ----------

type LayerMeta struct {
	Hash      string   `json:"hash"`
	Parent    string   `json:"parent,omitempty"`
	Basis     []string `json:"basis,omitempty"`
	Message   string   `json:"message,omitempty"`
	CreatedAt string   `json:"created_at"`
	CommittedBy string `json:"committed_by,omitempty"`
}

type WorkspaceMeta struct {
	Name       string   `json:"name"`
	Source     string   `json:"source"`       // layer:hash or ws:name or base:repo#ref
	FormedFrom string   `json:"formed_from"`  // the layer hash it was forked from
	State      string   `json:"state"`        // active
	CreatedAt  string   `json:"created_at"`
}

// ---------- Utils ----------

func ensureDirs() {
	os.MkdirAll(layersDir(), 0755)
	os.MkdirAll(workspacesDir(), 0755)
	os.MkdirAll(metaDir(), 0755)
}

func die(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+msg+"\n", args...)
	os.Exit(1)
}

func hashDir(dir string) string {
	h := sha256.New()
	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
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

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, rel)
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
		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return err
		}
		return os.Chmod(dstPath, info.Mode())
	})
}

func readLayers() map[string]LayerMeta {
	path := filepath.Join(metaDir(), "layers.json")
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		return map[string]LayerMeta{}
	}
	var m map[string]LayerMeta
	json.Unmarshal(data, &m)
	return m
}

func writeLayers(m map[string]LayerMeta) {
	path := filepath.Join(metaDir(), "layers.json")
	data, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(path, data, 0644)
}

func readWorkspaces() map[string]WorkspaceMeta {
	path := filepath.Join(metaDir(), "workspaces.json")
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		return map[string]WorkspaceMeta{}
	}
	var m map[string]WorkspaceMeta
	json.Unmarshal(data, &m)
	return m
}

func writeWorkspaces(m map[string]WorkspaceMeta) {
	path := filepath.Join(metaDir(), "workspaces.json")
	data, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(path, data, 0644)
}

func layerExists(hash string) bool {
	_, err := os.Stat(filepath.Join(layersDir(), hash))
	return !os.IsNotExist(err)
}

func workspaceExists(name string) bool {
	_, err := os.Stat(filepath.Join(workspacesDir(), name))
	return !os.IsNotExist(err)
}

// ---------- Commands ----------

func cmdGet(args []string) {
	if len(args) < 2 || args[0] != "get" {
		die("usage: ws get <source> --name=<ws-name>")
	}
	source := args[1]
	var name string
	for _, a := range args[2:] {
		if strings.HasPrefix(a, "--name=") {
			name = strings.TrimPrefix(a, "--name=")
		}
	}
	if name == "" {
		die("--name required")
	}
	if workspaceExists(name) {
		die("workspace %q already exists", name)
	}

	ensureDirs()
	layers := readLayers()
	workspaces := readWorkspaces()
	wsDir := filepath.Join(workspacesDir(), name)
	var formedFrom string

	switch {
	case strings.HasPrefix(source, "layer:"):
		hash := strings.TrimPrefix(source, "layer:")
		if !layerExists(hash) {
			die("layer %q not found", hash)
		}
		copyDir(filepath.Join(layersDir(), hash), wsDir)
		formedFrom = hash

	case strings.HasPrefix(source, "ws:"):
		srcName := strings.TrimPrefix(source, "ws:")
		if !workspaceExists(srcName) {
			die("workspace %q not found", srcName)
		}
		copyDir(filepath.Join(workspacesDir(), srcName), wsDir)
		// formed from the source workspace's formed_from
		if src, ok := workspaces[srcName]; ok {
			formedFrom = src.FormedFrom
		}

	case strings.HasPrefix(source, "base:"):
		rest := strings.TrimPrefix(source, "base:")
		parts := strings.SplitN(rest, "#", 2)
		repo := parts[0]
		ref := "HEAD"
		if len(parts) > 1 {
			ref = parts[1]
		}
		// clone to temp, hash it, store as layer, then copy to ws
		tmpDir, err := os.MkdirTemp("", "ws-clone-*")
		if err != nil {
			die("temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)
		fmt.Printf("cloning %s@%s...\n", repo, ref)
		cmd := exec.Command("git", "clone", "--depth=1", "--branch="+ref, repo, tmpDir)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			// try without branch (maybe ref is commit or default branch)
			cmd = exec.Command("git", "clone", "--depth=1", repo, tmpDir)
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				die("git clone failed: %v", err)
			}
			if ref != "HEAD" {
				exec.Command("git", "-C", tmpDir, "checkout", ref).Run()
			}
		}
		hash := hashDir(tmpDir)
		layerDir := filepath.Join(layersDir(), hash)
		if !layerExists(hash) {
			copyDir(tmpDir, layerDir)
			layers[hash] = LayerMeta{
				Hash:      hash,
				Message:   fmt.Sprintf("base:%s#%s", repo, ref),
				CreatedAt: time.Now().Format(time.RFC3339),
			}
			writeLayers(layers)
			fmt.Printf("created layer %s\n", hash)
		}
		copyDir(layerDir, wsDir)
		formedFrom = hash

	default:
		die("unknown source type: %s (expected layer:, ws:, or base:)", source)
	}

	workspaces[name] = WorkspaceMeta{
		Name:       name,
		Source:     source,
		FormedFrom: formedFrom,
		State:      "active",
		CreatedAt:  time.Now().Format(time.RFC3339),
	}
	writeWorkspaces(workspaces)
	fmt.Printf("workspace %s formed from %s\n", name, formedFrom)
}

func cmdRun(args []string) {
	if len(args) < 4 || args[0] != "run" || args[2] != "--" {
		die("usage: ws run <ws-name> -- <command...>")
	}
	name := args[1]
	if !workspaceExists(name) {
		die("workspace %q not found", name)
	}
	wsDir := filepath.Join(workspacesDir(), name)
	cmd := exec.Command(args[3], args[4:]...)
	cmd.Dir = wsDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		die("run failed: %v", err)
	}
}

func cmdDiff(args []string) {
	if len(args) < 2 || args[0] != "diff" {
		die("usage: ws diff <ws-name> [ws-name-B]")
	}
	nameA := args[1]
	if !workspaceExists(nameA) {
		die("workspace %q not found", nameA)
	}
	wsA := filepath.Join(workspacesDir(), nameA)

	var wsB string
	if len(args) >= 3 {
		nameB := args[2]
		if workspaceExists(nameB) {
			wsB = filepath.Join(workspacesDir(), nameB)
		} else if strings.HasPrefix(nameB, "layer:") {
			hash := strings.TrimPrefix(nameB, "layer:")
			if !layerExists(hash) {
				die("layer %q not found", hash)
			}
			wsB = filepath.Join(layersDir(), hash)
		} else {
			die("%q is not a workspace or layer", nameB)
		}
	} else {
		// diff against formedFrom layer
		workspaces := readWorkspaces()
		w, ok := workspaces[nameA]
		if !ok || w.FormedFrom == "" {
			die("no source layer known for workspace %q", nameA)
		}
		wsB = filepath.Join(layersDir(), w.FormedFrom)
	}

	cmd := exec.Command("diff", "-ruN", wsB, wsA)
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Print(string(out))
	} else if err != nil {
		fmt.Println("(no differences)")
	}
}

func cmdKeep(args []string) {
	if len(args) < 2 || args[0] != "keep" {
		die("usage: ws keep <ws-name> [--message=<msg>]")
	}
	name := args[1]
	if !workspaceExists(name) {
		die("workspace %q not found", name)
	}
	var msg string
	for _, a := range args[2:] {
		if strings.HasPrefix(a, "--message=") {
			msg = strings.TrimPrefix(a, "--message=")
		}
	}

	wsDir := filepath.Join(workspacesDir(), name)
	workspaces := readWorkspaces()
	w := workspaces[name]

	// Compute hash of current workspace
	hash := hashDir(wsDir)
	if layerExists(hash) {
		fmt.Printf("layer %s already exists (no changes since fork?)\n", hash)
		return
	}

	layerDir := filepath.Join(layersDir(), hash)
	copyDir(wsDir, layerDir)

	layers := readLayers()
	layers[hash] = LayerMeta{
		Hash:        hash,
		Parent:      w.FormedFrom,
		Message:     msg,
		CreatedAt:   time.Now().Format(time.RFC3339),
		CommittedBy: name,
	}
	writeLayers(layers)

	// Update workspace to point to the new layer as its basis
	w.FormedFrom = hash
	workspaces[name] = w
	writeWorkspaces(workspaces)
	fmt.Printf("kept layer %s\n", hash)
}

func cmdDrop(args []string) {
	if len(args) < 2 || args[0] != "drop" {
		die("usage: ws drop <ws-name>")
	}
	name := args[1]
	if !workspaceExists(name) {
		die("workspace %q not found", name)
	}
	wsDir := filepath.Join(workspacesDir(), name)
	os.RemoveAll(wsDir)
	workspaces := readWorkspaces()
	delete(workspaces, name)
	writeWorkspaces(workspaces)
	fmt.Printf("dropped workspace %s\n", name)
}

func cmdGraph(args []string) {
	if len(args) < 1 || args[0] != "graph" {
		die("usage: ws graph [ws-name]")
	}
	layers := readLayers()
	workspaces := readWorkspaces()

	if len(args) == 1 {
		// show all
		fmt.Println("Layers:")
		for hash, l := range layers {
			parent := "(base)"
			if l.Parent != "" {
				parent = l.Parent
			}
			fmt.Printf("  layer:%s <- %s [%s]\n", hash, parent, l.Message)
		}
		fmt.Println("\nWorkspaces:")
		for name, w := range workspaces {
			fmt.Printf("  ws:%s from layer:%s [%s]\n", name, w.FormedFrom, w.State)
		}
	} else {
		// show tree for specific workspace
		name := args[1]
		w, ok := workspaces[name]
		if !ok {
			die("workspace %q not found", name)
		}
		fmt.Printf("ws:%s [%s]\n", name, w.State)
		depth := 1
		cur := w.FormedFrom
		for cur != "" {
			l, ok := layers[cur]
			if !ok {
				break
			}
			indent := strings.Repeat("  ", depth)
			fmt.Printf("%s└─ layer:%s", indent, cur)
			if l.Message != "" {
				fmt.Printf(" [%s]", l.Message)
			}
			fmt.Println()
			cur = l.Parent
			depth++
		}
	}
}

func cmdLayer(args []string) {
	if len(args) < 2 || args[0] != "layer" {
		die("usage: ws layer <ls|show|gc>")
	}
	sub := args[1]
	switch sub {
	case "ls":
		layers := readLayers()
		for hash, l := range layers {
			fmt.Printf("%s  %s  %s\n", hash, l.CreatedAt, l.Message)
		}
	case "show":
		if len(args) < 3 {
			die("usage: ws layer show <hash>")
		}
		hash := strings.TrimPrefix(args[2], "layer:")
		layers := readLayers()
		l, ok := layers[hash]
		if !ok {
			die("layer %q not found", hash)
		}
		fmt.Printf("hash:      %s\n", l.Hash)
		fmt.Printf("parent:    %s\n", l.Parent)
		fmt.Printf("basis:     %v\n", l.Basis)
		fmt.Printf("message:   %s\n", l.Message)
		fmt.Printf("created:   %s\n", l.CreatedAt)
		fmt.Printf("committed: %s\n", l.CommittedBy)
	case "gc":
		layers := readLayers()
		workspaces := readWorkspaces()
		referenced := map[string]bool{}
		for _, w := range workspaces {
			referenced[w.FormedFrom] = true
		}
		// transitive closure of parents
		changed := true
		for changed {
			changed = false
			for hash, l := range layers {
				if !referenced[hash] {
					continue
				}
				if l.Parent != "" && !referenced[l.Parent] {
					referenced[l.Parent] = true
					changed = true
				}
			}
		}
		removed := 0
		for hash := range layers {
			if !referenced[hash] {
				os.RemoveAll(filepath.Join(layersDir(), hash))
				delete(layers, hash)
				removed++
			}
		}
		writeLayers(layers)
		fmt.Printf("gc: removed %d unreferenced layers\n", removed)
	default:
		die("unknown layer subcommand: %s", sub)
	}
}

// ---------- Main ----------

func usage() {
	fmt.Println(`ws — overlayfs workspace graph (v1, macOS copy backend)

Usage:
  ws get <source> --name=<ws-name>
       source: layer:HASH | ws:NAME | base:REPO#REF
  ws run <ws-name> -- <command...>
  ws diff <ws-name> [ws-name-B | layer:HASH]
  ws keep <ws-name> [--message=<msg>]
  ws drop <ws-name>
  ws graph [ws-name]
  ws layer <ls|show <hash>|gc>`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	ensureDirs()
	switch os.Args[1] {
	case "get":
		cmdGet(os.Args[1:])
	case "run":
		cmdRun(os.Args[1:])
	case "diff":
		cmdDiff(os.Args[1:])
	case "keep":
		cmdKeep(os.Args[1:])
	case "drop":
		cmdDrop(os.Args[1:])
	case "graph":
		cmdGraph(os.Args[1:])
	case "layer":
		cmdLayer(os.Args[1:])
	default:
		usage()
		os.Exit(1)
	}
}
