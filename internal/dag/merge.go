package dag

import (
	"fmt"
	"time"

	"github.com/vswaroop04/migratex/internal/diff"
)

// ConflictKind categorizes the type of merge conflict.
type ConflictKind string

const (
	ConflictAddVsDrop        ConflictKind = "column_add_vs_drop"
	ConflictAddVsAddDifferent ConflictKind = "column_add_vs_add_different"
	ConflictDropVsModify     ConflictKind = "table_drop_vs_modify"
	ConflictEnumDiverge      ConflictKind = "enum_diverge"
)

// Conflict describes a specific conflict between two migration branches.
type Conflict struct {
	Kind        ConflictKind `json:"kind"`
	TableName   string       `json:"tableName,omitempty"`
	ColumnName  string       `json:"columnName,omitempty"`
	Description string       `json:"description"`
}

// MergeResult is the outcome of attempting to merge two branch heads.
type MergeResult struct {
	CanAutoMerge bool         `json:"canAutoMerge"`
	Conflicts    []Conflict   `json:"conflicts,omitempty"`
	MergeNode    *MigrationNode `json:"mergeNode,omitempty"` // nil if conflicts exist
}

// DetectConflicts checks whether two sets of operations (from divergent branches) conflict.
func DetectConflicts(branchA, branchB []diff.Operation) []Conflict {
	var conflicts []Conflict

	// Build lookup sets for each branch
	aAdds := columnOps(branchA, diff.OpAddColumn)
	aDrops := columnOps(branchA, diff.OpDropColumn)
	aTableDrops := tableOps(branchA, diff.OpDropTable)
	aTableMods := tableModifications(branchA)

	bAdds := columnOps(branchB, diff.OpAddColumn)
	bDrops := columnOps(branchB, diff.OpDropColumn)
	bTableDrops := tableOps(branchB, diff.OpDropTable)
	bTableMods := tableModifications(branchB)

	// Check: column added in A, dropped in B (or vice versa)
	for key, aOp := range aAdds {
		if _, dropped := bDrops[key]; dropped {
			conflicts = append(conflicts, Conflict{
				Kind:        ConflictAddVsDrop,
				TableName:   aOp.TableName,
				ColumnName:  aOp.Column.Name,
				Description: fmt.Sprintf("branch A adds column %s.%s but branch B drops it", aOp.TableName, aOp.Column.Name),
			})
		}
	}
	for key, bOp := range bAdds {
		if _, dropped := aDrops[key]; dropped {
			conflicts = append(conflicts, Conflict{
				Kind:        ConflictAddVsDrop,
				TableName:   bOp.TableName,
				ColumnName:  bOp.Column.Name,
				Description: fmt.Sprintf("branch B adds column %s.%s but branch A drops it", bOp.TableName, bOp.Column.Name),
			})
		}
	}

	// Check: same column added in both branches with different types
	for key, aOp := range aAdds {
		if bOp, exists := bAdds[key]; exists {
			if aOp.Column.Type.Kind != bOp.Column.Type.Kind {
				conflicts = append(conflicts, Conflict{
					Kind:        ConflictAddVsAddDifferent,
					TableName:   aOp.TableName,
					ColumnName:  aOp.Column.Name,
					Description: fmt.Sprintf("both branches add %s.%s but with different types (%s vs %s)", aOp.TableName, aOp.Column.Name, aOp.Column.Type.Kind, bOp.Column.Type.Kind),
				})
			}
		}
	}

	// Check: table dropped in one branch, modified in another
	for table := range aTableDrops {
		if _, modified := bTableMods[table]; modified {
			conflicts = append(conflicts, Conflict{
				Kind:        ConflictDropVsModify,
				TableName:   table,
				Description: fmt.Sprintf("branch A drops table %s but branch B modifies it", table),
			})
		}
	}
	for table := range bTableDrops {
		if _, modified := aTableMods[table]; modified {
			conflicts = append(conflicts, Conflict{
				Kind:        ConflictDropVsModify,
				TableName:   table,
				Description: fmt.Sprintf("branch B drops table %s but branch A modifies it", table),
			})
		}
	}

	return conflicts
}

// CreateMergeNode creates a merge node that combines two branch heads.
// Only call this when DetectConflicts returns no conflicts.
func CreateMergeNode(headA, headB string, description string) *MigrationNode {
	parents := []string{headA, headB}
	id := ComputeID(parents, nil) // merge nodes have no operations

	return &MigrationNode{
		ID:          id,
		Parents:     parents,
		Timestamp:   time.Now().UTC(),
		Description: description,
		Operations:  nil, // empty — branches are compatible
		UpSQL:       "-- merge node: no operations",
		DownSQL:     "-- merge node: no operations",
		Checksum:    ComputeChecksum("-- merge node: no operations"),
	}
}

// --- helpers ---

// columnKey creates a unique key for a table.column pair
func columnKey(tableName, colName string) string {
	return tableName + "." + colName
}

// columnOps collects add/drop column operations into a map keyed by table.column
func columnOps(ops []diff.Operation, opType diff.OpType) map[string]*diff.Operation {
	result := make(map[string]*diff.Operation)
	for i := range ops {
		if ops[i].Type == opType && ops[i].Column != nil {
			key := columnKey(ops[i].TableName, ops[i].Column.Name)
			result[key] = &ops[i]
		}
	}
	return result
}

// tableOps collects table-level operations into a set keyed by table name
func tableOps(ops []diff.Operation, opType diff.OpType) map[string]bool {
	result := make(map[string]bool)
	for _, op := range ops {
		if op.Type == opType {
			result[op.TableName] = true
		}
	}
	return result
}

// tableModifications finds all tables modified (add/drop/alter column, etc.)
func tableModifications(ops []diff.Operation) map[string]bool {
	result := make(map[string]bool)
	for _, op := range ops {
		if op.TableName != "" && op.Type != diff.OpCreateTable && op.Type != diff.OpDropTable {
			result[op.TableName] = true
		}
	}
	return result
}
