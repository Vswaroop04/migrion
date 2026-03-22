package diff

// Go testing:
// - Test files end in _test.go (Go knows not to compile them into the binary)
// - Test functions start with Test (capital T)
// - They take *testing.T as a parameter
// - t.Errorf = test fails but continues; t.Fatalf = test fails and stops

import (
	"testing"

	"github.com/vswaroop04/migratex/internal/schema"
)

func TestDiffSchemas_EmptySchemas(t *testing.T) {
	expected := &schema.Schema{Dialect: "pg"}
	actual := &schema.Schema{Dialect: "pg"}

	ops := DiffSchemas(expected, actual)
	if len(ops) != 0 {
		t.Errorf("expected 0 operations for identical empty schemas, got %d", len(ops))
	}
}

func TestDiffSchemas_CreateTable(t *testing.T) {
	expected := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: schema.ColumnType{Kind: "serial"}, Nullable: false},
					{Name: "name", Type: schema.ColumnType{Kind: "text"}, Nullable: false},
				},
			},
		},
	}
	actual := &schema.Schema{Dialect: "pg"}

	ops := DiffSchemas(expected, actual)

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpCreateTable {
		t.Errorf("expected create_table, got %s", ops[0].Type)
	}
	if ops[0].TableName != "users" {
		t.Errorf("expected table name 'users', got %s", ops[0].TableName)
	}
}

func TestDiffSchemas_DropTable(t *testing.T) {
	expected := &schema.Schema{Dialect: "pg"}
	actual := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "old_table",
				Columns: []schema.Column{
					{Name: "id", Type: schema.ColumnType{Kind: "serial"}},
				},
			},
		},
	}

	ops := DiffSchemas(expected, actual)

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpDropTable {
		t.Errorf("expected drop_table, got %s", ops[0].Type)
	}
}

func TestDiffSchemas_AddColumn(t *testing.T) {
	expected := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: schema.ColumnType{Kind: "serial"}},
					{Name: "name", Type: schema.ColumnType{Kind: "text"}},
					{Name: "age", Type: schema.ColumnType{Kind: "int"}},
				},
			},
		},
	}
	actual := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: schema.ColumnType{Kind: "serial"}},
					{Name: "name", Type: schema.ColumnType{Kind: "text"}},
				},
			},
		},
	}

	ops := DiffSchemas(expected, actual)

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpAddColumn {
		t.Errorf("expected add_column, got %s", ops[0].Type)
	}
	if ops[0].Column.Name != "age" {
		t.Errorf("expected column 'age', got %s", ops[0].Column.Name)
	}
}

func TestDiffSchemas_DropColumn(t *testing.T) {
	expected := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: schema.ColumnType{Kind: "serial"}},
				},
			},
		},
	}
	actual := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: schema.ColumnType{Kind: "serial"}},
					{Name: "phone", Type: schema.ColumnType{Kind: "text"}},
				},
			},
		},
	}

	ops := DiffSchemas(expected, actual)

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpDropColumn {
		t.Errorf("expected drop_column, got %s", ops[0].Type)
	}
	if ops[0].Column.Name != "phone" {
		t.Errorf("expected column 'phone', got %s", ops[0].Column.Name)
	}
}

func TestDiffSchemas_AlterColumn_TypeChange(t *testing.T) {
	expected := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "age", Type: schema.ColumnType{Kind: "bigint"}},
				},
			},
		},
	}
	actual := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "age", Type: schema.ColumnType{Kind: "int"}},
				},
			},
		},
	}

	ops := DiffSchemas(expected, actual)

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpAlterColumn {
		t.Errorf("expected alter_column, got %s", ops[0].Type)
	}
	if ops[0].ColumnChanges.TypeChange == nil {
		t.Fatal("expected type change")
	}
	if ops[0].ColumnChanges.TypeChange.To.Kind != "bigint" {
		t.Errorf("expected new type 'bigint', got %s", ops[0].ColumnChanges.TypeChange.To.Kind)
	}
}

func TestDiffSchemas_AlterColumn_NullableChange(t *testing.T) {
	expected := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "name", Type: schema.ColumnType{Kind: "text"}, Nullable: false},
				},
			},
		},
	}
	actual := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "name", Type: schema.ColumnType{Kind: "text"}, Nullable: true},
				},
			},
		},
	}

	ops := DiffSchemas(expected, actual)

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].ColumnChanges.NullableChange == nil {
		t.Fatal("expected nullable change")
	}
	if ops[0].ColumnChanges.NullableChange.To != false {
		t.Error("expected nullable to become false")
	}
}

func TestDiffSchemas_TypeNormalization(t *testing.T) {
	// int4 and int should be treated as identical — no diff expected
	expected := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "age", Type: schema.ColumnType{Kind: "int"}},
				},
			},
		},
	}
	actual := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "age", Type: schema.ColumnType{Kind: "int4"}},
				},
			},
		},
	}

	ops := DiffSchemas(expected, actual)

	if len(ops) != 0 {
		t.Errorf("expected 0 ops (int4 == int), got %d: %+v", len(ops), ops)
	}
}

func TestDiffSchemas_CreateIndex(t *testing.T) {
	expected := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name:    "users",
				Columns: []schema.Column{{Name: "email", Type: schema.ColumnType{Kind: "text"}}},
				Indexes: []schema.Index{
					{Name: "idx_users_email", Columns: []string{"email"}, Unique: true},
				},
			},
		},
	}
	actual := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name:    "users",
				Columns: []schema.Column{{Name: "email", Type: schema.ColumnType{Kind: "text"}}},
			},
		},
	}

	ops := DiffSchemas(expected, actual)

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpCreateIndex {
		t.Errorf("expected create_index, got %s", ops[0].Type)
	}
}

func TestDiffSchemas_Enums(t *testing.T) {
	expected := &schema.Schema{
		Dialect: "pg",
		Enums: []schema.Enum{
			{Name: "status", Values: []string{"active", "inactive", "banned"}},
		},
	}
	actual := &schema.Schema{
		Dialect: "pg",
		Enums: []schema.Enum{
			{Name: "status", Values: []string{"active", "inactive"}},
		},
	}

	ops := DiffSchemas(expected, actual)

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpAlterEnum {
		t.Errorf("expected alter_enum, got %s", ops[0].Type)
	}
	if len(ops[0].AddValues) != 1 || ops[0].AddValues[0] != "banned" {
		t.Errorf("expected to add 'banned', got %v", ops[0].AddValues)
	}
}

func TestDiffSchemas_ForeignKey(t *testing.T) {
	expected := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name:    "orders",
				Columns: []schema.Column{{Name: "user_id", Type: schema.ColumnType{Kind: "int"}}},
				ForeignKeys: []schema.ForeignKey{
					{
						Name:              "fk_orders_user",
						Columns:           []string{"user_id"},
						ReferencedTable:   "users",
						ReferencedColumns: []string{"id"},
						OnDelete:          schema.FKCascade,
					},
				},
			},
		},
	}
	actual := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name:    "orders",
				Columns: []schema.Column{{Name: "user_id", Type: schema.ColumnType{Kind: "int"}}},
			},
		},
	}

	ops := DiffSchemas(expected, actual)

	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpAddForeignKey {
		t.Errorf("expected add_foreign_key, got %s", ops[0].Type)
	}
}

// Table-driven test: multiple scenarios in one test function.
// This is THE Go testing pattern. Define cases as a slice, loop through them.
func TestDiffSchemas_ComplexScenario(t *testing.T) {
	// Scenario: add a table, drop a column, add an index — all at once
	expected := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: schema.ColumnType{Kind: "serial"}},
					{Name: "email", Type: schema.ColumnType{Kind: "text"}},
				},
				Indexes: []schema.Index{
					{Name: "idx_users_email", Columns: []string{"email"}, Unique: true},
				},
			},
			{
				Name: "posts",
				Columns: []schema.Column{
					{Name: "id", Type: schema.ColumnType{Kind: "serial"}},
					{Name: "title", Type: schema.ColumnType{Kind: "text"}},
				},
			},
		},
	}
	actual := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: schema.ColumnType{Kind: "serial"}},
					{Name: "email", Type: schema.ColumnType{Kind: "text"}},
					{Name: "phone", Type: schema.ColumnType{Kind: "text"}},
				},
			},
		},
	}

	ops := DiffSchemas(expected, actual)

	// We expect: create_table(posts), drop_column(phone), create_index(idx_users_email)
	opTypes := make(map[OpType]int)
	for _, op := range ops {
		opTypes[op.Type]++
	}

	if opTypes[OpCreateTable] != 1 {
		t.Errorf("expected 1 create_table, got %d", opTypes[OpCreateTable])
	}
	if opTypes[OpDropColumn] != 1 {
		t.Errorf("expected 1 drop_column, got %d", opTypes[OpDropColumn])
	}
	if opTypes[OpCreateIndex] != 1 {
		t.Errorf("expected 1 create_index, got %d", opTypes[OpCreateIndex])
	}
}

func TestDiffSchemas_IdenticalSchemas(t *testing.T) {
	s := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: schema.ColumnType{Kind: "serial"}},
					{Name: "name", Type: schema.ColumnType{Kind: "text"}, Nullable: true},
				},
				Indexes: []schema.Index{
					{Name: "idx_name", Columns: []string{"name"}, Unique: false},
				},
			},
		},
		Enums: []schema.Enum{
			{Name: "role", Values: []string{"admin", "user"}},
		},
	}

	// Deep copy by using the same struct values
	actual := &schema.Schema{
		Dialect: "pg",
		Tables: []schema.Table{
			{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: schema.ColumnType{Kind: "serial"}},
					{Name: "name", Type: schema.ColumnType{Kind: "text"}, Nullable: true},
				},
				Indexes: []schema.Index{
					{Name: "idx_name", Columns: []string{"name"}, Unique: false},
				},
			},
		},
		Enums: []schema.Enum{
			{Name: "role", Values: []string{"admin", "user"}},
		},
	}

	ops := DiffSchemas(s, actual)
	if len(ops) != 0 {
		t.Errorf("expected 0 ops for identical schemas, got %d: %+v", len(ops), ops)
	}
}
