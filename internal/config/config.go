package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all migratex settings.
type Config struct {
	ORM           string `json:"orm" yaml:"orm"`
	Dialect       string `json:"dialect" yaml:"dialect"`
	Connection    string `json:"connection,omitempty" yaml:"connection,omitempty"` // optional override
	SchemaPath    string `json:"schemaPath" yaml:"schemaPath"`
	MigrationsDir string `json:"migrationsDir" yaml:"migrationsDir"`
}

func DefaultConfig() *Config {
	return &Config{
		ORM:           "drizzle",
		Dialect:       "pg",
		MigrationsDir: "./migrations",
	}
}

func Load(dir string) (*Config, error) {
	yamlPath := filepath.Join(dir, "migratex.config.yaml")
	if data, err := os.ReadFile(yamlPath); err == nil {
		cfg := DefaultConfig()
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", yamlPath, err)
		}
		return cfg, nil
	}

	jsonPath := filepath.Join(dir, "migratex.config.json")
	if data, err := os.ReadFile(jsonPath); err == nil {
		cfg := DefaultConfig()
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", jsonPath, err)
		}
		return cfg, nil
	}

	return nil, fmt.Errorf("no migratex config found (tried %s and %s)", yamlPath, jsonPath)
}

func Save(dir string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	path := filepath.Join(dir, "migratex.config.yaml")
	return os.WriteFile(path, data, 0o644)
}

// Validate checks required fields and resolves the database connection.
// If no connection is set, it reads from the ORM's config.
func (c *Config) Validate() error {
	if c.ORM == "" {
		return fmt.Errorf("orm is required (drizzle, prisma, typeorm)")
	}
	if c.Dialect == "" {
		return fmt.Errorf("dialect is required (pg, mysql)")
	}
	if c.SchemaPath == "" {
		return fmt.Errorf("schemaPath is required")
	}

	// If connection is already set (explicit override), resolve env: refs
	if c.Connection != "" {
		return c.resolveConnection()
	}

	// Otherwise, read connection from the ORM's config
	conn := resolveFromORM(c.ORM, c.Dialect)
	if conn != "" {
		c.Connection = conn
		return nil
	}

	// Last resort: DATABASE_URL from env or .env
	if url := getEnvVar("DATABASE_URL"); url != "" {
		c.Connection = url
		return nil
	}

	return fmt.Errorf("could not find database connection — migratex reads from your ORM config (%s), DATABASE_URL, or .env files", c.ORM)
}

func (c *Config) resolveConnection() error {
	if strings.HasPrefix(c.Connection, "env:") {
		envKey := strings.TrimPrefix(c.Connection, "env:")
		val := getEnvVar(envKey)
		if val == "" {
			return fmt.Errorf("%s is not set", envKey)
		}
		c.Connection = val
	}
	return nil
}

// resolveFromORM reads the database connection from the ORM's own config files.
func resolveFromORM(orm, dialect string) string {
	switch orm {
	case "drizzle":
		return resolveFromDrizzle(dialect)
	case "prisma":
		return resolveFromPrisma()
	case "typeorm":
		return resolveFromTypeORM()
	}
	return ""
}

func resolveFromDrizzle(dialect string) string {
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
		return ""
	}

	// Check if it uses a url/connectionString with an env var
	urlEnvRe := regexp.MustCompile(`(?:url|connectionString):\s*process\.env\.(\w+)`)
	if m := urlEnvRe.FindStringSubmatch(content); len(m) > 1 {
		if val := getEnvVar(m[1]); val != "" {
			return val
		}
	}

	// Check if it uses individual PG env vars (PGHOST, PGPORT, etc.)
	if strings.Contains(content, "process.env.PGHOST") || strings.Contains(content, "process.env.PG") {
		if url := buildPGURL(); url != "" {
			return url
		}
	}

	// Check if it uses individual MYSQL env vars
	if strings.Contains(content, "process.env.MYSQL_HOST") || strings.Contains(content, "process.env.DB_HOST") {
		if url := buildMySQLURL(); url != "" {
			return url
		}
	}

	return ""
}

func resolveFromPrisma() string {
	candidates := []string{"prisma/schema.prisma", "schema.prisma"}
	for _, c := range candidates {
		data, err := os.ReadFile(c)
		if err != nil {
			continue
		}
		// Prisma uses: url = env("DATABASE_URL")
		envRe := regexp.MustCompile(`url\s*=\s*env\("(\w+)"\)`)
		if m := envRe.FindStringSubmatch(string(data)); len(m) > 1 {
			if val := getEnvVar(m[1]); val != "" {
				return val
			}
		}
	}
	return ""
}

func resolveFromTypeORM() string {
	// ormconfig.json
	if data, err := os.ReadFile("ormconfig.json"); err == nil {
		var cfg struct {
			URL      string `json:"url"`
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
			Database string `json:"database"`
			Type     string `json:"type"`
		}
		if err := json.Unmarshal(data, &cfg); err == nil {
			if cfg.URL != "" {
				return cfg.URL
			}
			if cfg.Host != "" && cfg.Database != "" {
				port := cfg.Port
				if port == 0 {
					port = 5432
				}
				userInfo := ""
				if cfg.Username != "" && cfg.Password != "" {
					userInfo = cfg.Username + ":" + cfg.Password + "@"
				}
				proto := "postgresql"
				if cfg.Type == "mysql" {
					proto = "mysql"
				}
				return fmt.Sprintf("%s://%s%s:%d/%s", proto, userInfo, cfg.Host, port, cfg.Database)
			}
		}
	}

	// data-source.ts — check for DATABASE_URL
	for _, f := range []string{"data-source.ts", "src/data-source.ts"} {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "process.env.DATABASE_URL") {
			if val := getEnvVar("DATABASE_URL"); val != "" {
				return val
			}
		}
	}
	return ""
}

// buildPGURL constructs a postgres URL from PGHOST/PGPORT/PGUSER/PGPASSWORD/PGDATABASE.
func buildPGURL() string {
	host := getEnvVar("PGHOST")
	database := getEnvVar("PGDATABASE")
	if host == "" || database == "" {
		return ""
	}

	port := getEnvVar("PGPORT")
	if port == "" {
		port = "5432"
	}
	user := getEnvVar("PGUSER")
	password := getEnvVar("PGPASSWORD")
	ssl := getEnvVar("PGSSL")

	userInfo := ""
	if user != "" && password != "" {
		userInfo = user + ":" + password + "@"
	} else if user != "" {
		userInfo = user + "@"
	}

	url := fmt.Sprintf("postgresql://%s%s:%s/%s", userInfo, host, port, database)
	if ssl != "" && ssl != "false" {
		url += "?sslmode=require"
	}
	return url
}

// buildMySQLURL constructs a mysql DSN from MYSQL_HOST/DB_HOST env vars.
func buildMySQLURL() string {
	host := getEnvVar("MYSQL_HOST")
	if host == "" {
		host = getEnvVar("DB_HOST")
	}
	database := getEnvVar("MYSQL_DATABASE")
	if database == "" {
		database = getEnvVar("DB_NAME")
	}
	if host == "" || database == "" {
		return ""
	}

	port := getEnvVar("MYSQL_PORT")
	if port == "" {
		port = getEnvVar("DB_PORT")
	}
	if port == "" {
		port = "3306"
	}
	user := getEnvVar("MYSQL_USER")
	if user == "" {
		user = getEnvVar("DB_USER")
	}
	password := getEnvVar("MYSQL_PASSWORD")
	if password == "" {
		password = getEnvVar("DB_PASSWORD")
	}

	// go-sql-driver/mysql uses DSN format: user:password@tcp(host:port)/dbname
	if user != "" && password != "" {
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, database)
	}
	if user != "" {
		return fmt.Sprintf("%s@tcp(%s:%s)/%s", user, host, port, database)
	}
	return fmt.Sprintf("tcp(%s:%s)/%s", host, port, database)
}

// getEnvVar checks os env first, then .env files.
func getEnvVar(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	for _, f := range []string{".env", ".env.local", ".env.development"} {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
				val := strings.TrimSpace(parts[1])
				return strings.Trim(val, `"'`)
			}
		}
	}
	return ""
}
