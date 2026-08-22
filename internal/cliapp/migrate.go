package cliapp

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/yurimoinhos/ameego/internal/config"
	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/migrate"
	"github.com/yurimoinhos/ameego/pkg/goql"
)

// RunMigrate executes migration subcommands.
func RunMigrate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("migrate: expected subcommand (up, status)")
	}

	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	configPath := fs.String("config", config.DefaultConfigFile, "path to goql.yaml")
	dsn := fs.String("dsn", os.Getenv("DATABASE_URL"), "database connection string")
	driver := fs.String("driver", "", "database driver name (default: inferred from dialect)")
	schema := fs.String("schema", "", "override migration tracker schema")
	table := fs.String("table", "", "override migration tracker table name")
	migrations := fs.String("migrations", "", "override migrations directory")
	dialectName := fs.String("dialect", "", "override dialect from config")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	subcmd := args[0]
	cfg, migrationsDir, err := loadMigrateConfig(*configPath, *migrations, *dialectName, *schema, *table)
	if err != nil {
		return err
	}
	if *dsn == "" {
		return fmt.Errorf("migrate: --dsn or DATABASE_URL is required")
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
		return fmt.Errorf("migrate: connect: %w", err)
	}
	defer db.Close()

	runner, err := migrate.NewRunner(db, migrate.Config{
		Dialect: d,
		Dir:     migrationsDir,
		Schema:  cfg.MigrationSchema(),
		Table:   cfg.MigrationTable(),
	})
	if err != nil {
		return err
	}

	ctx := context.Background()
	switch subcmd {
	case "up":
		n, err := runner.Up(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("applied %d migration(s)\n", n)
		return nil
	case "status":
		status, err := runner.Status(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("schema: %s\n", cfg.MigrationSchema())
		fmt.Printf("table:  %s\n", dialect.MigrationTableName(d, cfg.MigrationSchema(), cfg.MigrationTable()))
		fmt.Printf("applied: %d\n", len(status.Applied))
		for _, v := range status.Applied {
			fmt.Printf("  ✓ %s\n", v)
		}
		fmt.Printf("pending: %d\n", len(status.Pending))
		for _, m := range status.Pending {
			fmt.Printf("  ○ %s (%s)\n", m.Version, m.Filename)
		}
		return nil
	default:
		return fmt.Errorf("migrate: unknown subcommand %q (use up, status)", subcmd)
	}
}

func loadMigrateConfig(configPath, migrationsOverride, dialectOverride, schemaOverride, tableOverride string) (*config.Config, string, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		if configPath != config.DefaultConfigFile {
			return nil, "", err
		}
		cfg = &config.Config{
			Dialect:    "postgres",
			Migrations: "migrations",
		}
		cfg.SetBaseDir(".")
	}
	if dialectOverride != "" {
		cfg.Dialect = dialectOverride
	}
	if migrationsOverride != "" {
		cfg.Migrations = migrationsOverride
	}
	if schemaOverride != "" {
		cfg.Migrate.Schema = schemaOverride
	}
	if tableOverride != "" {
		cfg.Migrate.Table = tableOverride
	}
	migrationsDir, _, _, _, err := cfg.ResolvePaths()
	if err != nil {
		return nil, "", err
	}
	return cfg, migrationsDir, nil
}

// unused import guard for sql in future down migrations
var _ = sql.ErrNoRows
