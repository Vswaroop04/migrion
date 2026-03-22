// Package diff implements the schema diff engine — the heart of migratex.
// It compares two Schema structs and produces a list of Operations
// describing how to transform one into the other.
package diff

import "github.com/vswaroop04/migratex/internal/schema"

// OpType is the type of a diff operation.
// Go doesn't have enums like TS. The idiomatic pattern is:
//   type MyType string
//   const ( Value1 MyType = "value1" ... )
type OpType string

const (
	OpCreateTable          OpType = "create_table"
	OpDropTable            OpType = "drop_table"
	OpAddColumn            OpType = "add_column"
	OpDropColumn           OpType = "drop_column"
	OpAlterColumn          OpType = "alter_column"
	OpCreateIndex          OpType = "create_index"
	OpDropIndex            OpType = "drop_index"
	OpAddForeignKey        OpType = "add_foreign_key"
	OpDropForeignKey       OpType = "drop_foreign_key"
	OpAddPrimaryKey        OpType = "add_primary_key"
	OpDropPrimaryKey       OpType = "drop_primary_key"
	OpAddUniqueConstraint  OpType = "add_unique_constraint"
	OpDropUniqueConstraint OpType = "drop_unique_constraint"
	OpAddCheckConstraint   OpType = "add_check_constraint"
	OpDropCheckConstraint  OpType = "drop_check_constraint"
	OpCreateEnum           OpType = "create_enum"
	OpDropEnum             OpType = "drop_enum"
	OpAlterEnum            OpType = "alter_enum"
)

// Operation represents a single schema change.
// Since Go doesn't have union types, we use a single struct with optional fields.
// Check the Type field to know which fields are populated.
type Operation struct {
	Type      OpType `json:"type"`
	TableName string `json:"tableName,omitempty"`

	// For create_table / drop_table — full table snapshot (needed for reverse migrations)
	Table *schema.Table `json:"table,omitempty"`

	// For add_column / drop_column
	Column *schema.Column `json:"column,omitempty"`

	// For alter_column
	ColumnName    string        `json:"columnName,omitempty"`
	ColumnChanges *ColumnChanges `json:"columnChanges,omitempty"`

	// For create_index / drop_index
	Index     *schema.Index `json:"index,omitempty"`
	IndexName string        `json:"indexName,omitempty"`

	// For add_foreign_key / drop_foreign_key
	ForeignKey *schema.ForeignKey `json:"foreignKey,omitempty"`
	FKName     string             `json:"fkName,omitempty"`

	// For add/drop primary key
	PrimaryKey *schema.PrimaryKey `json:"primaryKey,omitempty"`
	PKName     string             `json:"pkName,omitempty"`

	// For add/drop unique constraint
	UniqueConstraint *schema.UniqueConstraint `json:"uniqueConstraint,omitempty"`
	ConstraintName   string                   `json:"constraintName,omitempty"`

	// For add/drop check constraint
	CheckConstraint *schema.CheckConstraint `json:"checkConstraint,omitempty"`

	// For create_enum / drop_enum / alter_enum
	Enum       *schema.Enum `json:"enum,omitempty"`
	EnumName   string       `json:"enumName,omitempty"`
	AddValues  []string     `json:"addValues,omitempty"`
	DropValues []string     `json:"dropValues,omitempty"`
}

// ColumnChanges describes what changed in an alter_column operation.
type ColumnChanges struct {
	TypeChange     *TypeChange    `json:"typeChange,omitempty"`
	NullableChange *BoolChange    `json:"nullableChange,omitempty"`
	DefaultChange  *DefaultChange `json:"defaultChange,omitempty"`
}

// TypeChange records a column type change.
type TypeChange struct {
	From schema.ColumnType `json:"from"`
	To   schema.ColumnType `json:"to"`
}

// BoolChange records a boolean field change (e.g., nullable).
type BoolChange struct {
	From bool `json:"from"`
	To   bool `json:"to"`
}

// DefaultChange records a default value change.
type DefaultChange struct {
	From *schema.ColumnDefault `json:"from,omitempty"`
	To   *schema.ColumnDefault `json:"to,omitempty"`
}
