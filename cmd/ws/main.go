package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"encoding/json"

	"github.com/weslien/ws/internal/backend"
	"github.com/weslien/ws/internal/storage"
)

// version is set at build time via ldflags: -X main.version=v0.2.2
var version = "dev"

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
	case "status":
		cmdStatus(args, be, store, logger)
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
	case "update":
		cmdUpdate()
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
	force := false
	for _, a := range args[2:] {
		if strings.HasPrefix(a, "--name=") {
			name = strings.TrimPrefix(a, "--name=")
		}
		if a == "--force" || a == "-f" {
			force = true
		}
	}
	if name == "" {
		die("--name required")
	}
	workspaces := store.ReadWorkspaces()
	if _, ok := workspaces[name]; ok {
		if !force {
			die("workspace %q already exists (use --force to replace)", name)
		}
		log.Log("workspace %q exists, dropping (--force)...", name)
		if err := be.Destroy(name, log); err != nil {
			log.Error("force-drop: %v", err)
		}
		delete(workspaces, name)
		store.WriteWorkspaces(workspaces)
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

func cmdStatus(args []string, be backend.Backender, store *storage.Store, log backend.OperationLogger) {
	if len(args) > 1 && containsHelp(args) {
		printHelp("status")
		return
	}
	workspaces := store.ReadWorkspaces()
	layers := store.ReadLayers()

	if len(workspaces) == 0 {
		fmt.Println("No active workspaces.")
		return
	}

	// Sort workspace names for stable output
	names := make([]string, 0, len(workspaces))
	for n := range workspaces {
		names = append(names, n)
	}
	sort.Strings(names)

	// Table header
	fmt.Printf("%-20s %-20s %-8s %-20s %s\n", "WORKSPACE", "LAYER", "DIRTY", "MESSAGE", "CREATED")
	fmt.Printf("%-20s %-20s %-8s %-20s %s\n", strings.Repeat("-", 20), strings.Repeat("-", 20), strings.Repeat("-", 8), strings.Repeat("-", 20), strings.Repeat("-", 20))

	for _, name := range names {
		w := workspaces[name]
		layerHash := w.FormedFrom

		// Get the layer's message
		msg := ""
		if l, ok := layers[layerHash]; ok {
			msg = l.Message
		}
		if len(msg) > 20 {
			msg = msg[:17] + "..."
		}

		// Check if workspace is dirty (hash differs from layer)
		dirty := "?"
		wsHash := be.LayerHash(filepath.Join(store.Root(), "workspaces", name))
		if wsHash == layerHash {
			dirty = "no"
		} else {
			dirty = "yes"
		}

		created := w.CreatedAt
		if len(created) > 20 {
			created = created[:20]
		}

		fmt.Printf("%-20s %-20s %-8s %-20s %s\n", name, layerHash[:12], dirty, msg, created)
	}

	// Summary line
	fmt.Printf("\n%d workspaces, %d layers\n", len(workspaces), len(layers))
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

func cmdUpdate() {
	be, err := newSelfUpdater()
	if err != nil {
		die("update: %v", err)
	}
	if err := be.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
		os.Exit(1)
	}
}

type selfUpdater struct {
	repo      string
	binName   string
	installer string
}

func newSelfUpdater() (*selfUpdater, error) {
	return &selfUpdater{
		repo:    "github.com/weslien/ws",
		binName: "ws",
		installer: "https://raw.githubusercontent.com/weslien/ws/main/install.sh",
	}, nil
}

func (s *selfUpdater) Run() error {
	ex, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding self: %w", err)
	}
	info, err := os.Stat(ex)
	if err != nil {
		return fmt.Errorf("stat self: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		real, err := filepath.EvalSymlinks(ex)
		if err != nil {
			return fmt.Errorf("resolve symlink: %w", err)
		}
		ex = real
	}
	binDir := filepath.Dir(ex)

	// Detect platform
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	// Normalize darwin arch names for Go-style naming used in release artifacts.
	_ = goarch // always amd64 or arm64 today

	fmt.Printf("current binary: %s\n", ex)
	fmt.Printf("platform: %s/%s\n", goos, goarch)

	// Strategy 1: try prebuilt release
	if err := s.tryPrebuilt(ex, goos, goarch); err == nil {
		fmt.Println("updated via prebuilt release")
		return nil
	} else {
		fmt.Printf("prebuilt: %v\n", err)
	}

	// Strategy 2: try source build (can inject version via ldflags)
	fmt.Println("trying source build ...")
	if err := s.trySource(binDir); err == nil {
		fmt.Println("updated via source build")
		return nil
	} else {
		fmt.Printf("source: %v\n", err)
	}

	// Strategy 3: installer script
	fmt.Println("trying installer script ...")
	if err := s.tryInstaller(binDir); err == nil {
		fmt.Println("updated via installer script")
		return nil
	} else {
		fmt.Printf("installer: %v\n", err)
	}

	// Strategy 4: go install (last resort — version will be 'dev')
	fmt.Println("trying go install ...")
	if err := s.tryGoInstall(ex); err == nil {
		fmt.Println("updated via go install")
		return nil
	} else {
		fmt.Printf("go install: %v\n", err)
	}

	return fmt.Errorf("all update strategies failed")
}

func (s *selfUpdater) tryPrebuilt(currentPath, goos, goarch string) error {
	latestTag, err := s.fetchLatestTag()
	if err != nil {
		return err
	}
	if latestTag == "" {
		return fmt.Errorf("no version tag found")
	}

	assetURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/ws-%s-%s-%s.tar.gz", s.repo, latestTag, latestTag, goos, goarch)
	fmt.Printf("downloading %s ...\n", assetURL)

	resp, err := http.Get(assetURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmpDir, err := os.MkdirTemp("", "ws-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	tarPath := filepath.Join(tmpDir, "ws.tar.gz")
	f, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	f.Close()

	if err := exec.Command("tar", "xzf", tarPath, "-C", tmpDir).Run(); err != nil {
		return fmt.Errorf("untar: %w", err)
	}
	newBin := filepath.Join(tmpDir, s.binName)
	if runtime.GOOS == "windows" {
		newBin = filepath.Join(tmpDir, s.binName+".exe")
	}
	if _, err := os.Stat(newBin); err != nil {
		return fmt.Errorf("no binary found in archive")
	}
	return s.replaceInPlace(currentPath, newBin)
}

func (s *selfUpdater) tryGoInstall(currentPath string) error {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return err
	}
	cmd := exec.Command(goBin, "install", s.repo+"/cmd/"+s.binName+"@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	// go install puts the binary in GOPATH/bin or GOBIN.
	// Find it via `go env GOPATH` + /bin/ws, or GOBIN.
	gobin := os.Getenv("GOBIN")
	if gobin == "" {
		gopath, err := exec.Command(goBin, "env", "GOPATH").Output()
		if err != nil {
			return fmt.Errorf("go env GOPATH: %w", err)
		}
		gobin = filepath.Join(strings.TrimSpace(string(gopath)), "bin")
	}
	newBin := filepath.Join(gobin, s.binName)
	if runtime.GOOS == "windows" {
		newBin += ".exe"
	}
	return s.replaceInPlace(currentPath, newBin)
}

func (s *selfUpdater) tryInstaller(binDir string) error {
	curl, err := exec.LookPath("curl")
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "ws-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	scriptPath := filepath.Join(tmpDir, "install.sh")
	cmd := exec.Command(curl, "-fsSL", "-o", scriptPath, s.installer)
	if err := cmd.Run(); err != nil {
		return err
	}
	if err := os.Chmod(scriptPath, 0755); err != nil {
		return err
	}
	inst := exec.Command("bash", scriptPath)
	inst.Stdout = os.Stdout
	inst.Stderr = os.Stderr
	return inst.Run()
}

func (s *selfUpdater) trySource(binDir string) error {
	git, err := exec.LookPath("git")
	if err != nil {
		return err
	}
	latestTag, err := s.fetchLatestTag()
	if err != nil {
		latestTag = ""
	}
	tmpDir, err := os.MkdirTemp("", "ws-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	src := filepath.Join(tmpDir, "src")
	cmd := exec.Command(git, "clone", "--depth", "1", "https://"+s.repo+".git", src)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	ldflags := "-s -w"
	if latestTag != "" {
		ldflags = ldflags + " -X main.version=" + latestTag
	}
	build := exec.Command("go", "build", "-ldflags", ldflags, "-o", filepath.Join(binDir, s.binName), "./cmd/"+s.binName)
	build.Dir = src
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	return build.Run()
}

func (s *selfUpdater) fetchLatestTag() (string, error) {
	// Use the tags API (newest first) — more reliable than releases/latest
	// which returns by release creation date, not semver order.
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/tags", s.repo)
	resp, err := http.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", err
	}
	if len(tags) == 0 {
		return "", nil
	}
	return tags[0].Name, nil
}

func (s *selfUpdater) replaceInPlace(currentPath, newBin string) error {
	// atomic overwrite: move old to .old, move new to path
	old := currentPath + ".old"
	if err := os.Rename(currentPath, old); err != nil {
		return fmt.Errorf("backup old: %w", err)
	}
	if err := os.Rename(newBin, currentPath); err != nil {
		// attempt restore
		_ = os.Rename(old, currentPath)
		return fmt.Errorf("install new: %w", err)
	}
	// remove old backup silently (best-effort)
	_ = os.Remove(old)
	return nil
}
