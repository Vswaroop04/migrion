package planner

import "github.com/vswaroop04/migratex/internal/diff"

// ReverseOperation generates the opposite of a given operation.
// This is used to auto-generate "down" migrations for rollback.
//
// Mapping:
//
//	create_table <-> drop_table (uses table snapshot)
//	add_column   <-> drop_column (uses column snapshot)
//	alter_column -> alter_column (swap from/to)
//	create_index <-> drop_index
//	add_fk       <-> drop_fk
//	create_enum  <-> drop_enum
func ReverseOperation(op diff.Operation) diff.Operation {
	switch op.Type {
	case diff.OpCreateTable:
		return diff.Operation{
			Type:      diff.OpDropTable,
			TableName: op.TableName,
			Table:     op.Table,
		}

	case diff.OpDropTable:
		return diff.Operation{
			Type:      diff.OpCreateTable,
			TableName: op.TableName,
			Table:     op.Table,
		}

	case diff.OpAddColumn:
		return diff.Operation{
			Type:      diff.OpDropColumn,
			TableName: op.TableName,
			Column:    op.Column,
		}

	case diff.OpDropColumn:
		return diff.Operation{
			Type:      diff.OpAddColumn,
			TableName: op.TableName,
			Column:    op.Column,
		}

	case diff.OpAlterColumn:
		// Swap from/to in each change
		reversed := &diff.ColumnChanges{}
		if op.ColumnChanges.TypeChange != nil {
			reversed.TypeChange = &diff.TypeChange{
				From: op.ColumnChanges.TypeChange.To,
				To:   op.ColumnChanges.TypeChange.From,
			}
		}
		if op.ColumnChanges.NullableChange != nil {
			reversed.NullableChange = &diff.BoolChange{
				From: op.ColumnChanges.NullableChange.To,
				To:   op.ColumnChanges.NullableChange.From,
			}
		}
		if op.ColumnChanges.DefaultChange != nil {
			reversed.DefaultChange = &diff.DefaultChange{
				From: op.ColumnChanges.DefaultChange.To,
				To:   op.ColumnChanges.DefaultChange.From,
			}
		}
		return diff.Operation{
			Type:          diff.OpAlterColumn,
			TableName:     op.TableName,
			ColumnName:    op.ColumnName,
			ColumnChanges: reversed,
		}

	case diff.OpCreateIndex:
		return diff.Operation{
			Type:      diff.OpDropIndex,
			TableName: op.TableName,
			IndexName: op.Index.Name,
			Index:     op.Index,
		}

	case diff.OpDropIndex:
		return diff.Operation{
			Type:      diff.OpCreateIndex,
			TableName: op.TableName,
			Index:     op.Index,
		}

	case diff.OpAddForeignKey:
		return diff.Operation{
			Type:       diff.OpDropForeignKey,
			TableName:  op.TableName,
			FKName:     op.ForeignKey.Name,
			ForeignKey: op.ForeignKey,
		}

	case diff.OpDropForeignKey:
		return diff.Operation{
			Type:       diff.OpAddForeignKey,
			TableName:  op.TableName,
			ForeignKey: op.ForeignKey,
		}

	case diff.OpAddPrimaryKey:
		return diff.Operation{
			Type:       diff.OpDropPrimaryKey,
			TableName:  op.TableName,
			PKName:     op.PrimaryKey.Name,
			PrimaryKey: op.PrimaryKey,
		}

	case diff.OpDropPrimaryKey:
		return diff.Operation{
			Type:       diff.OpAddPrimaryKey,
			TableName:  op.TableName,
			PrimaryKey: op.PrimaryKey,
		}

	case diff.OpAddUniqueConstraint:
		return diff.Operation{
			Type:             diff.OpDropUniqueConstraint,
			TableName:        op.TableName,
			ConstraintName:   op.UniqueConstraint.Name,
			UniqueConstraint: op.UniqueConstraint,
		}

	case diff.OpDropUniqueConstraint:
		return diff.Operation{
			Type:             diff.OpAddUniqueConstraint,
			TableName:        op.TableName,
			UniqueConstraint: op.UniqueConstraint,
		}

	case diff.OpAddCheckConstraint:
		return diff.Operation{
			Type:            diff.OpDropCheckConstraint,
			TableName:       op.TableName,
			ConstraintName:  op.CheckConstraint.Name,
			CheckConstraint: op.CheckConstraint,
		}

	case diff.OpDropCheckConstraint:
		return diff.Operation{
			Type:            diff.OpAddCheckConstraint,
			TableName:       op.TableName,
			CheckConstraint: op.CheckConstraint,
		}

	case diff.OpCreateEnum:
		return diff.Operation{
			Type:     diff.OpDropEnum,
			EnumName: op.Enum.Name,
			Enum:     op.Enum,
		}

	case diff.OpDropEnum:
		return diff.Operation{
			Type: diff.OpCreateEnum,
			Enum: op.Enum,
		}

	case diff.OpAlterEnum:
		return diff.Operation{
			Type:       diff.OpAlterEnum,
			EnumName:   op.EnumName,
			AddValues:  op.DropValues,
			DropValues: op.AddValues,
		}
	}

	// Unknown operation — return as-is
	return op
}

// ReverseAll generates reverse operations for a list, in reverse order.
// This is the "down" migration: undo everything in the opposite sequence.
func ReverseAll(ops []diff.Operation) []diff.Operation {
	reversed := make([]diff.Operation, len(ops))
	for i, op := range ops {
		reversed[len(ops)-1-i] = ReverseOperation(op)
	}
	return reversed
}
