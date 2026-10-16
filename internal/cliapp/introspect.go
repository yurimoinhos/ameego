package cliapp

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yurimoinhos/ameego/internal/config"
	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/generator"
	"github.com/yurimoinhos/ameego/internal/introspect"
	"github.com/yurimoinhos/ameego/internal/migrate"
	"github.com/yurimoinhos/ameego/internal/schema"
	"github.com/yurimoinhos/ameego/pkg/goql"
)

// RunIntrospect reverse-engineers a live database into a migration file or,
// with --generate, straight into generated Go packages.
func RunIntrospect(args []string) error {
	fs := flag.NewFlagSet("introspect", flag.ExitOnError)
	configPath := fs.String("config", config.DefaultConfigFile, "path to ameego.yaml")
	dsn := fs.String("dsn", os.Getenv("DATABASE_URL"), "database connection string")
	driver := fs.String("driver", "", "database/sql driver name (default: inferred from dialect)")
	dialectName := fs.String("dialect", "", "override dialect from config")
	schemaNS := fs.String("schema", "", "database schema/namespace to read (postgres: public, mssql: dbo, mysql: database)")
	includeViews := fs.Bool("views", false, "also reverse-engineer views as read-only entities")
	out := fs.String("out", "", "output .sql migration file (default: <migrations>/000_introspected.sql)")
	generate := fs.Bool("generate", false, "generate Go packages directly instead of writing a migration file")
	outDir := fs.String("gen-out", "", "override generated output dir when --generate (default from config)")
	pkg := fs.String("package", "", "override generated Go package name when --generate")
	module := fs.String("module", "", "override Go module import path when --generate")
	noConfig := fs.Bool("no-config", false, "ignore ameego.yaml and use flags only")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := loadIntrospectConfig(*configPath, *noConfig, *dialectName)
	if err != nil {
		return err
	}
	if *dsn == "" {
		return fmt.Errorf("introspect: --dsn or DATABASE_URL is required")
	}

	d, err := dialect.ParseDialect(cfg.Dialect)
	if err != nil {
		return err
	}

	driverName := *driver
	if driverName == "" {
		driverName, _, err = goql.DriverForDialect(cfg.Dialect)
		if err != nil {
			return err
		}
	}

	db, err := migrate.OpenDB(driverName, *dsn)
	if err != nil {
		return fmt.Errorf("introspect: connect: %w (is the %q driver imported/registered?)", err, driverName)
	}
	defer db.Close()

	ins, err := introspect.For(d)
	if err != nil {
		return err
	}

	ctx := context.Background()
	s, err := ins.Introspect(ctx, db.DB, introspect.Options{
		Schema:       *schemaNS,
		IncludeViews: *includeViews,
	})
	if err != nil {
		return err
	}
	if len(s.Tables) == 0 {
		return fmt.Errorf("introspect: no tables found in schema %q", *schemaNS)
	}

	if *generate {
		return introspectGenerate(cfg, d, s, *outDir, *pkg, *module)
	}
	return introspectWriteDDL(cfg, d, s, *out)
}

// introspectWriteDDL renders the schema to a migration .sql file so the regular
// `generate` command remains the single codegen path.
func introspectWriteDDL(cfg *config.Config, d dialect.Dialect, s *schema.Schema, outPath string) error {
	ddl := introspect.EmitDDL(d, s)

	target := outPath
	if target == "" {
		migrationsDir, _, _, _, err := cfg.ResolvePaths()
		if err != nil {
			return err
		}
		target = filepath.Join(migrationsDir, "000_introspected.sql")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("introspect: mkdir %s: %w", filepath.Dir(target), err)
	}
	if err := os.WriteFile(target, []byte(ddl), 0o644); err != nil {
		return fmt.Errorf("introspect: write %s: %w", target, err)
	}
	fmt.Printf("introspected %d table(s) into %s\n", countTables(s), target)
	fmt.Printf("next: review the file, then run `ameego generate`\n")
	return nil
}

// introspectGenerate feeds the reverse-engineered schema straight into codegen.
func introspectGenerate(cfg *config.Config, d dialect.Dialect, s *schema.Schema, outDirOverride, pkgOverride, moduleOverride string) error {
	_, outDir, _, _, err := cfg.ResolvePaths()
	if err != nil {
		return err
	}
	if outDirOverride != "" {
		outDir = outDirOverride
	}
	outAbs, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}
	pkg := cfg.Package
	if pkgOverride != "" {
		pkg = pkgOverride
	}
	module := cfg.Module
	if moduleOverride != "" {
		module = moduleOverride
	}

	err = generator.Generate(generator.Config{
		Package:          pkg,
		OutputDir:        outAbs,
		Dialect:          d,
		Schema:           s,
		ModulePath:       module,
		ConfigPath:       config.DefaultConfigFile,
		WithQueries:      cfg.QueryHelpersEnabled(),
		WithEntities:     cfg.EntitiesEnabled(),
		WithRepositories: cfg.RepositoriesEnabled(),
		WithRoutines:     false,
	})
	if err != nil {
		return err
	}
	fmt.Printf("introspected %d table(s) and generated packages into %s\n", countTables(s), outAbs)
	return nil
}

func countTables(s *schema.Schema) int {
	n := 0
	for _, t := range s.Tables {
		if !t.IsView() {
			n++
		}
	}
	return n
}

func loadIntrospectConfig(configPath string, noConfig bool, dialectOverride string) (*config.Config, error) {
	var cfg *config.Config
	if !noConfig {
		if _, statErr := os.Stat(configPath); statErr == nil {
			loaded, err := config.Load(configPath)
			if err != nil {
				return nil, err
			}
			cfg = loaded
		} else if configPath != config.DefaultConfigFile {
			return nil, fmt.Errorf("config file not found: %s", configPath)
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
	if dialectOverride != "" {
		cfg.Dialect = dialectOverride
	}
	return cfg, nil
}
