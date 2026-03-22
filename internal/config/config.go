// Package config handles loading and validating the migratex configuration.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all migratex settings.
type Config struct {
	ORM           string `json:"orm" yaml:"orm"`                     // "drizzle", "prisma", "typeorm"
	Dialect       string `json:"dialect" yaml:"dialect"`             // "pg", "mysql"
	Connection    string `json:"connection" yaml:"connection"`       // DATABASE_URL
	SchemaPath    string `json:"schemaPath" yaml:"schemaPath"`       // path to ORM schema
	MigrationsDir string `json:"migrationsDir" yaml:"migrationsDir"` // default: "./migrations"
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		ORM:           "drizzle",
		Dialect:       "pg",
		MigrationsDir: "./migrations",
	}
}

// Load reads config from migratex.config.yaml (or .json) in the given directory.
func Load(dir string) (*Config, error) {
	// Try YAML first
	yamlPath := filepath.Join(dir, "migratex.config.yaml")
	if data, err := os.ReadFile(yamlPath); err == nil {
		cfg := DefaultConfig()
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", yamlPath, err)
		}
		return cfg, nil
	}

	// Try JSON
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

// Save writes config to migratex.config.yaml.
func Save(dir string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	path := filepath.Join(dir, "migratex.config.yaml")
	return os.WriteFile(path, data, 0o644)
}

// Validate checks that required fields are set.
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
	if c.Connection == "" {
		// Check env var fallback
		if url := os.Getenv("DATABASE_URL"); url != "" {
			c.Connection = url
		} else {
			return fmt.Errorf("connection is required (or set DATABASE_URL env var)")
		}
	}
	return nil
}
