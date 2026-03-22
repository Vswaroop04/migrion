package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vswaroop04/migratex/internal/config"
)

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

		// Auto-detect ORM, let user override
		defaultORM := detectORM()
		if defaultORM == "" {
			defaultORM = "drizzle"
		}
		fmt.Printf("ORM (drizzle/prisma/typeorm) [%s]: ", defaultORM)
		input, _ := reader.ReadString('\n')
		orm := strings.TrimSpace(input)
		if orm == "" {
			orm = defaultORM
		}

		// Auto-detect dialect from ORM config, let user override
		ormCfg := readORMConfig(orm)
		defaultDialect := ormCfg.dialect
		if defaultDialect == "" {
			defaultDialect = "pg"
		}
		fmt.Printf("Database (pg/mysql) [%s]: ", defaultDialect)
		input, _ = reader.ReadString('\n')
		dialect := strings.TrimSpace(input)
		if dialect == "" {
			dialect = defaultDialect
		}

		// Auto-detect schema path, let user override
		defaultSchema := ormCfg.schemaPath
		if defaultSchema == "" {
			defaultSchema = detectSchemaPath(orm)
		}
		fmt.Printf("Schema path [%s]: ", defaultSchema)
		input, _ = reader.ReadString('\n')
		schemaPath := strings.TrimSpace(input)
		if schemaPath == "" {
			schemaPath = defaultSchema
		}

		// No connection field — migratex reads it from the ORM config at runtime
		cfg := &config.Config{
			ORM:           orm,
			Dialect:       dialect,
			SchemaPath:    schemaPath,
			MigrationsDir: "./migrations",
		}

		if err := config.Save(".", cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		if err := os.MkdirAll("./migrations", 0o755); err != nil {
			return fmt.Errorf("creating migrations dir: %w", err)
		}

		fmt.Println()
		fmt.Println("Created migratex.config.yaml")
		fmt.Println("Created migrations/")
		fmt.Println()
		fmt.Printf("migratex will read the database connection from your %s config at runtime.\n", orm)
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  Run: migratex generate")

		return nil
	},
}

type ormConfigResult struct {
	dialect    string
	schemaPath string
}

func readORMConfig(orm string) ormConfigResult {
	switch orm {
	case "drizzle":
		return readDrizzleConfig()
	case "prisma":
		return readPrismaConfig()
	case "typeorm":
		return readTypeORMConfig()
	}
	return ormConfigResult{}
}

func readDrizzleConfig() ormConfigResult {
	cfg := ormConfigResult{}
	candidates := []string{"drizzle.config.ts", "drizzle.config.js", "db/drizzle.config.ts"}

	var content string
	for _, c := range candidates {
		data, err := os.ReadFile(c)
		if err == nil {
			content = string(data)
			break
		}
	}
	if content == "" {
		return cfg
	}

	dialectRe := regexp.MustCompile(`dialect:\s*["'](\w+)["']`)
	if m := dialectRe.FindStringSubmatch(content); len(m) > 1 {
		switch m[1] {
		case "postgresql", "pg":
			cfg.dialect = "pg"
		case "mysql":
			cfg.dialect = "mysql"
		}
	}

	schemaRe := regexp.MustCompile(`schema:\s*["']([^"']+)["']`)
	if m := schemaRe.FindStringSubmatch(content); len(m) > 1 {
		cfg.schemaPath = m[1]
	}

	return cfg
}

func readPrismaConfig() ormConfigResult {
	cfg := ormConfigResult{}
	candidates := []string{"prisma/schema.prisma", "schema.prisma"}

	for _, c := range candidates {
		data, err := os.ReadFile(c)
		if err != nil {
			continue
		}
		cfg.schemaPath = c

		providerRe := regexp.MustCompile(`provider\s*=\s*"(\w+)"`)
		if m := providerRe.FindStringSubmatch(string(data)); len(m) > 1 {
			switch m[1] {
			case "postgresql":
				cfg.dialect = "pg"
			case "mysql":
				cfg.dialect = "mysql"
			}
		}
		break
	}

	return cfg
}

func readTypeORMConfig() ormConfigResult {
	cfg := ormConfigResult{}

	if data, err := os.ReadFile("ormconfig.json"); err == nil {
		var ormCfg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(data, &ormCfg); err == nil {
			switch ormCfg.Type {
			case "postgres":
				cfg.dialect = "pg"
			case "mysql":
				cfg.dialect = "mysql"
			}
		}
		return cfg
	}

	for _, f := range []string{"data-source.ts", "src/data-source.ts"} {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		typeRe := regexp.MustCompile(`type:\s*["'](\w+)["']`)
		if m := typeRe.FindStringSubmatch(string(data)); len(m) > 1 {
			switch m[1] {
			case "postgres":
				cfg.dialect = "pg"
			case "mysql":
				cfg.dialect = "mysql"
			}
		}
		break
	}

	return cfg
}

func detectORM() string {
	data, err := os.ReadFile("package.json")
	if err != nil {
		return ""
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}

	allDeps := make(map[string]bool)
	for k := range pkg.Dependencies {
		allDeps[k] = true
	}
	for k := range pkg.DevDependencies {
		allDeps[k] = true
	}

	if allDeps["drizzle-orm"] {
		return "drizzle"
	}
	if allDeps["@prisma/client"] || allDeps["prisma"] {
		return "prisma"
	}
	if allDeps["typeorm"] {
		return "typeorm"
	}

	return ""
}

func detectSchemaPath(orm string) string {
	switch orm {
	case "prisma":
		return "./prisma/schema.prisma"
	default:
		candidates := []string{
			"./src/db/schema.ts",
			"./src/schema.ts",
			"./db/schema.ts",
			"./db/schema/index.ts",
			"./src/db/schema/index.ts",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
		return "./src/schema.ts"
	}
}
