package cliapp

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

// Run is the shared CLI entrypoint for cmd/ameego and cmd/goql.
func Run(args []string) int {
	if len(args) < 2 {
		printUsage()
		return 1
	}

	switch args[1] {
	case "generate", "gen":
		if err := RunGenerate(args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "ameego: %v\n", err)
			return 1
		}
		return 0
	case "migrate":
		if err := RunMigrate(args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "ameego: %v\n", err)
			return 1
		}
		return 0
	case "version", "-v", "--version":
		fmt.Printf("ameego v%s\n", Version)
		return 0
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "ameego: unknown command %q\n", args[1])
		printUsage()
		return 1
	}
}

// RunGenerate executes the codegen pipeline.
func RunGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	configPath := fs.String("config", config.DefaultConfigFile, "path to ameego.yaml")
	migrations := fs.String("migrations", "", "override migrations directory from config")
	dialectName := fs.String("dialect", "", "override dialect from config")
	out := fs.String("out", "", "override output directory from config")
	pkg := fs.String("package", "", "override generated Go package name")
	module := fs.String("module", "", "override Go module import path")
	withQueriesVal := true
	fs.BoolVar(&withQueriesVal, "queries", true, "generate typed query helpers")
	noConfig := fs.Bool("no-config", false, "ignore ameego.yaml and use flags only")
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
		} else if configFile != config.DefaultConfigFile {
			return fmt.Errorf("config file not found: %s", configFile)
		}
	}

	if cfg == nil {
		cfg = &config.Config{
			Dialect:    "postgres",
			Migrations: "migrations",
			Output:     "generated",
			Package:    "store",
		}
		cfg.SetBaseDir(".")
	}

	var withQueriesOverride *bool
	if queriesFlagSet {
		withQueriesOverride = &withQueriesVal
	}
	cfg.MergeFlags(*dialectName, *migrations, *out, *pkg, *module, withQueriesOverride)

	migrationsDir, outDir, queryFiles, routineFiles, err := cfg.ResolvePaths()
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

	routineBindings, err := loadRoutineBindings(routineFiles, d)
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
		WithRoutines:     cfg.RoutinesEnabled(),
		QueryBindings:    bindings,
		RoutineBindings:  routineBindings,
	})
}

func loadRoutineBindings(files []config.ResolvedRoutine, d dialect.Dialect) ([]generator.RoutineBinding, error) {
	var bindings []generator.RoutineBinding
	for _, f := range files {
		routines, err := parser.ParseRoutineFile(f.File, d)
		if err != nil {
			return nil, err
		}
		if len(routines) == 0 {
			return nil, fmt.Errorf("no routines found in %s", f.File)
		}
		bindings = append(bindings, generator.RoutineBinding{
			SourceFile: f.File,
			Routines:   routines,
		})
	}
	return bindings, nil
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
	fmt.Println(`ameego — compile-time type-safe SQL codegen for Go (sqlx, no reflection)

Usage:
  ameego generate [flags]
  ameego migrate up|status [flags]

Install:
  go install github.com/yurimoinhos/ameego/cmd/ameego@latest

Configuration:
  By default ameego reads ameego.yaml from the current directory.
  CLI flags override values from the config file.

Flags:
  --config      Path to ameego.yaml (default "ameego.yaml")
  --no-config   Ignore ameego.yaml and use flags only
  --migrations  Override migrations directory
  --dialect     Override dialect: postgres, mssql, mysql, sqlite
  --out         Override output directory
  --package     Override root Go package name (default "store")
  --module      Override Go module import path prefix for generated packages
  --queries     Generate typed query helpers (default true)

Output layout:
  <out>/doc_gen.go              root package (go:generate)
  <out>/tables/<table>/         one Go package per table
  <out>/functions/<name>/       one Go package per database function
  <out>/procedures/<name>/      one Go package per database procedure
  <out>/triggers/<name>/        one Go package per database trigger

Examples:
  ameego generate
  ameego generate --config ./examples/basic/ameego.yaml
  ameego generate --no-config --migrations ./migrations --dialect mssql --out ./gen`)
}
