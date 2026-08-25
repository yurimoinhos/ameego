package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "ameego.yaml"

// LegacyConfigFile is the deprecated config filename kept for backward compatibility.
const LegacyConfigFile = "goql.yaml"

// Config is the root ameego.yaml configuration.
type Config struct {
	Version    string          `yaml:"version"`
	Dialect    string          `yaml:"dialect"`
	Migrations string          `yaml:"migrations"`
	Output     string          `yaml:"output"`
	Package    string          `yaml:"package"`
	Module     string          `yaml:"module"`
	Generate   GenerateConfig  `yaml:"generate"`
	Migrate    MigrateConfig   `yaml:"migrate"`
	Queries    []QueryFileSpec `yaml:"queries"`
	Routines   []RoutineSpec   `yaml:"routines"`
	Relations  []RelationSpec  `yaml:"relations"`

	configDir string
}

// MigrateConfig controls the migration version tracker table.
type MigrateConfig struct {
	Schema string `yaml:"schema"` // PostgreSQL/MSSQL schema for ameego_migrations (default: public/dbo)
	Table  string `yaml:"table"`  // tracker table name (default: ameego_migrations)
}

// RoutineSpec binds a SQL routines file for codegen.
type RoutineSpec struct {
	File string `yaml:"file"`
}

// GenerateConfig toggles generated artifacts.
type GenerateConfig struct {
	Entities     *bool `yaml:"entities"`
	Repositories *bool `yaml:"repositories"`
	QueryHelpers *bool `yaml:"query_helpers"`
	Routines     *bool `yaml:"routines"`
}

// QueryFileSpec binds a SQL query file to an entity repository.
type QueryFileSpec struct {
	Entity string `yaml:"entity"`
	File   string `yaml:"file"`
}

// RelationSpec declares a joinable relationship between tables or views.
// Use this for views (no FKs) or extra joins beyond migration foreign keys.
type RelationSpec struct {
	From       string `yaml:"from"`        // table/view name or entity name (User, users)
	FromColumn string `yaml:"from_column"` // local join column
	To         string `yaml:"to"`          // referenced table/view or entity name
	ToColumn   string `yaml:"to_column"`   // referenced join column
	Join       string `yaml:"join"`        // inner | left | right | full (optional)
	Field      string `yaml:"field"`       // nested struct field name (optional)
}

// ResolvedQuery is an absolute query file binding.
type ResolvedQuery struct {
	Entity string
	File   string
}

// Load reads and validates an ameego.yaml file (or legacy goql.yaml).
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigFile
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(abs)
	if err != nil && path == DefaultConfigFile {
		legacy := filepath.Join(filepath.Dir(abs), LegacyConfigFile)
		raw, err = os.ReadFile(legacy)
		if err == nil {
			abs = legacy
		}
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.configDir = filepath.Dir(abs)
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Dialect == "" {
		c.Dialect = "postgres"
	}
	if c.Migrations == "" {
		return fmt.Errorf("config: migrations is required")
	}
	if c.Output == "" {
		c.Output = "generated"
	}
	if c.Package == "" {
		c.Package = "generated"
	}
	for i, q := range c.Queries {
		if strings.TrimSpace(q.Entity) == "" {
			return fmt.Errorf("config: queries[%d].entity is required", i)
		}
		if strings.TrimSpace(q.File) == "" {
			return fmt.Errorf("config: queries[%d].file is required", i)
		}
	}
	for i, r := range c.Relations {
		if strings.TrimSpace(r.From) == "" {
			return fmt.Errorf("config: relations[%d].from is required", i)
		}
		if strings.TrimSpace(r.FromColumn) == "" {
			return fmt.Errorf("config: relations[%d].from_column is required", i)
		}
		if strings.TrimSpace(r.To) == "" {
			return fmt.Errorf("config: relations[%d].to is required", i)
		}
		if strings.TrimSpace(r.ToColumn) == "" {
			return fmt.Errorf("config: relations[%d].to_column is required", i)
		}
		if r.Join != "" {
			switch strings.ToLower(strings.TrimSpace(r.Join)) {
			case "inner", "left", "right", "full":
			default:
				return fmt.Errorf("config: relations[%d].join must be inner, left, right, or full", i)
			}
		}
	}
	return nil
}

// ResolvedRoutine is an absolute routines file path.
type ResolvedRoutine struct {
	File string
}

// ResolvePaths returns absolute paths for migrations, output, query files, and routines.
func (c *Config) ResolvePaths() (migrations, output string, queryFiles []ResolvedQuery, routineFiles []ResolvedRoutine, err error) {
	migrations = resolvePath(c.configDir, c.Migrations)
	output = resolvePath(c.configDir, c.Output)
	for _, q := range c.Queries {
		queryFiles = append(queryFiles, ResolvedQuery{
			Entity: q.Entity,
			File:   resolvePath(c.configDir, q.File),
		})
	}
	for _, r := range c.Routines {
		routineFiles = append(routineFiles, ResolvedRoutine{
			File: resolvePath(c.configDir, r.File),
		})
	}
	return migrations, output, queryFiles, routineFiles, nil
}

func resolvePath(base, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}

// EntitiesEnabled reports whether entity structs should be generated.
func (c *Config) EntitiesEnabled() bool {
	if c.Generate.Entities == nil {
		return true
	}
	return *c.Generate.Entities
}

// RepositoriesEnabled reports whether repositories should be generated.
func (c *Config) RepositoriesEnabled() bool {
	if c.Generate.Repositories == nil {
		return true
	}
	return *c.Generate.Repositories
}

// QueryHelpersEnabled reports whether built-in query helpers should be generated.
func (c *Config) QueryHelpersEnabled() bool {
	if c.Generate.QueryHelpers == nil {
		return true
	}
	return *c.Generate.QueryHelpers
}

// RoutinesEnabled reports whether database routine wrappers should be generated.
func (c *Config) RoutinesEnabled() bool {
	if c.Generate.Routines == nil {
		return len(c.Routines) > 0
	}
	return *c.Generate.Routines
}

// MigrationSchema returns the configured migration tracker schema.
func (c *Config) MigrationSchema() string {
	return c.Migrate.Schema
}

// MigrationTable returns the configured migration tracker table name.
func (c *Config) MigrationTable() string {
	if c.Migrate.Table == "" {
		return "ameego_migrations"
	}
	return c.Migrate.Table
}

// ConfigDir returns the directory containing ameego.yaml.
func (c *Config) ConfigDir() string {
	if c.configDir == "" {
		return "."
	}
	return c.configDir
}

// SetBaseDir sets the base directory for resolving relative paths.
func (c *Config) SetBaseDir(dir string) {
	c.configDir = dir
}

// MergeFlags overlays CLI flag values onto config when flags are non-empty.
func (c *Config) MergeFlags(dialect, migrations, output, pkg, module string, withQueries *bool) {
	if dialect != "" {
		c.Dialect = dialect
	}
	if migrations != "" {
		c.Migrations = migrations
	}
	if output != "" {
		c.Output = output
	}
	if pkg != "" {
		c.Package = pkg
	}
	if module != "" {
		c.Module = module
	}
	if withQueries != nil {
		c.Generate.QueryHelpers = withQueries
	}
}
