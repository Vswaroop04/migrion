package pg

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/vswaroop04/migratex/internal/db"
	"github.com/vswaroop04/migratex/internal/schema"

	// pgx registers itself as a database/sql driver.
	// The underscore import means "import for side effects only" — a common Go pattern.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Introspector reads PostgreSQL schema via information_schema queries.
type Introspector struct {
	conn *sql.DB
}

// Connect opens a connection to PostgreSQL.
func (p *Introspector) Connect(connectionURL string) error {
	conn, err := sql.Open("pgx", connectionURL)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return fmt.Errorf("pinging postgres: %w", err)
	}
	p.conn = conn
	return nil
}

// Close closes the database connection.
func (p *Introspector) Close() error {
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

// Introspect reads the full schema from the database.
func (p *Introspector) Introspect() (*schema.Schema, error) {
	s := &schema.Schema{Dialect: "pg"}

	tables, err := p.readTables()
	if err != nil {
		return nil, err
	}

	for i := range tables {
		cols, err := p.readColumns(tables[i].Name)
		if err != nil {
			return nil, err
		}
		tables[i].Columns = cols

		indexes, err := p.readIndexes(tables[i].Name)
		if err != nil {
			return nil, err
		}
		tables[i].Indexes = indexes

		fks, err := p.readForeignKeys(tables[i].Name)
		if err != nil {
			return nil, err
		}
		tables[i].ForeignKeys = fks

		pk, err := p.readPrimaryKey(tables[i].Name)
		if err != nil {
			return nil, err
		}
		tables[i].PrimaryKey = pk
	}

	s.Tables = tables

	enums, err := p.readEnums()
	if err != nil {
		return nil, err
	}
	s.Enums = enums

	return s, nil
}

func (p *Introspector) readTables() ([]schema.Table, error) {
	// Query information_schema for user tables in the public schema
	rows, err := p.conn.Query(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_type = 'BASE TABLE'
		  AND table_name NOT LIKE '_migratex_%'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, fmt.Errorf("reading tables: %w", err)
	}
	defer rows.Close() // defer = runs when function returns (like "finally")

	var tables []schema.Table
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, schema.Table{Name: name, Schema: "public"})
	}
	return tables, rows.Err()
}

func (p *Introspector) readColumns(tableName string) ([]schema.Column, error) {
	rows, err := p.conn.Query(`
		SELECT column_name, data_type, is_nullable, column_default,
		       character_maximum_length, numeric_precision, numeric_scale,
		       udt_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("reading columns for %s: %w", tableName, err)
	}
	defer rows.Close()

	var columns []schema.Column
	for rows.Next() {
		var (
			name, dataType, nullable string
			colDefault               sql.NullString // sql.NullString handles NULL values
			charMaxLen               sql.NullInt64
			numPrecision, numScale   sql.NullInt64
			udtName                  string
		)

		if err := rows.Scan(&name, &dataType, &nullable, &colDefault,
			&charMaxLen, &numPrecision, &numScale, &udtName); err != nil {
			return nil, err
		}

		col := schema.Column{
			Name:     name,
			Type:     pgTypeToColumnType(dataType, udtName, charMaxLen, numPrecision, numScale),
			Nullable: nullable == "YES",
		}

		if colDefault.Valid {
			col.Default = pgParseDefault(colDefault.String)
		}

		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func (p *Introspector) readIndexes(tableName string) ([]schema.Index, error) {
	rows, err := p.conn.Query(`
		SELECT i.relname AS index_name,
		       ix.indisunique AS is_unique,
		       array_agg(a.attname ORDER BY array_position(ix.indkey, a.attnum)) AS columns,
		       am.amname AS index_type,
		       pg_get_expr(ix.indpred, ix.indrelid) AS predicate
		FROM pg_index ix
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		JOIN pg_am am ON am.oid = i.relam
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
		WHERE n.nspname = 'public'
		  AND t.relname = $1
		  AND NOT ix.indisprimary
		GROUP BY i.relname, ix.indisunique, am.amname, ix.indpred, ix.indrelid
		ORDER BY i.relname
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("reading indexes for %s: %w", tableName, err)
	}
	defer rows.Close()

	var indexes []schema.Index
	for rows.Next() {
		var (
			name      string
			unique    bool
			columns   string // comes as {col1,col2} from array_agg
			indexType string
			predicate sql.NullString
		)

		if err := rows.Scan(&name, &unique, &columns, &indexType, &predicate); err != nil {
			return nil, err
		}

		idx := schema.Index{
			Name:    name,
			Columns: parsePostgresArray(columns),
			Unique:  unique,
			Type:    indexType,
		}
		if predicate.Valid {
			idx.Where = predicate.String
		}

		indexes = append(indexes, idx)
	}
	return indexes, rows.Err()
}

func (p *Introspector) readForeignKeys(tableName string) ([]schema.ForeignKey, error) {
	rows, err := p.conn.Query(`
		SELECT
			tc.constraint_name,
			array_agg(DISTINCT kcu.column_name ORDER BY kcu.column_name) AS columns,
			ccu.table_name AS referenced_table,
			array_agg(DISTINCT ccu.column_name ORDER BY ccu.column_name) AS referenced_columns,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.table_schema
		JOIN information_schema.referential_constraints rc
			ON rc.constraint_name = tc.constraint_name
			AND rc.constraint_schema = tc.table_schema
		WHERE tc.table_schema = 'public'
			AND tc.table_name = $1
			AND tc.constraint_type = 'FOREIGN KEY'
		GROUP BY tc.constraint_name, ccu.table_name, rc.delete_rule, rc.update_rule
		ORDER BY tc.constraint_name
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("reading foreign keys for %s: %w", tableName, err)
	}
	defer rows.Close()

	var fks []schema.ForeignKey
	for rows.Next() {
		var (
			name, refTable           string
			columns, refColumns      string
			deleteRule, updateRule    string
		)

		if err := rows.Scan(&name, &columns, &refTable, &refColumns, &deleteRule, &updateRule); err != nil {
			return nil, err
		}

		fk := schema.ForeignKey{
			Name:              name,
			Columns:           parsePostgresArray(columns),
			ReferencedTable:   refTable,
			ReferencedColumns: parsePostgresArray(refColumns),
			OnDelete:          schema.ForeignKeyAction(deleteRule),
			OnUpdate:          schema.ForeignKeyAction(updateRule),
		}
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

func (p *Introspector) readPrimaryKey(tableName string) (*schema.PrimaryKey, error) {
	rows, err := p.conn.Query(`
		SELECT tc.constraint_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.table_schema = 'public'
			AND tc.table_name = $1
			AND tc.constraint_type = 'PRIMARY KEY'
		ORDER BY kcu.ordinal_position
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("reading primary key for %s: %w", tableName, err)
	}
	defer rows.Close()

	var pkName string
	var columns []string
	for rows.Next() {
		var constraintName, colName string
		if err := rows.Scan(&constraintName, &colName); err != nil {
			return nil, err
		}
		pkName = constraintName
		columns = append(columns, colName)
	}

	if len(columns) == 0 {
		return nil, rows.Err()
	}

	return &schema.PrimaryKey{Name: pkName, Columns: columns}, rows.Err()
}

func (p *Introspector) readEnums() ([]schema.Enum, error) {
	rows, err := p.conn.Query(`
		SELECT t.typname, array_agg(e.enumlabel ORDER BY e.enumsortorder)
		FROM pg_type t
		JOIN pg_enum e ON t.oid = e.enumtypid
		JOIN pg_namespace n ON t.typnamespace = n.oid
		WHERE n.nspname = 'public'
		GROUP BY t.typname
		ORDER BY t.typname
	`)
	if err != nil {
		return nil, fmt.Errorf("reading enums: %w", err)
	}
	defer rows.Close()

	var enums []schema.Enum
	for rows.Next() {
		var name, values string
		if err := rows.Scan(&name, &values); err != nil {
			return nil, err
		}
		enums = append(enums, schema.Enum{
			Name:   name,
			Values: parsePostgresArray(values),
		})
	}
	return enums, rows.Err()
}

// EnsureHistoryTable creates the migration tracking table if it doesn't exist.
func (p *Introspector) EnsureHistoryTable() error {
	_, err := p.conn.Exec(`
		CREATE TABLE IF NOT EXISTS _migratex_history (
			id TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			checksum TEXT NOT NULL
		)
	`)
	return err
}

// GetAppliedMigrations returns all previously applied migrations.
func (p *Introspector) GetAppliedMigrations() ([]db.AppliedMigration, error) {
	rows, err := p.conn.Query(`
		SELECT id, applied_at, checksum
		FROM _migratex_history
		ORDER BY applied_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var migrations []db.AppliedMigration
	for rows.Next() {
		var m db.AppliedMigration
		if err := rows.Scan(&m.ID, &m.AppliedAt, &m.Checksum); err != nil {
			return nil, err
		}
		migrations = append(migrations, m)
	}
	return migrations, rows.Err()
}

// RecordMigration marks a migration as applied in the history table.
func (p *Introspector) RecordMigration(id, checksum string) error {
	_, err := p.conn.Exec(`
		INSERT INTO _migratex_history (id, checksum)
		VALUES ($1, $2)
		ON CONFLICT (id) DO NOTHING
	`, id, checksum)
	return err
}

// Execute runs arbitrary SQL.
func (p *Introspector) Execute(sqlStr string) error {
	_, err := p.conn.Exec(sqlStr)
	return err
}

// AcquireLock uses PostgreSQL advisory locks to prevent concurrent migrations.
func (p *Introspector) AcquireLock() error {
	// pg_advisory_lock with a fixed lock ID (hash of "migratex")
	_, err := p.conn.Exec(`SELECT pg_advisory_lock(3456789012)`)
	return err
}

// ReleaseLock releases the advisory lock.
func (p *Introspector) ReleaseLock() error {
	_, err := p.conn.Exec(`SELECT pg_advisory_unlock(3456789012)`)
	return err
}

// --- helpers ---

// parsePostgresArray converts "{a,b,c}" to []string{"a", "b", "c"}
func parsePostgresArray(s string) []string {
	s = strings.Trim(s, "{}")
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// pgTypeToColumnType maps PostgreSQL data types to our internal ColumnType.
func pgTypeToColumnType(dataType, udtName string, charMaxLen, numPrecision, numScale sql.NullInt64) schema.ColumnType {
	switch strings.ToLower(dataType) {
	case "integer", "int", "int4":
		return schema.ColumnType{Kind: "int"}
	case "smallint", "int2":
		return schema.ColumnType{Kind: "int", Size: "smallint"}
	case "bigint", "int8":
		return schema.ColumnType{Kind: "int", Size: "bigint"}
	case "real", "float4":
		return schema.ColumnType{Kind: "float"}
	case "double precision", "float8":
		return schema.ColumnType{Kind: "float", Size: "double"}
	case "numeric", "decimal":
		ct := schema.ColumnType{Kind: "decimal"}
		if numPrecision.Valid {
			ct.Precision = int(numPrecision.Int64)
		}
		if numScale.Valid {
			ct.Scale = int(numScale.Int64)
		}
		return ct
	case "character varying", "varchar":
		ct := schema.ColumnType{Kind: "text"}
		if charMaxLen.Valid {
			ct.Length = int(charMaxLen.Int64)
		}
		return ct
	case "text":
		return schema.ColumnType{Kind: "text"}
	case "boolean", "bool":
		return schema.ColumnType{Kind: "boolean"}
	case "date":
		return schema.ColumnType{Kind: "date"}
	case "timestamp without time zone":
		return schema.ColumnType{Kind: "timestamp"}
	case "timestamp with time zone":
		return schema.ColumnType{Kind: "timestamp", WithTimezone: true}
	case "time without time zone":
		return schema.ColumnType{Kind: "time"}
	case "time with time zone":
		return schema.ColumnType{Kind: "time", WithTimezone: true}
	case "json":
		return schema.ColumnType{Kind: "json"}
	case "jsonb":
		return schema.ColumnType{Kind: "json", Binary: true}
	case "uuid":
		return schema.ColumnType{Kind: "uuid"}
	case "bytea":
		return schema.ColumnType{Kind: "bytea"}
	case "user-defined":
		return schema.ColumnType{Kind: "enum", EnumName: udtName}
	default:
		return schema.ColumnType{Kind: "custom", Raw: dataType}
	}
}

// pgParseDefault parses a PostgreSQL column default expression.
func pgParseDefault(defaultStr string) *schema.ColumnDefault {
	lower := strings.ToLower(defaultStr)

	// Sequences (serial columns)
	if strings.Contains(lower, "nextval(") {
		return &schema.ColumnDefault{Kind: "sequence", Value: defaultStr}
	}

	// Function calls and expressions
	if strings.Contains(lower, "(") || strings.HasPrefix(lower, "current_") || lower == "now()" {
		return &schema.ColumnDefault{Kind: "expression", Value: defaultStr}
	}

	// Strip type casts like '...'::text
	if idx := strings.Index(defaultStr, "::"); idx != -1 {
		val := strings.Trim(defaultStr[:idx], "'")
		return &schema.ColumnDefault{Kind: "value", Value: val}
	}

	return &schema.ColumnDefault{Kind: "value", Value: strings.Trim(defaultStr, "'")}
}
