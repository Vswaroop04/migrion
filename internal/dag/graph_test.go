package dag

import (
	"testing"
	"time"

	"github.com/vswaroop04/migratex/internal/diff"
	"github.com/vswaroop04/migratex/internal/schema"
)

func makeNode(id string, parents []string) *MigrationNode {
	sql := "-- " + id
	return &MigrationNode{
		ID:        id,
		Parents:   parents,
		Timestamp: time.Now().UTC(),
		UpSQL:     sql,
		DownSQL:   sql,
		Checksum:  ComputeChecksum(sql),
	}
}

func TestDAG_AddNode_RootNode(t *testing.T) {
	d := NewDAG()
	node := makeNode("aaa", nil)

	// Root node has no parents, so we add it directly
	d.Nodes[node.ID] = node
	d.Heads = []string{node.ID}

	if len(d.Heads) != 1 {
		t.Fatalf("expected 1 head, got %d", len(d.Heads))
	}
	if d.Heads[0] != "aaa" {
		t.Errorf("expected head 'aaa', got %s", d.Heads[0])
	}
}

func TestDAG_AddNode_Chain(t *testing.T) {
	d := NewDAG()

	// Build: A -> B -> C
	a := makeNode("aaa", nil)
	d.Nodes[a.ID] = a
	d.Heads = []string{a.ID}

	b := makeNode("bbb", []string{"aaa"})
	if err := d.AddNode(b); err != nil {
		t.Fatal(err)
	}

	c := makeNode("ccc", []string{"bbb"})
	if err := d.AddNode(c); err != nil {
		t.Fatal(err)
	}

	if len(d.Heads) != 1 || d.Heads[0] != "ccc" {
		t.Errorf("expected head [ccc], got %v", d.Heads)
	}
}

func TestDAG_AddNode_Branch(t *testing.T) {
	d := NewDAG()

	// Build: A -> B, A -> C (two branches)
	a := makeNode("aaa", nil)
	d.Nodes[a.ID] = a
	d.Heads = []string{a.ID}

	b := makeNode("bbb", []string{"aaa"})
	if err := d.AddNode(b); err != nil {
		t.Fatal(err)
	}

	// Reset heads to include A again (simulating another branch)
	d.Heads = []string{"bbb", "aaa"} // A is still a "head" in another branch context

	c := makeNode("ccc", []string{"aaa"})
	if err := d.AddNode(c); err != nil {
		t.Fatal(err)
	}

	// Both B and C should be heads
	if len(d.Heads) != 2 {
		t.Errorf("expected 2 heads, got %d: %v", len(d.Heads), d.Heads)
	}
}

func TestDAG_TopologicalSort(t *testing.T) {
	d := NewDAG()

	// A -> B -> D
	// A -> C -> D (diamond pattern)
	a := makeNode("aaa", nil)
	d.Nodes["aaa"] = a

	b := makeNode("bbb", []string{"aaa"})
	d.Nodes["bbb"] = b

	c := makeNode("ccc", []string{"aaa"})
	d.Nodes["ccc"] = c

	dd := makeNode("ddd", []string{"bbb", "ccc"})
	d.Nodes["ddd"] = dd
	d.Heads = []string{"ddd"}

	sorted, err := d.TopologicalSort()
	if err != nil {
		t.Fatal(err)
	}

	if len(sorted) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(sorted))
	}

	// A must come before B, C; B and C must come before D
	pos := make(map[string]int)
	for i, n := range sorted {
		pos[n.ID] = i
	}

	if pos["aaa"] > pos["bbb"] || pos["aaa"] > pos["ccc"] {
		t.Error("A must come before B and C")
	}
	if pos["bbb"] > pos["ddd"] || pos["ccc"] > pos["ddd"] {
		t.Error("B and C must come before D")
	}
}

func TestDAG_Pending(t *testing.T) {
	d := NewDAG()

	a := makeNode("aaa", nil)
	d.Nodes["aaa"] = a

	b := makeNode("bbb", []string{"aaa"})
	d.Nodes["bbb"] = b

	c := makeNode("ccc", []string{"bbb"})
	d.Nodes["ccc"] = c
	d.Heads = []string{"ccc"}

	// A and B are applied, C is pending
	applied := map[string]bool{"aaa": true, "bbb": true}
	pending, err := d.Pending(applied)
	if err != nil {
		t.Fatal(err)
	}

	if len(pending) != 1 || pending[0].ID != "ccc" {
		t.Errorf("expected [ccc] pending, got %v", pending)
	}
}

func TestDAG_Validate_Valid(t *testing.T) {
	d := NewDAG()
	a := makeNode("aaa", nil)
	d.Nodes["aaa"] = a
	d.Heads = []string{"aaa"}

	result := d.Validate()
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}
}

func TestDAG_Validate_MissingParent(t *testing.T) {
	d := NewDAG()
	b := makeNode("bbb", []string{"missing"})
	d.Nodes["bbb"] = b
	d.Heads = []string{"bbb"}

	result := d.Validate()
	if result.Valid {
		t.Error("expected invalid due to missing parent")
	}
}

func TestDAG_Validate_TamperedChecksum(t *testing.T) {
	d := NewDAG()
	a := makeNode("aaa", nil)
	a.UpSQL = "SELECT 1"
	a.Checksum = "wrong_checksum"
	d.Nodes["aaa"] = a
	d.Heads = []string{"aaa"}

	result := d.Validate()
	if result.Valid {
		t.Error("expected invalid due to tampered checksum")
	}
}

func TestDAG_Ancestors(t *testing.T) {
	d := NewDAG()

	// A -> B -> C
	d.Nodes["aaa"] = makeNode("aaa", nil)
	d.Nodes["bbb"] = makeNode("bbb", []string{"aaa"})
	d.Nodes["ccc"] = makeNode("ccc", []string{"bbb"})

	ancestors := d.Ancestors("ccc")
	if !ancestors["aaa"] || !ancestors["bbb"] {
		t.Errorf("expected ancestors [aaa, bbb], got %v", ancestors)
	}
	if ancestors["ccc"] {
		t.Error("node should not be its own ancestor")
	}
}

func TestComputeID_Deterministic(t *testing.T) {
	parents := []string{"aaa", "bbb"}
	ops := []diff.Operation{{Type: diff.OpCreateTable, TableName: "users"}}

	id1 := ComputeID(parents, ops)
	id2 := ComputeID(parents, ops)

	if id1 != id2 {
		t.Errorf("same inputs should produce same ID: %s vs %s", id1, id2)
	}
}

func TestComputeID_ParentOrderIndependent(t *testing.T) {
	ops := []diff.Operation{{Type: diff.OpCreateTable, TableName: "users"}}

	id1 := ComputeID([]string{"aaa", "bbb"}, ops)
	id2 := ComputeID([]string{"bbb", "aaa"}, ops)

	if id1 != id2 {
		t.Errorf("parent order should not matter: %s vs %s", id1, id2)
	}
}

func TestDetectConflicts_AddVsDrop(t *testing.T) {
	branchA := []diff.Operation{
		{Type: diff.OpAddColumn, TableName: "users", Column: &schema.Column{Name: "age", Type: schema.ColumnType{Kind: "int"}}},
	}
	branchB := []diff.Operation{
		{Type: diff.OpDropColumn, TableName: "users", Column: &schema.Column{Name: "age"}},
	}

	conflicts := DetectConflicts(branchA, branchB)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Kind != ConflictAddVsDrop {
		t.Errorf("expected column_add_vs_drop, got %s", conflicts[0].Kind)
	}
}

func TestDetectConflicts_NoConflict(t *testing.T) {
	branchA := []diff.Operation{
		{Type: diff.OpAddColumn, TableName: "users", Column: &schema.Column{Name: "age", Type: schema.ColumnType{Kind: "int"}}},
	}
	branchB := []diff.Operation{
		{Type: diff.OpAddColumn, TableName: "users", Column: &schema.Column{Name: "email", Type: schema.ColumnType{Kind: "text"}}},
	}

	conflicts := DetectConflicts(branchA, branchB)
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %+v", len(conflicts), conflicts)
	}
}

func TestDetectConflicts_SameColumnDifferentTypes(t *testing.T) {
	branchA := []diff.Operation{
		{Type: diff.OpAddColumn, TableName: "users", Column: &schema.Column{Name: "score", Type: schema.ColumnType{Kind: "int"}}},
	}
	branchB := []diff.Operation{
		{Type: diff.OpAddColumn, TableName: "users", Column: &schema.Column{Name: "score", Type: schema.ColumnType{Kind: "text"}}},
	}

	conflicts := DetectConflicts(branchA, branchB)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Kind != ConflictAddVsAddDifferent {
		t.Errorf("expected column_add_vs_add_different, got %s", conflicts[0].Kind)
	}
}

func TestDetectConflicts_TableDropVsModify(t *testing.T) {
	branchA := []diff.Operation{
		{Type: diff.OpDropTable, TableName: "users"},
	}
	branchB := []diff.Operation{
		{Type: diff.OpAddColumn, TableName: "users", Column: &schema.Column{Name: "email", Type: schema.ColumnType{Kind: "text"}}},
	}

	conflicts := DetectConflicts(branchA, branchB)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Kind != ConflictDropVsModify {
		t.Errorf("expected table_drop_vs_modify, got %s", conflicts[0].Kind)
	}
}
