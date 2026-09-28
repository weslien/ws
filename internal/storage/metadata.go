package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
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

// Store reads/writes JSON metadata from a root directory.
// ALL reads and writes are serialized through a single global lock
// (~/.ws/meta/.lock) to prevent concurrent processes from racing
// on the shared JSON index files (layers.json, workspaces.json).
type Store struct {
	root string
}

func NewStore(root string) *Store { return &Store{root: root} }
func (s *Store) Root() string     { return s.root }
func (s *Store) metaDir() string  { return filepath.Join(s.root, "meta") }
func (s *Store) layersFile() string {
	_ = os.MkdirAll(s.metaDir(), 0755)
	return filepath.Join(s.metaDir(), "layers.json")
}
func (s *Store) workspacesFile() string {
	_ = os.MkdirAll(s.metaDir(), 0755)
	return filepath.Join(s.metaDir(), "workspaces.json")
}

// globalLockFile returns the path to the single global metadata lock.
// ALL metadata operations (reads AND writes) must hold this lock to
// prevent cross-file races and lost-update races when multiple
// processes (agents, GC, tests) access the same ~/.ws store.
func (s *Store) globalLockFile() string {
	_ = os.MkdirAll(s.metaDir(), 0755)
	return filepath.Join(s.metaDir(), ".lock")
}

// GlobalLock acquires an exclusive lock on the global metadata lock file.
// Caller must Close the returned file handle to release the lock.
func (s *Store) GlobalLock() (*os.File, error) {
	f, err := os.OpenFile(s.globalLockFile(), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// atomicWrite writes data to path atomically: write to temp file, then rename.
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadLayers reads the layer index. Caller should hold GlobalLock.
func (s *Store) ReadLayers() map[string]LayerMeta {
	data, _ := os.ReadFile(s.layersFile())
	if len(data) == 0 {
		return map[string]LayerMeta{}
	}
	var m map[string]LayerMeta
	json.Unmarshal(data, &m)
	return m
}

// WriteLayers writes the layer index. Caller should hold GlobalLock.
func (s *Store) WriteLayers(m map[string]LayerMeta) error {
	data, _ := json.MarshalIndent(m, "", "  ")
	return atomicWrite(s.layersFile(), data)
}

func (s *Store) WorkspacesFile() string {
	return s.workspacesFile()
}

// ReadWorkspaces reads the workspace index. Caller should hold GlobalLock.
func (s *Store) ReadWorkspaces() map[string]WorkspaceMeta {
	data, _ := os.ReadFile(s.workspacesFile())
	if len(data) == 0 {
		return map[string]WorkspaceMeta{}
	}
	var m map[string]WorkspaceMeta
	json.Unmarshal(data, &m)
	return m
}

// WriteWorkspaces writes the workspace index. Caller should hold GlobalLock.
func (s *Store) WriteWorkspaces(m map[string]WorkspaceMeta) error {
	data, _ := json.MarshalIndent(m, "", "  ")
	return atomicWrite(s.workspacesFile(), data)
}

// UpdateWorkspaces acquires the global lock, reads the current workspaces,
// calls fn to modify the map, and writes it back atomically.
func (s *Store) UpdateWorkspaces(fn func(m map[string]WorkspaceMeta)) error {
	gLock, err := s.GlobalLock()
	if err != nil {
		// Fallback: read-modify-write without lock
		m := s.ReadWorkspaces()
		fn(m)
		data, _ := json.MarshalIndent(m, "", "  ")
		return os.WriteFile(s.workspacesFile(), data, 0644)
	}
	defer gLock.Close()

	m := s.ReadWorkspaces()
	fn(m)
	data, _ := json.MarshalIndent(m, "", "  ")
	return atomicWrite(s.workspacesFile(), data)
}

// UpdateLayers acquires the global lock, reads the current layers,
// calls fn to modify the map, and writes it back atomically.
func (s *Store) UpdateLayers(fn func(m map[string]LayerMeta)) error {
	gLock, err := s.GlobalLock()
	if err != nil {
		// Fallback: read-modify-write without lock
		m := s.ReadLayers()
		fn(m)
		data, _ := json.MarshalIndent(m, "", "  ")
		return os.WriteFile(s.layersFile(), data, 0644)
	}
	defer gLock.Close()

	m := s.ReadLayers()
	fn(m)
	data, _ := json.MarshalIndent(m, "", "  ")
	return atomicWrite(s.layersFile(), data)
}
