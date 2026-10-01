package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// LayerMeta is the JSON representation of an immutable layer.
type LayerMeta struct {
	Hash        string   `json:"hash"`
	Parent      string   `json:"parent,omitempty"`
	Basis       []string `json:"basis,omitempty"`
	Message     string   `json:"message,omitempty"`
	CreatedAt   string   `json:"created_at"`
	CommittedBy string   `json:"committed_by,omitempty"`
}

// WorkspaceMeta is the JSON representation of a mutable workspace.
type WorkspaceMeta struct {
	Name       string `json:"name"`
	Source     string `json:"source"`
	FormedFrom string `json:"formed_from"`
	State      string `json:"state"`
	CreatedAt  string `json:"created_at"`
}

// Store reads/writes metadata from a root directory using per-entity files.
//
// Architecture: one JSON file per workspace and per layer, not a monolithic
// index. Two sessions creating different workspaces write to DIFFERENT files
// — no read-modify-write race on a shared index. readdir is atomic and returns
// a consistent snapshot. Writes use temp-then-rename for crash safety.
//
// ~/.ws/meta/
//
//	workspaces/
//	  w1.json
//	  w2.json
//	layers/
//	  <hash>.json
//	  <hash>.json
//
// The only operation that needs a lock is GC (to prevent new layers from
// being created during the reachability scan). Normal get/keep/drop/write
// operations are lock-free.
type Store struct {
	root string
}

func NewStore(root string) *Store { return &Store{root: root} }
func (s *Store) Root() string     { return s.root }
func (s *Store) metaDir() string  { return filepath.Join(s.root, "meta") }
func (s *Store) workspacesDir() string {
	_ = os.MkdirAll(filepath.Join(s.metaDir(), "workspaces"), 0755)
	return filepath.Join(s.metaDir(), "workspaces")
}
func (s *Store) layersDir() string {
	_ = os.MkdirAll(filepath.Join(s.metaDir(), "layers"), 0755)
	return filepath.Join(s.metaDir(), "layers")
}

// Legacy monolithic file paths (for backward-compat migration).
func (s *Store) LayersFile() string     { return filepath.Join(s.metaDir(), "layers.json") }
func (s *Store) layersFile() string     { return s.LayersFile() }
func (s *Store) WorkspacesFile() string { return filepath.Join(s.metaDir(), "workspaces.json") }
func (s *Store) workspacesFile() string { return s.WorkspacesFile() }

// ── Global Lock (GC only) ──────────────────────────────────────────────

// globalLockFile returns the path to the metadata lock file.
// Only needed by GC to prevent new layers during reachability scan.
func (s *Store) globalLockFile() string {
	_ = os.MkdirAll(s.metaDir(), 0755)
	return filepath.Join(s.metaDir(), ".lock")
}

// GlobalLock acquires an exclusive lock on the metadata lock file.
// Only needed by GC. Normal operations are lock-free with per-entity files.
func (s *Store) GlobalLock() (*os.File, error) {
	f, err := os.OpenFile(s.globalLockFile(), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	if err := lockFileExclusive(f); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// ── Atomic write helper ───────────────────────────────────────────────

// atomicWrite writes data to path atomically: write to temp file, then rename.
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ── Workspace per-entity operations ───────────────────────────────────

// WriteWorkspace writes a single workspace's metadata to its own file.
// Atomic (temp + rename). No lock needed — different workspaces = different files.
func (s *Store) WriteWorkspace(name string, meta WorkspaceMeta) error {
	data, _ := json.MarshalIndent(meta, "", "  ")
	return atomicWrite(filepath.Join(s.workspacesDir(), name+".json"), data)
}

// DeleteWorkspace removes a single workspace's metadata file.
func (s *Store) DeleteWorkspace(name string) error {
	return os.Remove(filepath.Join(s.workspacesDir(), name+".json"))
}

// ReadWorkspace reads a single workspace's metadata. Returns false if not found.
func (s *Store) ReadWorkspace(name string) (WorkspaceMeta, bool) {
	data, err := os.ReadFile(filepath.Join(s.workspacesDir(), name+".json"))
	if err != nil {
		// Try legacy monolithic file
		m := s.ReadWorkspaces()
		w, ok := m[name]
		return w, ok
	}
	var w WorkspaceMeta
	if json.Unmarshal(data, &w) != nil {
		return WorkspaceMeta{}, false
	}
	return w, true
}

// ── Layer per-entity operations ──────────────────────────────────────

// WriteLayer writes a single layer's metadata to its own file.
// Atomic (temp + rename). No lock needed.
func (s *Store) WriteLayer(hash string, meta LayerMeta) error {
	data, _ := json.MarshalIndent(meta, "", "  ")
	return atomicWrite(filepath.Join(s.layersDir(), hash+".json"), data)
}

// DeleteLayer removes a single layer's metadata file.
func (s *Store) DeleteLayer(hash string) error {
	return os.Remove(filepath.Join(s.layersDir(), hash+".json"))
}

// ReadLayer reads a single layer's metadata. Returns false if not found.
func (s *Store) ReadLayer(hash string) (LayerMeta, bool) {
	data, err := os.ReadFile(filepath.Join(s.layersDir(), hash+".json"))
	if err != nil {
		// Try legacy monolithic file
		m := s.ReadLayers()
		l, ok := m[hash]
		return l, ok
	}
	var l LayerMeta
	if json.Unmarshal(data, &l) != nil {
		return LayerMeta{}, false
	}
	return l, true
}

// ── Bulk reads (readdir + read all) ───────────────────────────────────

// ReadWorkspaces reads all workspaces by listing the workspaces/ directory.
// readdir is atomic — returns a consistent snapshot. Falls back to legacy
// monolithic file if per-entity dir doesn't exist yet.
func (s *Store) ReadWorkspaces() map[string]WorkspaceMeta {
	result := map[string]WorkspaceMeta{}

	// Try per-entity files first
	entries, err := os.ReadDir(s.workspacesDir())
	if err == nil && len(entries) > 0 {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(s.workspacesDir(), e.Name()))
			if err != nil {
				continue
			}
			var w WorkspaceMeta
			if json.Unmarshal(data, &w) == nil && w.Name != "" {
				result[w.Name] = w
			}
		}
		return result
	}

	// Fall back to legacy monolithic file
	data, _ := os.ReadFile(s.workspacesFile())
	if len(data) == 0 {
		return result
	}
	var m map[string]WorkspaceMeta
	json.Unmarshal(data, &m)
	return m
}

// ReadLayers reads all layers by listing the layers/ directory.
// readdir is atomic — returns a consistent snapshot. Falls back to legacy
// monolithic file if per-entity dir doesn't exist yet.
func (s *Store) ReadLayers() map[string]LayerMeta {
	result := map[string]LayerMeta{}

	// Try per-entity files first
	entries, err := os.ReadDir(s.layersDir())
	if err == nil && len(entries) > 0 {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(s.layersDir(), e.Name()))
			if err != nil {
				continue
			}
			var l LayerMeta
			if json.Unmarshal(data, &l) == nil && l.Hash != "" {
				result[l.Hash] = l
			}
		}
		return result
	}

	// Fall back to legacy monolithic file
	data, _ := os.ReadFile(s.layersFile())
	if len(data) == 0 {
		return result
	}
	var m map[string]LayerMeta
	json.Unmarshal(data, &m)
	return m
}

// ── Legacy bulk writes (deprecated, use per-entity methods) ──────────

// WriteLayers writes all layers. DEPRECATED — use WriteLayer/DeleteLayer.
// Kept for backward compat. Migrates to per-entity files.
func (s *Store) WriteLayers(m map[string]LayerMeta) error {
	// Write each layer to its own file
	for hash, meta := range m {
		if err := s.WriteLayer(hash, meta); err != nil {
			return err
		}
	}
	// Remove legacy monolithic file (migrated)
	os.Remove(s.layersFile())
	return nil
}

// WriteWorkspaces writes all workspaces. DEPRECATED — use WriteWorkspace/DeleteWorkspace.
// Kept for backward compat. Migrates to per-entity files.
func (s *Store) WriteWorkspaces(m map[string]WorkspaceMeta) error {
	// Write each workspace to its own file
	for name, meta := range m {
		if err := s.WriteWorkspace(name, meta); err != nil {
			return err
		}
	}
	// Remove legacy monolithic file (migrated)
	os.Remove(s.workspacesFile())
	return nil
}

// ── Legacy lock-based helpers (deprecated) ───────────────────────────

// UpdateWorkspaces acquires the global lock, reads the current workspaces,
// calls fn to modify the map, and writes it back. DEPRECATED — use
// WriteWorkspace/DeleteWorkspace for lock-free per-entity operations.
func (s *Store) UpdateWorkspaces(fn func(m map[string]WorkspaceMeta)) error {
	m := s.ReadWorkspaces()
	fn(m)
	for name, meta := range m {
		s.WriteWorkspace(name, meta)
	}
	return nil
}

// UpdateLayers acquires the global lock, reads the current layers,
// calls fn to modify the map, and writes it back. DEPRECATED — use
// WriteLayer/DeleteLayer for lock-free per-entity operations.
func (s *Store) UpdateLayers(fn func(m map[string]LayerMeta)) error {
	m := s.ReadLayers()
	fn(m)
	for hash, meta := range m {
		s.WriteLayer(hash, meta)
	}
	return nil
}
