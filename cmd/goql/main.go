package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yurimoinhos/ameego/internal/config"
	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/generator"
	"github.com/yurimoinhos/ameego/internal/parser"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate", "gen":
		if err := runGenerate(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "goql: %v\n", err)
			os.Exit(1)
		}
	case "version", "-v", "--version":
		fmt.Println("goql dev")
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "goql: unknown command %q\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	configPath := fs.String("config", config.DefaultConfigFile, "path to goql.yaml")
	migrations := fs.String("migrations", "", "override migrations directory from config")
	dialectName := fs.String("dialect", "", "override dialect from config")
	out := fs.String("out", "", "override output directory from config")
	pkg := fs.String("package", "", "override generated Go package name")
	module := fs.String("module", "", "override Go module import path")
	withQueriesVal := true
	fs.BoolVar(&withQueriesVal, "queries", true, "generate typed query helpers")
	noConfig := fs.Bool("no-config", false, "ignore goql.yaml and use flags only")
	if err := fs.Parse(args); err != nil {
		return err
	}
	queriesFlagSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "queries" {
			queriesFlagSet = true
		}
	})

	var cfg *config.Config
	var err error
	configFile := *configPath

	if !*noConfig {
		if _, statErr := os.Stat(configFile); statErr == nil {
			cfg, err = config.Load(configFile)
			if err != nil {
				return err
			}
		} else if !*noConfig && configFile != config.DefaultConfigFile {
			return fmt.Errorf("config file not found: %s", configFile)
		}
	}

	if cfg == nil {
		cfg = &config.Config{
			Dialect:    "postgres",
			Migrations: "migrations",
			Output:     "generated",
			Package:    "generated",
		}
		cfg.SetBaseDir(".")
	}

	var withQueriesOverride *bool
	if queriesFlagSet {
		withQueriesOverride = &withQueriesVal
	}
	cfg.MergeFlags(*dialectName, *migrations, *out, *pkg, *module, withQueriesOverride)

	migrationsDir, outDir, queryFiles, err := cfg.ResolvePaths()
	if err != nil {
		return err
	}

	d, err := dialect.ParseDialect(cfg.Dialect)
	if err != nil {
		return err
	}

	schema, err := parser.ParseMigrations(parser.Options{MigrationsDir: migrationsDir})
	if err != nil {
		return err
	}
	if len(schema.Tables) == 0 {
		return fmt.Errorf("no tables found in %s", migrationsDir)
	}

	outAbs, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}

	bindings, err := loadQueryBindings(queryFiles)
	if err != nil {
		return err
	}

	configAbs, _ := filepath.Abs(configFile)
	if !fileExists(configFile) {
		configAbs = config.DefaultConfigFile
	}

	return generator.Generate(generator.Config{
		Package:          cfg.Package,
		OutputDir:        outAbs,
		Dialect:          d,
		Schema:           schema,
		ModulePath:       cfg.Module,
		ConfigPath:       configAbs,
		WithQueries:      cfg.QueryHelpersEnabled(),
		WithEntities:     cfg.EntitiesEnabled(),
		WithRepositories: cfg.RepositoriesEnabled(),
		QueryBindings:    bindings,
	})
}

func loadQueryBindings(files []config.ResolvedQuery) ([]generator.QueryBinding, error) {
	var bindings []generator.QueryBinding
	for _, f := range files {
		queries, err := parser.ParseQueryFile(f.File)
		if err != nil {
			return nil, err
		}
		if len(queries) == 0 {
			return nil, fmt.Errorf("no named queries found in %s", f.File)
		}
		bindings = append(bindings, generator.QueryBinding{
			Entity:  f.Entity,
			Queries: queries,
		})
	}
	return bindings, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func printUsage() {
	fmt.Println(`goql — compile-time SQL ORM codegen for Go (sqlx, no reflection)

Usage:
  goql generate [flags]

Configuration:
  By default goql reads goql.yaml from the current directory.
  CLI flags override values from the config file.

Flags:
  --config      Path to goql.yaml (default "goql.yaml")
  --no-config   Ignore goql.yaml and use flags only
  --migrations  Override migrations directory
  --dialect     Override dialect: postgres, mssql, mysql, sqlite
  --out         Override output directory
  --package     Override generated Go package name
  --module      Override Go module import path
  --queries     Generate typed query helpers (default true)

Examples:
  goql generate
  goql generate --config ./examples/basic/goql.yaml
  goql generate --no-config --migrations ./migrations --dialect mssql --out ./gen --package gen`)
}
