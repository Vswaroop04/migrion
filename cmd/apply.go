package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vswaroop04/migratex/internal/config"
	"github.com/vswaroop04/migratex/internal/dag"
	dbpkg "github.com/vswaroop04/migratex/internal/db"
	"github.com/vswaroop04/migratex/internal/db/mysql"
	"github.com/vswaroop04/migratex/internal/db/pg"
)

func init() {
	rootCmd.AddCommand(applyCmd)
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply pending migrations to the database",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(".")
		if err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return err
		}

		// Connect to DB
		var introspector dbpkg.Introspector
		switch cfg.Dialect {
		case "pg":
			introspector = &pg.Introspector{}
		case "mysql":
			introspector = &mysql.Introspector{}
		default:
			return fmt.Errorf("unsupported dialect: %s", cfg.Dialect)
		}

		if err := introspector.Connect(cfg.Connection); err != nil {
			return err
		}
		defer introspector.Close()

		// Ensure history table exists
		if err := introspector.EnsureHistoryTable(); err != nil {
			return fmt.Errorf("creating history table: %w", err)
		}

		// Acquire migration lock
		fmt.Println("Acquiring migration lock...")
		if err := introspector.AcquireLock(); err != nil {
			return fmt.Errorf("acquiring lock: %w", err)
		}
		defer introspector.ReleaseLock()

		// Load migration graph
		store := dag.NewStore(cfg.MigrationsDir)
		graph, err := store.LoadGraph()
		if err != nil {
			return fmt.Errorf("loading migration graph: %w", err)
		}

		// Get applied migrations
		applied, err := introspector.GetAppliedMigrations()
		if err != nil {
			return fmt.Errorf("reading migration history: %w", err)
		}

		appliedSet := make(map[string]bool)
		for _, m := range applied {
			appliedSet[m.ID] = true
		}

		// Find pending migrations in execution order
		pending, err := graph.Pending(appliedSet)
		if err != nil {
			return err
		}

		if len(pending) == 0 {
			fmt.Println("No pending migrations.")
			return nil
		}

		fmt.Printf("Applying %d migration(s)...\n\n", len(pending))

		// Apply each migration
		for _, node := range pending {
			// Verify checksum
			expectedChecksum := dag.ComputeChecksum(node.UpSQL)
			if node.Checksum != "" && node.Checksum != expectedChecksum {
				return fmt.Errorf("checksum mismatch for migration %s — file may have been tampered", node.ID)
			}

			fmt.Printf("  Applying %s: %s...", node.ID, node.Description)

			if err := introspector.Execute(node.UpSQL); err != nil {
				fmt.Println(" FAILED")
				return fmt.Errorf("applying migration %s: %w", node.ID, err)
			}

			if err := introspector.RecordMigration(node.ID, expectedChecksum); err != nil {
				return fmt.Errorf("recording migration %s: %w", node.ID, err)
			}

			fmt.Println(" done")
		}

		fmt.Printf("\nSuccessfully applied %d migration(s).\n", len(pending))
		return nil
	},
}
