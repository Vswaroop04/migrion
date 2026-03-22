// Package pg implements PostgreSQL-specific functionality.
package pg

import (
	"fmt"
	"strings"

	"github.com/vswaroop04/migratex/internal/schema"
)

// Dialect implements the SQLDialect interface for PostgreSQL.
type Dialect struct{}

func (Dialect) QuoteIdentifier(name string) string {
	return fmt.Sprintf(`"%s"`, strings.ReplaceAll(name, `"`, `""`))
}

func (Dialect) SupportsEnumType() bool { return true }

func (Dialect) RenderColumnType(ct schema.ColumnType) string {
	switch ct.Kind {
	case "serial":
		if ct.Size == "smallserial" {
			return "SMALLSERIAL"
		}
		if ct.Size == "bigserial" {
			return "BIGSERIAL"
		}
		return "SERIAL"
	case "int":
		if ct.Size == "smallint" {
			return "SMALLINT"
		}
		if ct.Size == "bigint" {
			return "BIGINT"
		}
		return "INTEGER"
	case "float":
		if ct.Size == "double" {
			return "DOUBLE PRECISION"
		}
		return "REAL"
	case "decimal":
		if ct.Precision > 0 {
			return fmt.Sprintf("NUMERIC(%d, %d)", ct.Precision, ct.Scale)
		}
		return "NUMERIC"
	case "text":
		if ct.Length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", ct.Length)
		}
		return "TEXT"
	case "boolean":
		return "BOOLEAN"
	case "date":
		return "DATE"
	case "timestamp":
		if ct.WithTimezone {
			return "TIMESTAMPTZ"
		}
		return "TIMESTAMP"
	case "time":
		if ct.WithTimezone {
			return "TIMETZ"
		}
		return "TIME"
	case "json":
		if ct.Binary {
			return "JSONB"
		}
		return "JSON"
	case "uuid":
		return "UUID"
	case "bytea":
		return "BYTEA"
	case "enum":
		return ct.EnumName // PG uses the type name directly
	case "custom":
		return ct.Raw
	default:
		return strings.ToUpper(ct.Kind)
	}
}

func (Dialect) RenderDefault(d *schema.ColumnDefault) string {
	switch d.Kind {
	case "expression":
		return d.Value
	case "sequence":
		return d.Value
	default:
		// Wrap string values in quotes
		return fmt.Sprintf("'%s'", strings.ReplaceAll(d.Value, "'", "''"))
	}
}
