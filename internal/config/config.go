package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultConfigFile = "goql.yaml"

// Config is the root goql.yaml configuration.
type Config struct {
	Version    string          `yaml:"version"`
	Dialect    string          `yaml:"dialect"`
	Migrations string          `yaml:"migrations"`
	Output     string          `yaml:"output"`
	Package    string          `yaml:"package"`
	Module     string          `yaml:"module"`
	Generate   GenerateConfig  `yaml:"generate"`
	Queries    []QueryFileSpec `yaml:"queries"`

	configDir string
}

// GenerateConfig toggles generated artifacts.
type GenerateConfig struct {
	Entities     *bool `yaml:"entities"`
	Repositories *bool `yaml:"repositories"`
	QueryHelpers *bool `yaml:"query_helpers"`
}

// QueryFileSpec binds a SQL query file to an entity repository.
type QueryFileSpec struct {
	Entity string `yaml:"entity"`
	File   string `yaml:"file"`
}

// ResolvedQuery is an absolute query file binding.
type ResolvedQuery struct {
	Entity string
	File   string
}

// Load reads and validates a goql.yaml file.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigFile
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(abs)
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
	return nil
}

// ResolvePaths returns absolute paths for migrations, output, and query files.
func (c *Config) ResolvePaths() (migrations, output string, queryFiles []ResolvedQuery, err error) {
	migrations = resolvePath(c.configDir, c.Migrations)
	output = resolvePath(c.configDir, c.Output)
	for _, q := range c.Queries {
		queryFiles = append(queryFiles, ResolvedQuery{
			Entity: q.Entity,
			File:   resolvePath(c.configDir, q.File),
		})
	}
	return migrations, output, queryFiles, nil
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

// ConfigDir returns the directory containing goql.yaml.
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
