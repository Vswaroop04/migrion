// Package drift detects when the database schema has changed outside of migrations.
package drift

import (
	"fmt"
	"strings"

	"github.com/vswaroop04/migratex/internal/diff"
	"github.com/vswaroop04/migratex/internal/schema"
)

// Result describes drift detection findings.
type Result struct {
	HasDrift   bool             `json:"hasDrift"`
	Operations []diff.Operation `json:"operations,omitempty"` // changes found out-of-band
	Summary    string           `json:"summary"`
}

// Detect compares the expected schema (from ORM) with the actual database schema.
// Any differences indicate drift — changes made outside the migration system.
func Detect(expected, actual *schema.Schema) *Result {
	ops := diff.DiffSchemas(expected, actual)

	if len(ops) == 0 {
		return &Result{
			HasDrift: false,
			Summary:  "No drift detected. Database matches expected schema.",
		}
	}

	// Build human-readable summary
	var lines []string
	for _, op := range ops {
		lines = append(lines, describeDrift(op))
	}

	return &Result{
		HasDrift:   true,
		Operations: ops,
		Summary:    fmt.Sprintf("Drift detected (%d changes):\n%s", len(ops), strings.Join(lines, "\n")),
	}
}

func describeDrift(op diff.Operation) string {
	switch op.Type {
	case diff.OpCreateTable:
		return fmt.Sprintf("  + table %s expected but missing in database", op.TableName)
	case diff.OpDropTable:
		return fmt.Sprintf("  - table %s exists in database but not in schema", op.TableName)
	case diff.OpAddColumn:
		return fmt.Sprintf("  + column %s.%s expected but missing in database", op.TableName, op.Column.Name)
	case diff.OpDropColumn:
		return fmt.Sprintf("  - column %s.%s exists in database but not in schema", op.TableName, op.Column.Name)
	case diff.OpAlterColumn:
		return fmt.Sprintf("  ~ column %s.%s has different definition", op.TableName, op.ColumnName)
	case diff.OpCreateIndex:
		return fmt.Sprintf("  + index %s on %s expected but missing", op.Index.Name, op.TableName)
	case diff.OpDropIndex:
		return fmt.Sprintf("  - index %s on %s exists but not in schema", op.IndexName, op.TableName)
	case diff.OpAddForeignKey:
		return fmt.Sprintf("  + foreign key %s on %s expected but missing", op.ForeignKey.Name, op.TableName)
	case diff.OpDropForeignKey:
		return fmt.Sprintf("  - foreign key %s on %s exists but not in schema", op.FKName, op.TableName)
	case diff.OpCreateEnum:
		return fmt.Sprintf("  + enum %s expected but missing", op.Enum.Name)
	case diff.OpDropEnum:
		return fmt.Sprintf("  - enum %s exists but not in schema", op.EnumName)
	case diff.OpAlterEnum:
		return fmt.Sprintf("  ~ enum %s has different values", op.EnumName)
	default:
		return fmt.Sprintf("  ? %s on %s", op.Type, op.TableName)
	}
}
