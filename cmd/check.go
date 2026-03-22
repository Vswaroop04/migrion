package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vswaroop04/migratex/internal/config"
	"github.com/vswaroop04/migratex/internal/dag"
	"github.com/vswaroop04/migratex/internal/drift"
)

func init() {
	rootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate migrations and detect drift (for CI/CD)",
	Long: `Runs validation checks suitable for CI/CD pipelines:
  1. Validates migration graph integrity (no cycles, no missing parents)
  2. Verifies migration checksums (no tampering)
  3. Detects schema drift (database changed outside migrations)

Exits with code 1 if any issues are found.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(".")
		if err != nil {
			return err
		}

		hasErrors := false

		// 1. Validate migration graph
		fmt.Println("Checking migration graph...")
		store := dag.NewStore(cfg.MigrationsDir)
		graph, err := store.LoadGraph()
		if err != nil {
			fmt.Printf("  FAIL: %v\n", err)
			hasErrors = true
		} else {
			result := graph.Validate()
			if result.Valid {
				fmt.Printf("  OK: %d migration(s), graph is valid\n", len(graph.Nodes))
			} else {
				hasErrors = true
				for _, e := range result.Errors {
					fmt.Printf("  FAIL: %s\n", e)
				}
			}
		}

		// 2. Detect drift (requires DB connection)
		if cfg.Connection != "" || os.Getenv("DATABASE_URL") != "" {
			if err := cfg.Validate(); err == nil {
				fmt.Println("\nChecking for schema drift...")
				ormSchema, err := readORMSchema(cfg)
				if err != nil {
					fmt.Printf("  WARN: Could not read ORM schema: %v\n", err)
				} else {
					dbSchema, err := introspectDB(cfg)
					if err != nil {
						fmt.Printf("  WARN: Could not connect to database: %v\n", err)
					} else {
						driftResult := drift.Detect(ormSchema, dbSchema)
						if driftResult.HasDrift {
							hasErrors = true
							fmt.Println(driftResult.Summary)
						} else {
							fmt.Println("  OK: No drift detected")
						}
					}
				}
			}
		} else {
			fmt.Println("\nSkipping drift detection (no database connection configured)")
		}

		if hasErrors {
			fmt.Println("\nCheck FAILED")
			os.Exit(1)
		}

		fmt.Println("\nAll checks passed")
		return nil
	},
}
