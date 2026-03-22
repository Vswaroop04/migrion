// Package cmd contains all CLI commands.
// In Go, package name = folder name. Everything in this folder is part of "package cmd".
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd is the base command. When you run `migratex` with no subcommand, this runs.
// cobra.Command is a struct — Go's version of a class/interface. No "new" keyword needed.
var rootCmd = &cobra.Command{
	Use:   "migratex",
	Short: "Schema diff and migration engine for ORMs",
	Long: `Migratex is a schema diff and migration engine that works on top of ORMs
like Drizzle, Prisma, and TypeORM. It uses a DAG-based migration graph
(like git) instead of linear migrations, enabling safe branch merging
and conflict detection.`,
}

// Execute is called from main.go. This is the entry point for the CLI.
// In Go, exported functions start with an uppercase letter (like public in Java/TS).
// Lowercase = unexported (private to the package).
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
