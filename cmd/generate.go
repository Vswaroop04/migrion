package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/vswaroop04/migratex/internal/config"
	"github.com/vswaroop04/migratex/internal/dag"
	dbpkg "github.com/vswaroop04/migratex/internal/db"
	"github.com/vswaroop04/migratex/internal/db/mysql"
	"github.com/vswaroop04/migratex/internal/db/pg"
	"github.com/vswaroop04/migratex/internal/diff"
	"github.com/vswaroop04/migratex/internal/planner"
	"github.com/vswaroop04/migratex/internal/schema"
)

func init() {
	generateCmd.Flags().StringP("message", "m", "", "Migration description")
	rootCmd.AddCommand(generateCmd)
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new migration from schema changes",
	Long: `Reads your ORM schema, compares it against the database,
and generates a new migration with up/down SQL.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Load config
		cfg, err := config.Load(".")
		if err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return err
		}

		// 2. Read ORM schema via sidecar
		fmt.Println("Reading ORM schema...")
		ormSchema, err := readORMSchema(cfg)
		if err != nil {
			return fmt.Errorf("reading ORM schema: %w", err)
		}

		// 3. Introspect database
		fmt.Println("Introspecting database...")
		dbSchema, err := introspectDB(cfg)
		if err != nil {
			return fmt.Errorf("introspecting database: %w", err)
		}

		// 4. Diff
		fmt.Println("Computing diff...")
		ops := diff.DiffSchemas(ormSchema, dbSchema)

		if len(ops) == 0 {
			fmt.Println("No changes detected. Schema is up to date.")
			return nil
		}

		fmt.Printf("Found %d change(s):\n", len(ops))
		for _, op := range ops {
			fmt.Printf("  %s", op.Type)
			if op.TableName != "" {
				fmt.Printf(" %s", op.TableName)
			}
			if op.Column != nil {
				fmt.Printf(".%s", op.Column.Name)
			}
			fmt.Println()
		}

		// 5. Plan and render SQL
		dialect := getDialect(cfg)
		upSQL, downSQL := planner.RenderMigrationSQL(ops, dialect)

		// 6. Build migration node
		store := dag.NewStore(cfg.MigrationsDir)
		graph, err := store.LoadGraph()
		if err != nil {
			return fmt.Errorf("loading migration graph: %w", err)
		}

		parents := graph.Heads
		id := dag.ComputeID(parents, ops)

		description, _ := cmd.Flags().GetString("message")
		if description == "" {
			description = generateDescription(ops)
		}

		node := &dag.MigrationNode{
			ID:          id,
			Parents:     parents,
			Timestamp:   time.Now().UTC(),
			Description: description,
			Operations:  ops,
			UpSQL:       upSQL,
			DownSQL:     downSQL,
			Checksum:    dag.ComputeChecksum(upSQL),
		}

		// 7. Check for merge conflicts if multiple heads
		if len(graph.Heads) > 1 {
			fmt.Printf("\nMultiple heads detected (%d branches). Checking for conflicts...\n", len(graph.Heads))
			// For now, just warn. Full merge support in future.
		}

		// 8. Save to disk
		if err := store.SaveNode(node); err != nil {
			return fmt.Errorf("saving migration: %w", err)
		}

		// Update graph
		if len(parents) == 0 {
			// Root node
			graph.Nodes[id] = node
			graph.Heads = []string{id}
		} else {
			if err := graph.AddNode(node); err != nil {
				return fmt.Errorf("adding to graph: %w", err)
			}
		}

		if err := store.UpdateGraph(graph); err != nil {
			return fmt.Errorf("updating graph: %w", err)
		}

		fmt.Printf("\nGenerated migration: %s\n", id)
		fmt.Printf("  %s/%s/up.sql\n", cfg.MigrationsDir, id)
		fmt.Printf("  %s/%s/down.sql\n", cfg.MigrationsDir, id)
		fmt.Printf("  Description: %s\n", description)

		return nil
	},
}

// readORMSchema calls the TypeScript sidecar to extract the ORM schema.
func readORMSchema(cfg *config.Config) (*schema.Schema, error) {
	// Find the sidecar binary
	sidecarPath := findSidecar()

	// Run: npx tsx <sidecar>/bin/migratex-export.ts --orm <orm> --schema <path> --dialect <dialect>
	absSchema, err := filepath.Abs(cfg.SchemaPath)
	if err != nil {
		return nil, err
	}

	cmdArgs := []string{sidecarPath, "--orm", cfg.ORM, "--schema", absSchema, "--dialect", cfg.Dialect}

	// Use npx tsx to run the TypeScript sidecar
	command := exec.Command("npx", append([]string{"tsx"}, cmdArgs...)...)
	command.Stderr = os.Stderr

	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("sidecar failed: %w", err)
	}

	var s schema.Schema
	if err := json.Unmarshal(output, &s); err != nil {
		return nil, fmt.Errorf("parsing sidecar output: %w (output: %s)", err, string(output))
	}

	return &s, nil
}

// findSidecar locates the sidecar script.
func findSidecar() string {
	// Check relative to the migratex binary
	candidates := []string{
		"./sidecar/bin/migratex-export.ts",
		"./node_modules/@migratex/sidecar/dist/bin/migratex-export.js",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	// Default: assume it's in the sidecar directory
	return "./sidecar/bin/migratex-export.ts"
}

// introspectDB connects to the database and reads its schema.
func introspectDB(cfg *config.Config) (*schema.Schema, error) {
	var introspector dbpkg.Introspector

	switch cfg.Dialect {
	case "pg":
		introspector = &pg.Introspector{}
	case "mysql":
		introspector = &mysql.Introspector{}
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", cfg.Dialect)
	}

	if err := introspector.Connect(cfg.Connection); err != nil {
		return nil, err
	}
	defer introspector.Close()

	return introspector.Introspect()
}

func getDialect(cfg *config.Config) planner.SQLDialect {
	switch cfg.Dialect {
	case "pg":
		return pg.Dialect{}
	case "mysql":
		return mysql.Dialect{}
	default:
		return pg.Dialect{}
	}
}

func generateDescription(ops []diff.Operation) string {
	if len(ops) == 1 {
		op := ops[0]
		switch op.Type {
		case diff.OpCreateTable:
			return fmt.Sprintf("create table %s", op.TableName)
		case diff.OpDropTable:
			return fmt.Sprintf("drop table %s", op.TableName)
		case diff.OpAddColumn:
			return fmt.Sprintf("add column %s.%s", op.TableName, op.Column.Name)
		case diff.OpDropColumn:
			return fmt.Sprintf("drop column %s.%s", op.TableName, op.Column.Name)
		}
	}

	// Count operation types
	counts := make(map[diff.OpType]int)
	for _, op := range ops {
		counts[op.Type]++
	}

	var parts []string
	for opType, count := range counts {
		parts = append(parts, fmt.Sprintf("%d %s", count, opType))
	}

	return fmt.Sprintf("migration: %s", parts[0])
}
