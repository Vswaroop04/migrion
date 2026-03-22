package mysql

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/vswaroop04/migratex/internal/db"
	"github.com/vswaroop04/migratex/internal/schema"

	_ "github.com/go-sql-driver/mysql"
)

// Introspector reads MySQL schema via information_schema queries.
type Introspector struct {
	conn   *sql.DB
	dbName string
}

func (m *Introspector) Connect(connectionURL string) error {
	conn, err := sql.Open("mysql", connectionURL)
	if err != nil {
		return fmt.Errorf("connecting to mysql: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return fmt.Errorf("pinging mysql: %w", err)
	}
	m.conn = conn

	// Get current database name
	var dbName string
	if err := conn.QueryRow("SELECT DATABASE()").Scan(&dbName); err != nil {
		return fmt.Errorf("getting database name: %w", err)
	}
	m.dbName = dbName
	return nil
}

func (m *Introspector) Close() error {
	if m.conn != nil {
		return m.conn.Close()
	}
	return nil
}

func (m *Introspector) Introspect() (*schema.Schema, error) {
	s := &schema.Schema{Dialect: "mysql"}

	tables, err := m.readTables()
	if err != nil {
		return nil, err
	}

	for i := range tables {
		cols, err := m.readColumns(tables[i].Name)
		if err != nil {
			return nil, err
		}
		tables[i].Columns = cols

		indexes, err := m.readIndexes(tables[i].Name)
		if err != nil {
			return nil, err
		}
		tables[i].Indexes = indexes

		fks, err := m.readForeignKeys(tables[i].Name)
		if err != nil {
			return nil, err
		}
		tables[i].ForeignKeys = fks

		pk, err := m.readPrimaryKey(tables[i].Name)
		if err != nil {
			return nil, err
		}
		tables[i].PrimaryKey = pk
	}

	s.Tables = tables
	return s, nil
}

func (m *Introspector) readTables() ([]schema.Table, error) {
	rows, err := m.conn.Query(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = ?
		  AND table_type = 'BASE TABLE'
		  AND table_name NOT LIKE '_migratex_%'
		ORDER BY table_name
	`, m.dbName)
	if err != nil {
		return nil, fmt.Errorf("reading tables: %w", err)
	}
	defer rows.Close()

	var tables []schema.Table
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, schema.Table{Name: name})
	}
	return tables, rows.Err()
}

func (m *Introspector) readColumns(tableName string) ([]schema.Column, error) {
	rows, err := m.conn.Query(`
		SELECT column_name, data_type, is_nullable, column_default,
		       character_maximum_length, numeric_precision, numeric_scale,
		       column_type, extra
		FROM information_schema.columns
		WHERE table_schema = ? AND table_name = ?
		ORDER BY ordinal_position
	`, m.dbName, tableName)
	if err != nil {
		return nil, fmt.Errorf("reading columns for %s: %w", tableName, err)
	}
	defer rows.Close()

	var columns []schema.Column
	for rows.Next() {
		var (
			name, dataType, nullable string
			colDefault               sql.NullString
			charMaxLen               sql.NullInt64
			numPrecision, numScale   sql.NullInt64
			columnType               string
			extra                    string
		)

		if err := rows.Scan(&name, &dataType, &nullable, &colDefault,
			&charMaxLen, &numPrecision, &numScale, &columnType, &extra); err != nil {
			return nil, err
		}

		col := schema.Column{
			Name:     name,
			Type:     mysqlTypeToColumnType(dataType, columnType, charMaxLen, numPrecision, numScale),
			Nullable: nullable == "YES",
		}

		// auto_increment maps to serial
		if strings.Contains(extra, "auto_increment") {
			col.Type = schema.ColumnType{Kind: "serial"}
		}

		if colDefault.Valid {
			col.Default = mysqlParseDefault(colDefault.String)
		}

		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func (m *Introspector) readIndexes(tableName string) ([]schema.Index, error) {
	rows, err := m.conn.Query(`
		SELECT index_name, non_unique,
		       GROUP_CONCAT(column_name ORDER BY seq_in_index) AS columns,
		       index_type
		FROM information_schema.statistics
		WHERE table_schema = ? AND table_name = ?
		  AND index_name != 'PRIMARY'
		GROUP BY index_name, non_unique, index_type
		ORDER BY index_name
	`, m.dbName, tableName)
	if err != nil {
		return nil, fmt.Errorf("reading indexes for %s: %w", tableName, err)
	}
	defer rows.Close()

	var indexes []schema.Index
	for rows.Next() {
		var (
			name      string
			nonUnique bool
			columns   string
			indexType string
		)

		if err := rows.Scan(&name, &nonUnique, &columns, &indexType); err != nil {
			return nil, err
		}

		indexes = append(indexes, schema.Index{
			Name:    name,
			Columns: strings.Split(columns, ","),
			Unique:  !nonUnique,
			Type:    strings.ToLower(indexType),
		})
	}
	return indexes, rows.Err()
}

func (m *Introspector) readForeignKeys(tableName string) ([]schema.ForeignKey, error) {
	rows, err := m.conn.Query(`
		SELECT
			kcu.constraint_name,
			GROUP_CONCAT(DISTINCT kcu.column_name ORDER BY kcu.ordinal_position) AS columns,
			kcu.referenced_table_name,
			GROUP_CONCAT(DISTINCT kcu.referenced_column_name ORDER BY kcu.ordinal_position) AS ref_columns,
			rc.delete_rule,
			rc.update_rule
		FROM information_schema.key_column_usage kcu
		JOIN information_schema.referential_constraints rc
			ON rc.constraint_name = kcu.constraint_name
			AND rc.constraint_schema = kcu.table_schema
		WHERE kcu.table_schema = ?
			AND kcu.table_name = ?
			AND kcu.referenced_table_name IS NOT NULL
		GROUP BY kcu.constraint_name, kcu.referenced_table_name, rc.delete_rule, rc.update_rule
		ORDER BY kcu.constraint_name
	`, m.dbName, tableName)
	if err != nil {
		return nil, fmt.Errorf("reading foreign keys for %s: %w", tableName, err)
	}
	defer rows.Close()

	var fks []schema.ForeignKey
	for rows.Next() {
		var (
			name, refTable        string
			columns, refColumns   string
			deleteRule, updateRule string
		)

		if err := rows.Scan(&name, &columns, &refTable, &refColumns, &deleteRule, &updateRule); err != nil {
			return nil, err
		}

		fks = append(fks, schema.ForeignKey{
			Name:              name,
			Columns:           strings.Split(columns, ","),
			ReferencedTable:   refTable,
			ReferencedColumns: strings.Split(refColumns, ","),
			OnDelete:          schema.ForeignKeyAction(deleteRule),
			OnUpdate:          schema.ForeignKeyAction(updateRule),
		})
	}
	return fks, rows.Err()
}

func (m *Introspector) readPrimaryKey(tableName string) (*schema.PrimaryKey, error) {
	rows, err := m.conn.Query(`
		SELECT column_name
		FROM information_schema.key_column_usage
		WHERE table_schema = ?
		  AND table_name = ?
		  AND constraint_name = 'PRIMARY'
		ORDER BY ordinal_position
	`, m.dbName, tableName)
	if err != nil {
		return nil, fmt.Errorf("reading primary key for %s: %w", tableName, err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err != nil {
			return nil, err
		}
		columns = append(columns, colName)
	}

	if len(columns) == 0 {
		return nil, rows.Err()
	}

	return &schema.PrimaryKey{Name: "PRIMARY", Columns: columns}, rows.Err()
}

func (m *Introspector) EnsureHistoryTable() error {
	_, err := m.conn.Exec(`
		CREATE TABLE IF NOT EXISTS _migratex_history (
			id VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			checksum VARCHAR(255) NOT NULL
		)
	`)
	return err
}

func (m *Introspector) GetAppliedMigrations() ([]db.AppliedMigration, error) {
	rows, err := m.conn.Query(`
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
		var am db.AppliedMigration
		if err := rows.Scan(&am.ID, &am.AppliedAt, &am.Checksum); err != nil {
			return nil, err
		}
		migrations = append(migrations, am)
	}
	return migrations, rows.Err()
}

func (m *Introspector) RecordMigration(id, checksum string) error {
	_, err := m.conn.Exec(`
		INSERT IGNORE INTO _migratex_history (id, checksum)
		VALUES (?, ?)
	`, id, checksum)
	return err
}

func (m *Introspector) Execute(sqlStr string) error {
	_, err := m.conn.Exec(sqlStr)
	return err
}

// AcquireLock uses MySQL GET_LOCK for migration safety.
func (m *Introspector) AcquireLock() error {
	var result int
	err := m.conn.QueryRow("SELECT GET_LOCK('migratex_migration', 30)").Scan(&result)
	if err != nil {
		return err
	}
	if result != 1 {
		return fmt.Errorf("could not acquire migration lock (timeout)")
	}
	return nil
}

func (m *Introspector) ReleaseLock() error {
	_, err := m.conn.Exec("SELECT RELEASE_LOCK('migratex_migration')")
	return err
}

// --- helpers ---

func mysqlTypeToColumnType(dataType, columnType string, charMaxLen, numPrecision, numScale sql.NullInt64) schema.ColumnType {
	switch strings.ToLower(dataType) {
	case "int":
		return schema.ColumnType{Kind: "int"}
	case "smallint":
		return schema.ColumnType{Kind: "int", Size: "smallint"}
	case "tinyint":
		// tinyint(1) is boolean in MySQL
		if strings.Contains(columnType, "(1)") {
			return schema.ColumnType{Kind: "boolean"}
		}
		return schema.ColumnType{Kind: "int", Size: "smallint"}
	case "bigint":
		return schema.ColumnType{Kind: "int", Size: "bigint"}
	case "mediumint":
		return schema.ColumnType{Kind: "int"}
	case "float":
		return schema.ColumnType{Kind: "float"}
	case "double":
		return schema.ColumnType{Kind: "float", Size: "double"}
	case "decimal":
		ct := schema.ColumnType{Kind: "decimal"}
		if numPrecision.Valid {
			ct.Precision = int(numPrecision.Int64)
		}
		if numScale.Valid {
			ct.Scale = int(numScale.Int64)
		}
		return ct
	case "varchar":
		ct := schema.ColumnType{Kind: "text"}
		if charMaxLen.Valid {
			ct.Length = int(charMaxLen.Int64)
		}
		return ct
	case "char":
		ct := schema.ColumnType{Kind: "text"}
		if charMaxLen.Valid {
			ct.Length = int(charMaxLen.Int64)
		}
		return ct
	case "text", "mediumtext", "longtext", "tinytext":
		return schema.ColumnType{Kind: "text"}
	case "date":
		return schema.ColumnType{Kind: "date"}
	case "datetime", "timestamp":
		return schema.ColumnType{Kind: "timestamp"}
	case "time":
		return schema.ColumnType{Kind: "time"}
	case "json":
		return schema.ColumnType{Kind: "json"}
	case "blob", "mediumblob", "longblob", "tinyblob", "binary", "varbinary":
		return schema.ColumnType{Kind: "bytea"}
	case "enum":
		return schema.ColumnType{Kind: "enum", Raw: columnType}
	default:
		return schema.ColumnType{Kind: "custom", Raw: dataType}
	}
}

func mysqlParseDefault(defaultStr string) *schema.ColumnDefault {
	lower := strings.ToLower(defaultStr)

	if strings.Contains(lower, "(") || lower == "current_timestamp" {
		return &schema.ColumnDefault{Kind: "expression", Value: defaultStr}
	}

	return &schema.ColumnDefault{Kind: "value", Value: defaultStr}
}
