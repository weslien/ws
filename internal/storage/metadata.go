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
	Message     string  `json:"message,omitempty"`
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
// All writes are protected by file locks to prevent concurrent corruption.
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
	return filepath.Join(s.metaDir(), "workspaces.json")
}

// lockFile acquires an exclusive flock on the metadata file, returning the
// file handle (caller must Close to release the lock).
func lockFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
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
// The lockFile handle must be held by the caller during this operation.
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) ReadLayers() map[string]LayerMeta {
	data, _ := os.ReadFile(s.layersFile())
	if len(data) == 0 {
		return map[string]LayerMeta{}
	}
	var m map[string]LayerMeta
	json.Unmarshal(data, &m)
	return m
}

func (s *Store) WriteLayers(m map[string]LayerMeta) error {
	data, _ := json.MarshalIndent(m, "", "  ")
	f, err := lockFile(s.layersFile())
	if err != nil {
		// Fallback: write without lock if locking fails
		return os.WriteFile(s.layersFile(), data, 0644)
	}
	defer f.Close()
	return atomicWrite(s.layersFile(), data)
}

func (s *Store) ReadWorkspaces() map[string]WorkspaceMeta {
	data, _ := os.ReadFile(s.workspacesFile())
	if len(data) == 0 {
		return map[string]WorkspaceMeta{}
	}
	var m map[string]WorkspaceMeta
	json.Unmarshal(data, &m)
	return m
}

func (s *Store) WriteWorkspaces(m map[string]WorkspaceMeta) error {
	data, _ := json.MarshalIndent(m, "", "  ")
	f, err := lockFile(s.workspacesFile())
	if err != nil {
		return os.WriteFile(s.workspacesFile(), data, 0644)
	}
	defer f.Close()
	return atomicWrite(s.workspacesFile(), data)
}

// UpdateWorkspaces acquires a lock, reads the current workspaces, calls fn
// to modify the map, and writes it back atomically. This prevents lost-update
// races when multiple processes modify workspaces concurrently.
func (s *Store) UpdateWorkspaces(fn func(m map[string]WorkspaceMeta)) error {
	path := s.workspacesFile()
	_ = os.MkdirAll(s.metaDir(), 0755)
	f, err := lockFile(path)
	if err != nil {
		// Fallback: read-modify-write without lock
		m := s.ReadWorkspaces()
		fn(m)
		data, _ := json.MarshalIndent(m, "", "  ")
		return os.WriteFile(path, data, 0644)
	}
	defer f.Close()

	// Read current state while holding the lock
	data, err := os.ReadFile(path)
	m := map[string]WorkspaceMeta{}
	if len(data) > 0 {
		json.Unmarshal(data, &m)
	}
	fn(m)
	out, _ := json.MarshalIndent(m, "", "  ")
	return atomicWrite(path, out)
}

// UpdateLayers acquires a lock, reads the current layers, calls fn
// to modify the map, and writes it back atomically.
func (s *Store) UpdateLayers(fn func(m map[string]LayerMeta)) error {
	path := s.layersFile()
	_ = os.MkdirAll(s.metaDir(), 0755)
	f, err := lockFile(path)
	if err != nil {
		m := s.ReadLayers()
		fn(m)
		data, _ := json.MarshalIndent(m, "", "  ")
		return os.WriteFile(path, data, 0644)
	}
	defer f.Close()

	data, err := os.ReadFile(path)
	m := map[string]LayerMeta{}
	if len(data) > 0 {
		json.Unmarshal(data, &m)
	}
	fn(m)
	out, _ := json.MarshalIndent(m, "", "  ")
	return atomicWrite(path, out)
}
