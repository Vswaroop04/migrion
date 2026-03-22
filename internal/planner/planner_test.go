package planner

import (
	"strings"
	"testing"

	"github.com/vswaroop04/migratex/internal/db/pg"
	"github.com/vswaroop04/migratex/internal/diff"
	"github.com/vswaroop04/migratex/internal/schema"
)

func TestPlanMigration_OrdersCorrectly(t *testing.T) {
	// Give ops in wrong order — planner should fix it
	ops := []diff.Operation{
		{Type: diff.OpAddForeignKey, TableName: "orders", ForeignKey: &schema.ForeignKey{Name: "fk_user"}},
		{Type: diff.OpCreateTable, TableName: "users", Table: &schema.Table{Name: "users"}},
		{Type: diff.OpCreateEnum, Enum: &schema.Enum{Name: "status", Values: []string{"active"}}},
		{Type: diff.OpAddColumn, TableName: "users", Column: &schema.Column{Name: "age"}},
		{Type: diff.OpDropColumn, TableName: "old", Column: &schema.Column{Name: "x"}},
	}

	planned := PlanMigration(ops)

	// Expected order: create_enum, create_table, add_column, add_fk, drop_column
	expectedOrder := []diff.OpType{
		diff.OpCreateEnum,
		diff.OpCreateTable,
		diff.OpAddColumn,
		diff.OpAddForeignKey,
		diff.OpDropColumn,
	}

	for i, expected := range expectedOrder {
		if planned[i].Type != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, planned[i].Type)
		}
	}
}

func TestReverseOperation_RoundTrip(t *testing.T) {
	// reverse(reverse(op)) should give back the original
	ops := []diff.Operation{
		{Type: diff.OpCreateTable, TableName: "users", Table: &schema.Table{Name: "users"}},
		{Type: diff.OpAddColumn, TableName: "users", Column: &schema.Column{Name: "age", Type: schema.ColumnType{Kind: "int"}}},
		{Type: diff.OpCreateIndex, TableName: "users", Index: &schema.Index{Name: "idx_age", Columns: []string{"age"}}},
	}

	for _, op := range ops {
		reversed := ReverseOperation(op)
		doubleReversed := ReverseOperation(reversed)

		if doubleReversed.Type != op.Type {
			t.Errorf("round-trip failed for %s: got %s", op.Type, doubleReversed.Type)
		}
	}
}

func TestRenderMigrationSQL_CreateTable(t *testing.T) {
	ops := []diff.Operation{
		{
			Type:      diff.OpCreateTable,
			TableName: "users",
			Table: &schema.Table{
				Name: "users",
				Columns: []schema.Column{
					{Name: "id", Type: schema.ColumnType{Kind: "serial"}, Nullable: false},
					{Name: "name", Type: schema.ColumnType{Kind: "text"}, Nullable: false},
					{Name: "email", Type: schema.ColumnType{Kind: "text", Length: 255}, Nullable: true},
				},
				PrimaryKey: &schema.PrimaryKey{Columns: []string{"id"}},
			},
		},
	}

	up, down := RenderMigrationSQL(ops, pg.Dialect{})

	if !strings.Contains(up, "CREATE TABLE") {
		t.Error("up SQL should contain CREATE TABLE")
	}
	if !strings.Contains(up, "SERIAL") {
		t.Error("up SQL should contain SERIAL type")
	}
	if !strings.Contains(up, "VARCHAR(255)") {
		t.Error("up SQL should contain VARCHAR(255)")
	}
	if !strings.Contains(up, "NOT NULL") {
		t.Error("up SQL should contain NOT NULL")
	}
	if !strings.Contains(down, "DROP TABLE") {
		t.Error("down SQL should contain DROP TABLE")
	}

	t.Logf("UP SQL:\n%s", up)
	t.Logf("DOWN SQL:\n%s", down)
}

func TestRenderMigrationSQL_AddColumn(t *testing.T) {
	ops := []diff.Operation{
		{
			Type:      diff.OpAddColumn,
			TableName: "users",
			Column: &schema.Column{
				Name:     "age",
				Type:     schema.ColumnType{Kind: "int"},
				Nullable: true,
			},
		},
	}

	up, down := RenderMigrationSQL(ops, pg.Dialect{})

	if !strings.Contains(up, `ALTER TABLE "users" ADD COLUMN "age" INTEGER`) {
		t.Errorf("unexpected up SQL: %s", up)
	}
	if !strings.Contains(down, `ALTER TABLE "users" DROP COLUMN "age"`) {
		t.Errorf("unexpected down SQL: %s", down)
	}
}

func TestRenderMigrationSQL_Enum(t *testing.T) {
	ops := []diff.Operation{
		{
			Type: diff.OpCreateEnum,
			Enum: &schema.Enum{Name: "status", Values: []string{"active", "inactive"}},
		},
	}

	up, _ := RenderMigrationSQL(ops, pg.Dialect{})

	if !strings.Contains(up, "CREATE TYPE") {
		t.Error("up SQL should contain CREATE TYPE for PG enum")
	}
	if !strings.Contains(up, "'active'") {
		t.Error("up SQL should contain enum values")
	}
}
