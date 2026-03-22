// Package db defines the interface for database introspection and migration management.
package db

import "github.com/vswaroop04/migratex/internal/schema"

// AppliedMigration records that a migration was applied to the database.
type AppliedMigration struct {
	ID        string `json:"id"`
	AppliedAt string `json:"appliedAt"`
	Checksum  string `json:"checksum"`
}

// Introspector reads the live database schema and manages migration state.
// Both PG and MySQL implement this interface.
//
// Go interfaces are implicit: any struct that has these methods satisfies
// the interface automatically. No "implements" keyword needed.
type Introspector interface {
	// Connect establishes a database connection.
	Connect(connectionURL string) error

	// Close closes the database connection.
	Close() error

	// Introspect reads the current database schema.
	Introspect() (*schema.Schema, error)

	// EnsureHistoryTable creates the _migratex_history table if it doesn't exist.
	EnsureHistoryTable() error

	// GetAppliedMigrations returns all migrations that have been applied.
	GetAppliedMigrations() ([]AppliedMigration, error)

	// RecordMigration marks a migration as applied.
	RecordMigration(id, checksum string) error

	// Execute runs arbitrary SQL (for applying migrations).
	Execute(sql string) error

	// AcquireLock prevents concurrent migration runs.
	AcquireLock() error

	// ReleaseLock releases the migration lock.
	ReleaseLock() error
}
