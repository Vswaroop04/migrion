package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vswaroop04/migratex/internal/config"
)

// init() runs automatically when this package is imported.
// This is Go's way of doing module-level side effects.
func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize migratex in the current project",
	Long:  "Creates migratex.config.yaml and the migrations directory.",
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		fmt.Println("Initializing migratex...")
		fmt.Println()

		// Ask for ORM
		fmt.Print("ORM (drizzle/prisma/typeorm) [drizzle]: ")
		orm, _ := reader.ReadString('\n')
		orm = strings.TrimSpace(orm)
		if orm == "" {
			orm = "drizzle"
		}

		// Ask for dialect
		fmt.Print("Database (pg/mysql) [pg]: ")
		dialect, _ := reader.ReadString('\n')
		dialect = strings.TrimSpace(dialect)
		if dialect == "" {
			dialect = "pg"
		}

		// Ask for schema path
		defaultSchema := "./src/schema.ts"
		if orm == "prisma" {
			defaultSchema = "./prisma/schema.prisma"
		}
		fmt.Printf("Schema path [%s]: ", defaultSchema)
		schemaPath, _ := reader.ReadString('\n')
		schemaPath = strings.TrimSpace(schemaPath)
		if schemaPath == "" {
			schemaPath = defaultSchema
		}

		// Ask for connection
		fmt.Print("Database URL (or set DATABASE_URL env): ")
		connection, _ := reader.ReadString('\n')
		connection = strings.TrimSpace(connection)

		cfg := &config.Config{
			ORM:           orm,
			Dialect:       dialect,
			Connection:    connection,
			SchemaPath:    schemaPath,
			MigrationsDir: "./migrations",
		}

		// Save config
		if err := config.Save(".", cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		// Create migrations directory
		if err := os.MkdirAll("./migrations", 0o755); err != nil {
			return fmt.Errorf("creating migrations dir: %w", err)
		}

		fmt.Println()
		fmt.Println("Created migratex.config.yaml")
		fmt.Println("Created migrations/")
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  1. Set DATABASE_URL if you haven't already")
		fmt.Println("  2. Run: migratex generate")

		return nil
	},
}
