package dag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Store handles reading and writing migration files to disk.
// Layout:
//
//	migrations/
//	  graph.json                 # lightweight index (heads + node metadata)
//	  <hash>/
//	    migration.json           # full MigrationNode
//	    up.sql                   # apply SQL
//	    down.sql                 # revert SQL
type Store struct {
	Dir string // path to migrations directory
}

// NewStore creates a Store for the given directory.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

// graphIndex is the lightweight index stored in graph.json
type graphIndex struct {
	Heads []string           `json:"heads"`
	Nodes map[string]nodeMeta `json:"nodes"`
}

type nodeMeta struct {
	ID          string   `json:"id"`
	Parents     []string `json:"parents"`
	Timestamp   string   `json:"timestamp"`
	Description string   `json:"description"`
	Checksum    string   `json:"checksum"`
}

// LoadGraph reads the migration graph from disk.
// If no graph exists yet, returns an empty DAG.
func (s *Store) LoadGraph() (*DAG, error) {
	dag := NewDAG()

	indexPath := filepath.Join(s.Dir, "graph.json")
	data, err := os.ReadFile(indexPath)
	if os.IsNotExist(err) {
		return dag, nil // fresh project, no migrations yet
	}
	if err != nil {
		return nil, fmt.Errorf("reading graph.json: %w", err)
	}

	var index graphIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("parsing graph.json: %w", err)
	}

	// Load full nodes from their individual directories
	for id := range index.Nodes {
		node, err := s.ReadNode(id)
		if err != nil {
			return nil, fmt.Errorf("loading migration %s: %w", id, err)
		}
		dag.Nodes[id] = node
	}
	dag.Heads = index.Heads

	return dag, nil
}

// SaveNode writes a migration node to disk (creates its directory + files).
func (s *Store) SaveNode(node *MigrationNode) error {
	nodeDir := filepath.Join(s.Dir, node.ID)
	if err := os.MkdirAll(nodeDir, 0o755); err != nil {
		return fmt.Errorf("creating migration dir: %w", err)
	}

	// Write migration.json
	data, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling migration: %w", err)
	}
	if err := os.WriteFile(filepath.Join(nodeDir, "migration.json"), data, 0o644); err != nil {
		return fmt.Errorf("writing migration.json: %w", err)
	}

	// Write up.sql
	if err := os.WriteFile(filepath.Join(nodeDir, "up.sql"), []byte(node.UpSQL), 0o644); err != nil {
		return fmt.Errorf("writing up.sql: %w", err)
	}

	// Write down.sql
	if err := os.WriteFile(filepath.Join(nodeDir, "down.sql"), []byte(node.DownSQL), 0o644); err != nil {
		return fmt.Errorf("writing down.sql: %w", err)
	}

	return nil
}

// ReadNode loads a single migration node from its directory.
func (s *Store) ReadNode(id string) (*MigrationNode, error) {
	path := filepath.Join(s.Dir, id, "migration.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading migration %s: %w", id, err)
	}

	var node MigrationNode
	if err := json.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("parsing migration %s: %w", id, err)
	}

	return &node, nil
}

// UpdateGraph writes the graph index (graph.json) to disk.
// This is the lightweight index for fast traversal.
func (s *Store) UpdateGraph(dag *DAG) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("creating migrations dir: %w", err)
	}

	index := graphIndex{
		Heads: dag.Heads,
		Nodes: make(map[string]nodeMeta),
	}

	for id, node := range dag.Nodes {
		index.Nodes[id] = nodeMeta{
			ID:          node.ID,
			Parents:     node.Parents,
			Timestamp:   node.Timestamp.UTC().Format("2006-01-02T15:04:05Z"),
			Description: node.Description,
			Checksum:    node.Checksum,
		}
	}

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling graph index: %w", err)
	}

	return os.WriteFile(filepath.Join(s.Dir, "graph.json"), data, 0o644)
}
