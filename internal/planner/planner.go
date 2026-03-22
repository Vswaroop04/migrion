// Package planner orders diff operations into a safe execution sequence
// and generates reverse (down) operations for rollback.
package planner

import "github.com/vswaroop04/migratex/internal/diff"

// operationPriority defines the safe execution order.
// Lower number = execute first. This ensures dependencies are met:
// - Enums must exist before columns reference them
// - Tables must exist before columns/indexes/FKs are added
// - FKs/indexes must be dropped before the table/column they reference
var operationPriority = map[diff.OpType]int{
	diff.OpCreateEnum:           1,
	diff.OpAlterEnum:            2,
	diff.OpCreateTable:          3,
	diff.OpAddColumn:            4,
	diff.OpAlterColumn:          5,
	diff.OpAddPrimaryKey:        6,
	diff.OpAddUniqueConstraint:  7,
	diff.OpAddCheckConstraint:   8,
	diff.OpCreateIndex:          9,
	diff.OpAddForeignKey:        10, // FKs last — referenced table must exist
	diff.OpDropForeignKey:       11, // drop FKs before dropping tables/columns
	diff.OpDropIndex:            12,
	diff.OpDropCheckConstraint:  13,
	diff.OpDropUniqueConstraint: 14,
	diff.OpDropPrimaryKey:       15,
	diff.OpDropColumn:           16,
	diff.OpDropTable:            17, // tables last — everything else must be cleaned up
	diff.OpDropEnum:             18,
}

// PlanMigration sorts operations into safe execution order.
// It uses a stable sort to preserve relative order within the same priority.
func PlanMigration(ops []diff.Operation) []diff.Operation {
	// Separate creates/adds from drops — drops need reverse ordering
	// within the same priority level
	sorted := make([]diff.Operation, len(ops))
	copy(sorted, ops)

	// Simple priority-based sort (stable)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && priority(sorted[j]) < priority(sorted[j-1]); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	return sorted
}

func priority(op diff.Operation) int {
	if p, ok := operationPriority[op.Type]; ok {
		return p
	}
	return 100 // unknown ops go last
}
