package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vswaroop04/migratex/internal/config"
	"github.com/vswaroop04/migratex/internal/dag"
	dbpkg "github.com/vswaroop04/migratex/internal/db"
	"github.com/vswaroop04/migratex/internal/db/pg"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration graph status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(".")
		if err != nil {
			return err
		}

		store := dag.NewStore(cfg.MigrationsDir)
		graph, err := store.LoadGraph()
		if err != nil {
			return fmt.Errorf("loading migration graph: %w", err)
		}

		if len(graph.Nodes) == 0 {
			fmt.Println("No migrations yet. Run: migratex generate")
			return nil
		}

		// Try to get applied migrations from DB
		appliedSet := make(map[string]bool)
		if cfg.Connection != "" {
			if err := cfg.Validate(); err == nil {
				var introspector dbpkg.Introspector
				switch cfg.Dialect {
				case "pg":
					introspector = &pg.Introspector{}
				}
				if introspector != nil {
					if err := introspector.Connect(cfg.Connection); err == nil {
						defer introspector.Close()
						introspector.EnsureHistoryTable()
						applied, err := introspector.GetAppliedMigrations()
						if err == nil {
							for _, m := range applied {
								appliedSet[m.ID] = true
							}
						}
					}
				}
			}
		}

		// Print migration graph
		fmt.Printf("Migration Graph (%d nodes)\n", len(graph.Nodes))
		fmt.Println(repeat("=", 40))

		sorted, err := graph.TopologicalSort()
		if err != nil {
			return err
		}

		for _, node := range sorted {
			status := "pending"
			marker := "o"
			if appliedSet[node.ID] {
				status = "applied"
				marker = "*"
			}

			isHead := false
			for _, h := range graph.Heads {
				if h == node.ID {
					isHead = true
					break
				}
			}

			headTag := ""
			if isHead {
				headTag = " <- HEAD"
			}

			parents := ""
			if len(node.Parents) > 0 {
				parents = fmt.Sprintf(" (parents: %v)", node.Parents)
			}

			fmt.Printf("  %s %s [%s] %s%s%s\n",
				marker, node.ID, status, node.Description, parents, headTag)
		}

		// Summary
		pending, _ := graph.Pending(appliedSet)
		fmt.Println()
		fmt.Printf("Heads: %v\n", graph.Heads)
		fmt.Printf("Applied: %d | Pending: %d | Total: %d\n",
			len(appliedSet), len(pending), len(graph.Nodes))

		return nil
	},
}

func repeat(s string, n int) string {
	result := ""
	for range n {
		result += s
	}
	return result
}
