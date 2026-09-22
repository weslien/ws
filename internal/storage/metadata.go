package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
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
type Store struct {
	root string
}

func NewStore(root string) *Store { return &Store{root: root} }
func (s *Store) Root() string         { return s.root }
func (s *Store) metaDir() string  { return filepath.Join(s.root, "meta") }
func (s *Store) layersFile() string {
	_ = os.MkdirAll(s.metaDir(), 0755)
	return filepath.Join(s.metaDir(), "layers.json")
}
func (s *Store) workspacesFile() string {
	return filepath.Join(s.metaDir(), "workspaces.json")
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
	return os.WriteFile(s.layersFile(), data, 0644)
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
	return os.WriteFile(s.workspacesFile(), data, 0644)
}
