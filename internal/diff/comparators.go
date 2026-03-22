package diff

import (
	"slices"
	"strings"

	"github.com/vswaroop04/migratex/internal/schema"
)

// typeAliases maps dialect-specific type names to canonical names.
// This prevents false diffs like "int4" vs "integer" in PostgreSQL.
var typeAliases = map[string]string{
	"int4":              "int",
	"int8":              "bigint",
	"int2":              "smallint",
	"float4":            "real",
	"float8":            "double",
	"bool":              "boolean",
	"character varying": "text",
	"varchar":           "text",
	"timestamptz":       "timestamp",
	"timetz":            "time",
}

// normalizeKind maps type aliases to their canonical form.
func normalizeKind(kind string) string {
	lower := strings.ToLower(kind)
	if canonical, ok := typeAliases[lower]; ok {
		return canonical
	}
	return lower
}

// columnTypesEqual compares two ColumnTypes after normalization.
func columnTypesEqual(a, b schema.ColumnType) bool {
	ak := normalizeKind(a.Kind)
	bk := normalizeKind(b.Kind)

	if ak != bk {
		return false
	}

	// Compare type-specific fields based on kind
	switch ak {
	case "int", "smallint", "bigint", "serial":
		return normalizeKind(a.Size) == normalizeKind(b.Size)
	case "text":
		return a.Length == b.Length
	case "decimal":
		return a.Precision == b.Precision && a.Scale == b.Scale
	case "timestamp", "time":
		return a.WithTimezone == b.WithTimezone
	case "json":
		return a.Binary == b.Binary
	case "enum":
		return a.EnumName == b.EnumName
	case "custom":
		return strings.EqualFold(a.Raw, b.Raw)
	}
	return true
}

// defaultsEqual compares two ColumnDefaults.
func defaultsEqual(a, b *schema.ColumnDefault) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Kind == b.Kind && a.Value == b.Value
}

// compareColumn compares two columns and returns changes, or nil if identical.
func compareColumn(expected, actual *schema.Column) *ColumnChanges {
	changes := &ColumnChanges{}
	hasChanges := false

	if !columnTypesEqual(expected.Type, actual.Type) {
		changes.TypeChange = &TypeChange{From: actual.Type, To: expected.Type}
		hasChanges = true
	}

	if expected.Nullable != actual.Nullable {
		changes.NullableChange = &BoolChange{From: actual.Nullable, To: expected.Nullable}
		hasChanges = true
	}

	if !defaultsEqual(expected.Default, actual.Default) {
		changes.DefaultChange = &DefaultChange{From: actual.Default, To: expected.Default}
		hasChanges = true
	}

	if !hasChanges {
		return nil
	}
	return changes
}

// indexesEqual compares two indexes for equality.
func indexesEqual(a, b *schema.Index) bool {
	if a.Unique != b.Unique {
		return false
	}
	if !slices.Equal(a.Columns, b.Columns) {
		return false
	}
	// Only compare type if both are set (default is btree)
	aType := strings.ToLower(a.Type)
	bType := strings.ToLower(b.Type)
	if aType == "" {
		aType = "btree"
	}
	if bType == "" {
		bType = "btree"
	}
	if aType != bType {
		return false
	}
	return a.Where == b.Where
}

// foreignKeysEqual compares two foreign keys for equality.
func foreignKeysEqual(a, b *schema.ForeignKey) bool {
	if !slices.Equal(a.Columns, b.Columns) {
		return false
	}
	if a.ReferencedTable != b.ReferencedTable {
		return false
	}
	if !slices.Equal(a.ReferencedColumns, b.ReferencedColumns) {
		return false
	}
	if a.OnDelete != b.OnDelete || a.OnUpdate != b.OnUpdate {
		return false
	}
	return true
}

// enumsEqual compares two enums for equality.
func enumsEqual(a, b *schema.Enum) bool {
	return slices.Equal(a.Values, b.Values)
}
