// Package mysql implements MySQL-specific functionality.
package mysql

import (
	"fmt"
	"strings"

	"github.com/vswaroop04/migratex/internal/schema"
)

// Dialect implements the SQLDialect interface for MySQL.
type Dialect struct{}

func (Dialect) QuoteIdentifier(name string) string {
	return fmt.Sprintf("`%s`", strings.ReplaceAll(name, "`", "``"))
}

func (Dialect) SupportsEnumType() bool { return false } // MySQL uses inline ENUM

func (Dialect) RenderColumnType(ct schema.ColumnType) string {
	switch ct.Kind {
	case "serial":
		return "BIGINT AUTO_INCREMENT"
	case "int":
		if ct.Size == "smallint" {
			return "SMALLINT"
		}
		if ct.Size == "bigint" {
			return "BIGINT"
		}
		return "INT"
	case "float":
		if ct.Size == "double" {
			return "DOUBLE"
		}
		return "FLOAT"
	case "decimal":
		if ct.Precision > 0 {
			return fmt.Sprintf("DECIMAL(%d, %d)", ct.Precision, ct.Scale)
		}
		return "DECIMAL"
	case "text":
		if ct.Length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", ct.Length)
		}
		return "TEXT"
	case "boolean":
		return "TINYINT(1)"
	case "date":
		return "DATE"
	case "timestamp":
		return "TIMESTAMP" // MySQL doesn't differentiate tz
	case "time":
		return "TIME"
	case "json":
		return "JSON"
	case "uuid":
		return "CHAR(36)" // MySQL doesn't have native UUID
	case "bytea":
		return "BLOB"
	case "enum":
		return ct.EnumName // will be rendered inline
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
	default:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(d.Value, "'", "''"))
	}
}
