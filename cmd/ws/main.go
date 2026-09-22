package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/weslien/ws/internal/backend"
	"github.com/weslien/ws/internal/storage"
)

func die(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+msg+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		printHelp("")
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[1:]

	// Global flags
	if cmd == "--help" || cmd == "-h" {
		printHelp("")
		return
	}
	if cmd == "--version" || cmd == "-v" {
		printVersion()
		return
	}

	home, _ := os.UserHomeDir()
	root := filepath.Join(home, ".ws")

	be := backend.NewAuto()
	logger := &consoleLogger{}

	if err := be.Init(root); err != nil {
		die("backend init: %v", err)
	}
	store := storage.NewStore(root)

	switch cmd {
	case "get":
		cmdGet(args, be, store, logger)
	case "run":
		cmdRun(args, be, store, logger)
	case "diff":
		cmdDiff(args, be, store, logger)
	case "keep":
		cmdKeep(args, be, store, logger)
	case "drop":
		cmdDrop(args, be, store, logger)
	case "graph":
		cmdGraph(args, be, store, logger)
	case "layer":
		cmdLayer(args, be, store, logger)
	case "help":
		if len(os.Args) > 2 {
			printHelp(os.Args[2])
		} else {
			printHelp("")
		}
	case "skill":
		cmdSkill()
	default:
		printHelp("")
		os.Exit(1)
	}
}

type consoleLogger struct{}

func (c *consoleLogger) Log(format string, args ...any)   { fmt.Printf(format+"\n", args...) }
func (c *consoleLogger) Error(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...) }

// containsHelp returns true if any arg contains "help" or "--help".
func containsHelp(args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			return true
		}
	}
	return false
}

func cmdGet(args []string, be backend.Backender, store *storage.Store, log backend.OperationLogger) {
	if len(args) < 2 || containsHelp(args) {
		printHelp("get")
		os.Exit(1)
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
	workspaces := store.ReadWorkspaces()
	if _, ok := workspaces[name]; ok {
		die("workspace %q already exists", name)
	}

	var formedFrom string
	switch {
	case strings.HasPrefix(source, "layer:"):
		hash := strings.TrimPrefix(source, "layer:")
		if log != nil {
			log.Log("forking from layer %s...", hash)
		}
		if err := be.Fork(hash, name, log); err != nil {
			die("fork from layer: %v", err)
		}
		formedFrom = hash

	case strings.HasPrefix(source, "ws:"):
		srcName := strings.TrimPrefix(source, "ws:")
		if _, ok := workspaces[srcName]; !ok {
			die("workspace %q not found", srcName)
		}
		snapHash, err := be.ForkFromWorkspace(srcName, name, log)
		if err != nil {
			die("fork from workspace: %v", err)
		}
		// If the backend returned a non-empty snapHash, use it as the formedFrom
		// to link the new workspace to the snapshot (not the original base).
		// If snapHash is empty (e.g. copy backend), fall back to src's formedFrom.
		if snapHash != "" {
			formedFrom = snapHash
		} else {
			formedFrom = workspaces[srcName].FormedFrom
		}

	case strings.HasPrefix(source, "base:"):
		rest := strings.TrimPrefix(source, "base:")
		parts := strings.SplitN(rest, "#", 2)
		repo := parts[0]
		ref := "HEAD"
		if len(parts) > 1 {
			ref = parts[1]
		}
		hash, tmpDir, err := be.CloneGitRepo(repo, ref, log)
		if err != nil {
			die("clone: %v", err)
		}
		if tmpDir != "" {
			defer os.RemoveAll(tmpDir)
		}
		layers := store.ReadLayers()
		if _, exists := layers[hash]; !exists {
			log.Log("created layer %s", hash)
			layers[hash] = storage.LayerMeta{
				Hash:    hash,
				Message: fmt.Sprintf("base:%s#%s", repo, ref),
			}
			store.WriteLayers(layers)
		}
		if err := be.Fork(hash, name, log); err != nil {
			die("fork from cloned layer: %v", err)
		}
		formedFrom = hash

	default:
		die("unknown source type: %s", source)
	}

	workspaces[name] = storage.WorkspaceMeta{
		Name:       name,
		Source:     source,
		FormedFrom: formedFrom,
		State:      "active",
		CreatedAt:  time.Now().Format(time.RFC3339),
	}
	store.WriteWorkspaces(workspaces)
	fmt.Printf("workspace %s formed from %s\n", name, formedFrom)
}

func cmdRun(args []string, be backend.Backender, store *storage.Store, log backend.OperationLogger) {
	if len(args) < 4 || containsHelp(args) || args[2] != "--" {
			printHelp("run")
		os.Exit(1)
	}
	name := args[1]
	workspaces := store.ReadWorkspaces()
	if _, ok := workspaces[name]; !ok {
		die("workspace %q not found", name)
	}
	wsDir := filepath.Join(os.Getenv("HOME"), ".ws", "workspaces", name)
	cmd := args[3]
	cmdArgs := args[4:]

	// Mount if needed (overlayfs backend)
	_ = be.Mount(name, workspaces[name].FormedFrom, log)
	c := exec.Command(cmd, cmdArgs...)
	c.Dir = wsDir
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		die("run failed: %v", err)
	}
}

func cmdDiff(args []string, be backend.Backender, store *storage.Store, log backend.OperationLogger) {
	if len(args) < 2 || containsHelp(args) {
			printHelp("diff")
		os.Exit(1)
	}
	nameA := args[1]
	workspaces := store.ReadWorkspaces()
	if _, ok := workspaces[nameA]; !ok {
		die("workspace %q not found", nameA)
	}

	var wsB, layerB string
	if len(args) >= 3 {
		second := args[2]
		if strings.HasPrefix(second, "layer:") {
			layerB = strings.TrimPrefix(second, "layer:")
		} else if _, ok := workspaces[second]; ok {
			wsB = second
		} else {
			die("%q is not a workspace or layer", second)
		}
	}
	if wsB == "" && layerB == "" {
		layerB = workspaces[nameA].FormedFrom
	}

	_ = be.Mount(nameA, workspaces[nameA].FormedFrom, log)
	if wsB != "" {
		_ = be.Mount(wsB, workspaces[wsB].FormedFrom, log)
	}

	if err := be.Diff(nameA, wsB, layerB, os.Stdout, log); err != nil {
		die("diff: %v", err)
	}
}

func cmdKeep(args []string, be backend.Backender, store *storage.Store, log backend.OperationLogger) {
	if len(args) < 2 || containsHelp(args) {
			printHelp("keep")
		os.Exit(1)
	}
	name := args[1]
	var msg string
	for _, a := range args[2:] {
		if strings.HasPrefix(a, "--message=") {
			msg = strings.TrimPrefix(a, "--message=")
		}
	}

	workspaces := store.ReadWorkspaces()
	w, ok := workspaces[name]
	if !ok {
		die("workspace %q not found", name)
	}

	// Mount before commit (overlayfs may need it)
	_ = be.Mount(name, w.FormedFrom, log)

	hash, err := be.Commit(name, log)
	if err != nil {
		die("commit: %v", err)
	}
	if hash == "" {
		die("commit produced empty hash")
	}

	layers := store.ReadLayers()
	if _, exists := layers[hash]; !exists {
		layers[hash] = storage.LayerMeta{
			Hash:        hash,
			Parent:      w.FormedFrom,
			Message:     msg,
			CreatedAt:   time.Now().Format(time.RFC3339),
			CommittedBy: name,
		}
		store.WriteLayers(layers)
	}

	// Update workspace to point to the new layer as its basis
	w.FormedFrom = hash
	workspaces[name] = w
	store.WriteWorkspaces(workspaces)
	fmt.Printf("kept layer %s\n", hash)
}

func cmdDrop(args []string, be backend.Backender, store *storage.Store, log backend.OperationLogger) {
	if len(args) < 2 || containsHelp(args) {
			printHelp("drop")
		os.Exit(1)
	}
	name := args[1]
	workspaces := store.ReadWorkspaces()
	if _, ok := workspaces[name]; !ok {
		die("workspace %q not found", name)
	}
	if err := be.Destroy(name, log); err != nil {
		die("destroy: %v", err)
	}
	delete(workspaces, name)
	store.WriteWorkspaces(workspaces)
	fmt.Printf("dropped workspace %s\n", name)
}

func cmdGraph(args []string, be backend.Backender, store *storage.Store, log backend.OperationLogger) {
	if len(args) > 1 && containsHelp(args) {
		printHelp("graph")
		return
	}
	layers := store.ReadLayers()
	workspaces := store.ReadWorkspaces()

	if len(args) == 1 {
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

func cmdLayer(args []string, be backend.Backender, store *storage.Store, log backend.OperationLogger) {
	if len(args) < 2 || containsHelp(args) {
			printHelp("layer")
		os.Exit(1)
	}
	sub := args[1]
	switch sub {
	case "ls":
		layers := store.ReadLayers()
		for hash, l := range layers {
			fmt.Printf("%s  %s  %s\n", hash, l.CreatedAt, l.Message)
		}
	case "show":
		if len(args) < 3 {
			die("usage: ws layer show <hash>")
		}
		hash := strings.TrimPrefix(args[2], "layer:")
		layers := store.ReadLayers()
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
		layers := store.ReadLayers()
		workspaces := store.ReadWorkspaces()
		referenced := map[string]bool{}
		for _, w := range workspaces {
			referenced[w.FormedFrom] = true
		}
		// transitive closure
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
				os.RemoveAll(filepath.Join(os.Getenv("HOME"), ".ws", "layers", hash))
				delete(layers, hash)
				removed++
			}
		}
		store.WriteLayers(layers)
		fmt.Printf("gc: removed %d unreferenced layers\n", removed)
	default:
		die("unknown layer subcommand: %s", sub)
	}
}

// skillLocations returns the directories where Hermes skill stores are expected.
func skillLocations() []string {
	var dirs []string
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE") // Windows fallback
	}
	// Default Hermes skill directory — note: this may differ if the user
	// customised their profile. These are the most common paths.
	for _, path := range []string{
		filepath.Join(home, ".hermes", "skills"),
		filepath.Join(home, ".config", "hermes", "skills"),
		filepath.Join(home, ".hermes", "profiles", "default", "skills"),
	} {
		dirs = append(dirs, path)
	}
	return dirs
}

func cmdSkill() {
	// Determine source directory: prefer filesystem assets, fall back to embedded.
	ex, err := os.Executable()
	if err != nil {
		die("cannot locate self: %v", err)
	}
	binDir := filepath.Dir(ex)
	src := filepath.Join(binDir, "assets", "skill", defaultSkillName)
	fallbackMode := false
	if _, err := os.Stat(src); os.IsNotExist(err) {
		// Embedded fallback for release binaries or go-install installs.
		tmpDir, err := os.MkdirTemp("", "ws-skill-*")
		if err != nil {
			die("cannot create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)
		skillDir := filepath.Join(tmpDir, defaultSkillName)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			die("cannot create skill dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(defaultSkillContent), 0644); err != nil {
			die("cannot write embedded skill: %v", err)
		}
		// Note: the built-in help text matches this file. If you update SKILL.md
		// in the repo, regenerate skill-embedded.go so the binary stays in sync.
		src = skillDir
		fallbackMode = true
	}

	for _, dst := range skillLocations() {
		if err := os.MkdirAll(dst, 0755); err != nil {
			continue
		}
		if _, err := os.Stat(dst); err == nil {
			target := filepath.Join(dst, defaultSkillName)
			// Remove previous installation if present
			_ = os.RemoveAll(target)
			if err := os.CopyFS(target, os.DirFS(src)); err != nil {
				fmt.Fprintf(os.Stderr, "warn: failed to copy to %s: %v\n", target, err)
				continue
			}
			if fallbackMode {
				fmt.Printf("installed skill '%s' (embedded) to %s\n", defaultSkillName, target)
			} else {
				fmt.Printf("installed skill '%s' to %s\n", defaultSkillName, target)
			}
			return
		}
	}
	die("could not install skill; tried: %s", strings.Join(skillLocations(), ", "))
}
