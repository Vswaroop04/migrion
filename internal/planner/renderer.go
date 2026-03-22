package planner

import (
	"fmt"
	"strings"

	"github.com/vswaroop04/migratex/internal/diff"
	"github.com/vswaroop04/migratex/internal/schema"
)

// SQLDialect defines how to generate SQL for a specific database.
type SQLDialect interface {
	RenderColumnType(ct schema.ColumnType) string
	RenderDefault(d *schema.ColumnDefault) string
	SupportsEnumType() bool // PG: true (CREATE TYPE), MySQL: false (inline ENUM)
	QuoteIdentifier(name string) string
}

// RenderMigrationSQL generates up and down SQL for a set of operations.
func RenderMigrationSQL(ops []diff.Operation, dialect SQLDialect) (up, down string) {
	planned := PlanMigration(ops)

	var upLines []string
	for _, op := range planned {
		upLines = append(upLines, renderOperation(op, dialect))
	}

	reversed := ReverseAll(planned)
	var downLines []string
	for _, op := range reversed {
		downLines = append(downLines, renderOperation(op, dialect))
	}

	up = "BEGIN;\n\n" + strings.Join(upLines, "\n\n") + "\n\nCOMMIT;\n"
	down = "BEGIN;\n\n" + strings.Join(downLines, "\n\n") + "\n\nCOMMIT;\n"
	return up, down
}

func renderOperation(op diff.Operation, d SQLDialect) string {
	q := d.QuoteIdentifier

	switch op.Type {
	case diff.OpCreateTable:
		return renderCreateTable(op, d)

	case diff.OpDropTable:
		return fmt.Sprintf("DROP TABLE %s;", q(op.TableName))

	case diff.OpAddColumn:
		col := renderColumnDef(op.Column, d)
		return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", q(op.TableName), col)

	case diff.OpDropColumn:
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", q(op.TableName), q(op.Column.Name))

	case diff.OpAlterColumn:
		return renderAlterColumn(op, d)

	case diff.OpCreateIndex:
		return renderCreateIndex(op, d)

	case diff.OpDropIndex:
		return fmt.Sprintf("DROP INDEX %s;", q(op.IndexName))

	case diff.OpAddForeignKey:
		return renderAddForeignKey(op, d)

	case diff.OpDropForeignKey:
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", q(op.TableName), q(op.FKName))

	case diff.OpAddPrimaryKey:
		cols := quoteJoin(op.PrimaryKey.Columns, d)
		return fmt.Sprintf("ALTER TABLE %s ADD PRIMARY KEY (%s);", q(op.TableName), cols)

	case diff.OpDropPrimaryKey:
		if op.PKName != "" {
			return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", q(op.TableName), q(op.PKName))
		}
		return fmt.Sprintf("ALTER TABLE %s DROP PRIMARY KEY;", q(op.TableName))

	case diff.OpAddUniqueConstraint:
		cols := quoteJoin(op.UniqueConstraint.Columns, d)
		return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s UNIQUE (%s);",
			q(op.TableName), q(op.UniqueConstraint.Name), cols)

	case diff.OpDropUniqueConstraint:
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", q(op.TableName), q(op.ConstraintName))

	case diff.OpAddCheckConstraint:
		return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s);",
			q(op.TableName), q(op.CheckConstraint.Name), op.CheckConstraint.Expression)

	case diff.OpDropCheckConstraint:
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", q(op.TableName), q(op.ConstraintName))

	case diff.OpCreateEnum:
		if d.SupportsEnumType() {
			vals := make([]string, len(op.Enum.Values))
			for i, v := range op.Enum.Values {
				vals[i] = fmt.Sprintf("'%s'", v)
			}
			return fmt.Sprintf("CREATE TYPE %s AS ENUM (%s);", q(op.Enum.Name), strings.Join(vals, ", "))
		}
		return "-- enum creation handled inline for this dialect"

	case diff.OpDropEnum:
		if d.SupportsEnumType() {
			return fmt.Sprintf("DROP TYPE %s;", q(op.EnumName))
		}
		return "-- enum drop handled inline for this dialect"

	case diff.OpAlterEnum:
		if d.SupportsEnumType() {
			var lines []string
			for _, v := range op.AddValues {
				lines = append(lines, fmt.Sprintf("ALTER TYPE %s ADD VALUE '%s';", q(op.EnumName), v))
			}
			if len(op.DropValues) > 0 {
				lines = append(lines, fmt.Sprintf("-- WARNING: PostgreSQL cannot remove enum values. Dropped values: %v", op.DropValues))
			}
			return strings.Join(lines, "\n")
		}
		return "-- enum alter handled inline for this dialect"
	}

	return fmt.Sprintf("-- unknown operation: %s", op.Type)
}

func renderCreateTable(op diff.Operation, d SQLDialect) string {
	q := d.QuoteIdentifier
	var lines []string

	for _, col := range op.Table.Columns {
		lines = append(lines, "  "+renderColumnDef(&col, d))
	}

	if op.Table.PrimaryKey != nil {
		cols := quoteJoin(op.Table.PrimaryKey.Columns, d)
		lines = append(lines, fmt.Sprintf("  PRIMARY KEY (%s)", cols))
	}

	return fmt.Sprintf("CREATE TABLE %s (\n%s\n);", q(op.TableName), strings.Join(lines, ",\n"))
}

func renderColumnDef(col *schema.Column, d SQLDialect) string {
	q := d.QuoteIdentifier
	parts := []string{q(col.Name), d.RenderColumnType(col.Type)}

	if !col.Nullable {
		parts = append(parts, "NOT NULL")
	}

	if col.Default != nil {
		parts = append(parts, "DEFAULT "+d.RenderDefault(col.Default))
	}

	if col.Unique {
		parts = append(parts, "UNIQUE")
	}

	return strings.Join(parts, " ")
}

func renderAlterColumn(op diff.Operation, d SQLDialect) string {
	q := d.QuoteIdentifier
	var lines []string

	if op.ColumnChanges.TypeChange != nil {
		newType := d.RenderColumnType(op.ColumnChanges.TypeChange.To)
		lines = append(lines, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;",
			q(op.TableName), q(op.ColumnName), newType))
	}

	if op.ColumnChanges.NullableChange != nil {
		if op.ColumnChanges.NullableChange.To {
			lines = append(lines, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;",
				q(op.TableName), q(op.ColumnName)))
		} else {
			lines = append(lines, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;",
				q(op.TableName), q(op.ColumnName)))
		}
	}

	if op.ColumnChanges.DefaultChange != nil {
		if op.ColumnChanges.DefaultChange.To != nil {
			defVal := d.RenderDefault(op.ColumnChanges.DefaultChange.To)
			lines = append(lines, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;",
				q(op.TableName), q(op.ColumnName), defVal))
		} else {
			lines = append(lines, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;",
				q(op.TableName), q(op.ColumnName)))
		}
	}

	return strings.Join(lines, "\n")
}

func renderCreateIndex(op diff.Operation, d SQLDialect) string {
	q := d.QuoteIdentifier
	unique := ""
	if op.Index.Unique {
		unique = "UNIQUE "
	}

	cols := quoteJoin(op.Index.Columns, d)
	sql := fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)", unique, q(op.Index.Name), q(op.TableName), cols)

	if op.Index.Type != "" && op.Index.Type != "btree" {
		sql = fmt.Sprintf("CREATE %sINDEX %s ON %s USING %s (%s)",
			unique, q(op.Index.Name), q(op.TableName), op.Index.Type, cols)
	}

	if op.Index.Where != "" {
		sql += " WHERE " + op.Index.Where
	}

	return sql + ";"
}

func renderAddForeignKey(op diff.Operation, d SQLDialect) string {
	q := d.QuoteIdentifier
	fk := op.ForeignKey
	cols := quoteJoin(fk.Columns, d)
	refCols := quoteJoin(fk.ReferencedColumns, d)

	sql := fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
		q(op.TableName), q(fk.Name), cols, q(fk.ReferencedTable), refCols)

	if fk.OnDelete != "" {
		sql += " ON DELETE " + string(fk.OnDelete)
	}
	if fk.OnUpdate != "" {
		sql += " ON UPDATE " + string(fk.OnUpdate)
	}

	return sql + ";"
}

func quoteJoin(names []string, d SQLDialect) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = d.QuoteIdentifier(n)
	}
	return strings.Join(quoted, ", ")
}
