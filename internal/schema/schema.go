// Package schema defines the internal schema model — the central abstraction
// that everything in migratex normalizes to and diffs against.
//
// Both ORM readers (Drizzle, Prisma, TypeORM via the TS sidecar) and
// DB introspectors (PG, MySQL) produce a Schema struct.
// The diff engine compares two Schema structs to produce operations.
package schema

// Schema is the top-level representation of a database schema.
// This is the "lingua franca" — every data source converts to this.
type Schema struct {
	Dialect string  `json:"dialect"` // "pg" or "mysql"
	Tables  []Table `json:"tables"`
	Enums   []Enum  `json:"enums,omitempty"`
}

// Table represents a database table with all its objects.
type Table struct {
	Name              string             `json:"name"`
	Schema            string             `json:"schema,omitempty"` // "public" for PG, db name for MySQL
	Columns           []Column           `json:"columns"`
	PrimaryKey        *PrimaryKey        `json:"primaryKey,omitempty"`
	Indexes           []Index            `json:"indexes,omitempty"`
	ForeignKeys       []ForeignKey       `json:"foreignKeys,omitempty"`
	UniqueConstraints []UniqueConstraint `json:"uniqueConstraints,omitempty"`
	CheckConstraints  []CheckConstraint  `json:"checkConstraints,omitempty"`
}

// Column represents a single column in a table.
type Column struct {
	Name       string        `json:"name"`
	Type       ColumnType    `json:"type"`
	Nullable   bool          `json:"nullable"`
	Default    *ColumnDefault `json:"default,omitempty"`
	PrimaryKey bool          `json:"primaryKey,omitempty"` // shorthand for inline PK
	Unique     bool          `json:"unique,omitempty"`     // shorthand for inline unique
}

// ColumnType describes a column's data type.
// Go doesn't have union types like TypeScript's `"int" | "text" | ...`.
// Instead, we use a Kind field (the discriminator) plus optional fields.
// This is the idiomatic Go pattern — check Kind first, then read the relevant fields.
type ColumnType struct {
	Kind string `json:"kind"` // "int", "text", "boolean", "timestamp", "json", "uuid", "serial", "float", "decimal", "date", "time", "bytea", "enum", "custom"

	// Size variants (used by int, float, serial)
	Size string `json:"size,omitempty"` // e.g., "smallint", "bigint", "real", "double", "smallserial", "bigserial"

	// Text length (used by text/varchar)
	Length int `json:"length,omitempty"` // 0 means unlimited (TEXT)

	// Decimal precision (used by decimal)
	Precision int `json:"precision,omitempty"`
	Scale     int `json:"scale,omitempty"`

	// Timestamp/time options
	WithTimezone bool `json:"withTimezone,omitempty"`

	// JSON options
	Binary bool `json:"binary,omitempty"` // true = jsonb (PG)

	// Enum reference
	EnumName string `json:"enumName,omitempty"`

	// Escape hatch for types we don't model
	Raw string `json:"raw,omitempty"`
}

// ColumnDefault represents a column's default value.
type ColumnDefault struct {
	Kind  string `json:"kind"`  // "value", "expression", "sequence"
	Value string `json:"value"` // the literal value or SQL expression
}

// PrimaryKey represents a table's primary key (can be composite).
type PrimaryKey struct {
	Name    string   `json:"name,omitempty"`
	Columns []string `json:"columns"`
}

// Index represents a database index.
type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Type    string   `json:"type,omitempty"`  // "btree", "hash", "gin", "gist"
	Where   string   `json:"where,omitempty"` // partial index predicate (PG)
}

// ForeignKeyAction defines what happens on delete/update of referenced row.
type ForeignKeyAction string

const (
	FKCascade    ForeignKeyAction = "CASCADE"
	FKSetNull    ForeignKeyAction = "SET NULL"
	FKSetDefault ForeignKeyAction = "SET DEFAULT"
	FKRestrict   ForeignKeyAction = "RESTRICT"
	FKNoAction   ForeignKeyAction = "NO ACTION"
)

// ForeignKey represents a foreign key constraint.
type ForeignKey struct {
	Name              string           `json:"name"`
	Columns           []string         `json:"columns"`
	ReferencedTable   string           `json:"referencedTable"`
	ReferencedColumns []string         `json:"referencedColumns"`
	OnDelete          ForeignKeyAction `json:"onDelete,omitempty"`
	OnUpdate          ForeignKeyAction `json:"onUpdate,omitempty"`
}

// UniqueConstraint represents a unique constraint (can be multi-column).
type UniqueConstraint struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

// CheckConstraint represents a CHECK constraint.
type CheckConstraint struct {
	Name       string `json:"name"`
	Expression string `json:"expression"`
}

// Enum represents a user-defined enum type (PG: CREATE TYPE, MySQL: inline ENUM).
type Enum struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
	Schema string   `json:"schema,omitempty"`
}
