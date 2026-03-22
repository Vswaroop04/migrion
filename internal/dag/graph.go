package dag

import (
	"fmt"
	"time"

	"github.com/vswaroop04/migratex/internal/diff"
)

// MigrationNode is a single node in the DAG — like a git commit.
// It records what changed, the SQL to apply/revert, and its parent(s).
type MigrationNode struct {
	ID          string           `json:"id"`          // content-hash (12 hex chars)
	Parents     []string         `json:"parents"`     // 0 = root, 1 = normal, 2 = merge
	Timestamp   time.Time        `json:"timestamp"`
	Description string           `json:"description"`
	Operations  []diff.Operation `json:"operations"`
	UpSQL       string           `json:"upSql"`
	DownSQL     string           `json:"downSql"`
	Checksum    string           `json:"checksum"` // hash of UpSQL for tamper detection
}

// DAG is the migration graph. Nodes are stored in a map keyed by ID.
// Heads are leaf nodes (nodes that no other node lists as a parent).
type DAG struct {
	Nodes map[string]*MigrationNode `json:"nodes"`
	Heads []string                  `json:"heads"` // current leaf node IDs
}

// NewDAG creates an empty migration graph.
func NewDAG() *DAG {
	return &DAG{
		Nodes: make(map[string]*MigrationNode),
		Heads: []string{},
	}
}

// AddNode inserts a migration node into the graph and updates heads.
// After adding, the new node becomes a head, and its parents stop being heads.
func (d *DAG) AddNode(node *MigrationNode) error {
	// Validate parents exist
	for _, parentID := range node.Parents {
		if _, exists := d.Nodes[parentID]; !exists {
			return fmt.Errorf("parent %s not found in graph", parentID)
		}
	}

	d.Nodes[node.ID] = node

	// Remove parents from heads (they now have a child)
	parentSet := make(map[string]bool)
	for _, p := range node.Parents {
		parentSet[p] = true
	}

	newHeads := []string{}
	for _, h := range d.Heads {
		if !parentSet[h] {
			newHeads = append(newHeads, h)
		}
	}
	newHeads = append(newHeads, node.ID)
	d.Heads = newHeads

	return nil
}

// TopologicalSort returns all nodes in dependency order (parents before children).
// This uses Kahn's algorithm — a BFS-based topological sort.
func (d *DAG) TopologicalSort() ([]*MigrationNode, error) {
	if len(d.Nodes) == 0 {
		return nil, nil
	}

	// Count incoming edges (number of parents) for each node
	inDegree := make(map[string]int)
	children := make(map[string][]string)

	for id, node := range d.Nodes {
		if _, ok := inDegree[id]; !ok {
			inDegree[id] = 0
		}
		for _, parentID := range node.Parents {
			children[parentID] = append(children[parentID], id)
			inDegree[id]++
		}
	}

	// Start with root nodes (0 incoming edges)
	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	var sorted []*MigrationNode
	for len(queue) > 0 {
		// Pop from queue
		id := queue[0]
		queue = queue[1:]

		sorted = append(sorted, d.Nodes[id])

		for _, childID := range children[id] {
			inDegree[childID]--
			if inDegree[childID] == 0 {
				queue = append(queue, childID)
			}
		}
	}

	// If we didn't visit all nodes, there's a cycle
	if len(sorted) != len(d.Nodes) {
		return nil, fmt.Errorf("cycle detected in migration graph")
	}

	return sorted, nil
}

// Ancestors returns the set of all ancestors of a given node (transitive parents).
func (d *DAG) Ancestors(nodeID string) map[string]bool {
	ancestors := make(map[string]bool)
	d.collectAncestors(nodeID, ancestors)
	return ancestors
}

func (d *DAG) collectAncestors(nodeID string, visited map[string]bool) {
	node, exists := d.Nodes[nodeID]
	if !exists {
		return
	}
	for _, parentID := range node.Parents {
		if !visited[parentID] {
			visited[parentID] = true
			d.collectAncestors(parentID, visited)
		}
	}
}

// Pending returns nodes that are in the graph but not yet applied to the database.
// It returns them in execution order (topologically sorted).
func (d *DAG) Pending(applied map[string]bool) ([]*MigrationNode, error) {
	sorted, err := d.TopologicalSort()
	if err != nil {
		return nil, err
	}

	var pending []*MigrationNode
	for _, node := range sorted {
		if !applied[node.ID] {
			pending = append(pending, node)
		}
	}
	return pending, nil
}

// Validate checks the graph for integrity issues.
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

func (d *DAG) Validate() ValidationResult {
	result := ValidationResult{Valid: true}

	// Check all parents exist
	for id, node := range d.Nodes {
		for _, parentID := range node.Parents {
			if _, exists := d.Nodes[parentID]; !exists {
				result.Valid = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("node %s references missing parent %s", id, parentID))
			}
		}
	}

	// Check for cycles
	_, err := d.TopologicalSort()
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
	}

	// Verify checksums
	for id, node := range d.Nodes {
		if node.Checksum != "" && node.UpSQL != "" {
			expected := ComputeChecksum(node.UpSQL)
			if node.Checksum != expected {
				result.Valid = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("checksum mismatch for migration %s: file may have been tampered", id))
			}
		}
	}

	// Verify heads are correct
	childOf := make(map[string]bool)
	for _, node := range d.Nodes {
		for _, parentID := range node.Parents {
			childOf[parentID] = true
		}
	}
	for _, head := range d.Heads {
		if childOf[head] {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("head %s has children — should not be a head", head))
		}
	}

	return result
}
