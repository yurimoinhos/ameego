package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/yurimoinhos/ameego/internal/dialect"
)

const DefaultTable = "ameego_migrations"

var versionPrefixRe = regexp.MustCompile(`^(\d+)[^0-9]`)

// Config controls migration execution.
type Config struct {
	Dialect dialect.Dialect
	Dir     string
	Schema  string
	Table   string
}

// Migration represents a single migration file.
type Migration struct {
	Version  string
	Filename string
	Path     string
	SQL      string
	Checksum string
}

// Status describes applied vs pending migrations.
type Status struct {
	Applied  []string
	Pending  []Migration
	All      []Migration
}

// Runner applies versioned SQL migrations.
type Runner struct {
	db   *sqlx.DB
	cfg  Config
	fqn  string
}

// NewRunner creates a migration runner.
func NewRunner(db *sqlx.DB, cfg Config) (*Runner, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("migrate: migrations directory is required")
	}
	if cfg.Dialect == nil {
		return nil, fmt.Errorf("migrate: dialect is required")
	}
	if cfg.Table == "" {
		cfg.Table = DefaultTable
	}
	if cfg.Schema == "" {
		switch cfg.Dialect.Name() {
		case "postgres":
			cfg.Schema = "public"
		case "mssql":
			cfg.Schema = "dbo"
		default:
			cfg.Schema = ""
		}
	}
	return &Runner{db: db, cfg: cfg, fqn: dialect.MigrationTableName(cfg.Dialect, cfg.Schema, cfg.Table)}, nil
}

// EnsureSchema creates the migration schema and tracking table.
func (r *Runner) EnsureSchema(ctx context.Context) error {
	if ddl := dialect.EnsureSchemaDDL(r.cfg.Dialect, r.cfg.Schema); ddl != "" {
		if _, err := r.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("migrate: ensure schema: %w", err)
		}
	}
	ddl := dialect.MigrationTableDDL(r.cfg.Dialect, r.cfg.Schema, r.cfg.Table)
	if _, err := r.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("migrate: ensure migration table: %w", err)
	}
	return nil
}

// Load reads migration files from disk sorted by version.
func Load(dir string) ([]Migration, error) {
	var files []Migration
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".sql") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		version := versionFromFilename(d.Name())
		sum := sha256.Sum256(raw)
		files = append(files, Migration{
			Version:  version,
			Filename: d.Name(),
			Path:     path,
			SQL:      string(raw),
			Checksum: hex.EncodeToString(sum[:]),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("migrate: walk %s: %w", dir, err)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Version < files[j].Version
	})
	return files, nil
}

func versionFromFilename(name string) string {
	if m := versionPrefixRe.FindStringSubmatch(name); len(m) == 2 {
		return m[1]
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// Applied returns migration versions already recorded.
func (r *Runner) Applied(ctx context.Context) ([]string, error) {
	if err := r.EnsureSchema(ctx); err != nil {
		return nil, err
	}
	q := fmt.Sprintf("SELECT version FROM %s ORDER BY version", r.fqn)
	var versions []string
	if err := r.db.SelectContext(ctx, &versions, q); err != nil {
		return nil, fmt.Errorf("migrate: list applied: %w", err)
	}
	return versions, nil
}

// Status compares disk migrations with applied versions.
func (r *Runner) Status(ctx context.Context) (Status, error) {
	all, err := Load(r.cfg.Dir)
	if err != nil {
		return Status{}, err
	}
	applied, err := r.Applied(ctx)
	if err != nil {
		return Status{}, err
	}
	appliedSet := map[string]struct{}{}
	for _, v := range applied {
		appliedSet[v] = struct{}{}
	}
	var pending []Migration
	for _, m := range all {
		if _, ok := appliedSet[m.Version]; !ok {
			pending = append(pending, m)
		}
	}
	return Status{Applied: applied, Pending: pending, All: all}, nil
}

// Up applies all pending migrations inside a single transaction.
func (r *Runner) Up(ctx context.Context) (int, error) {
	status, err := r.Status(ctx)
	if err != nil {
		return 0, err
	}
	if len(status.Pending) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("migrate: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	applied := 0
	for _, m := range status.Pending {
		if err := r.applyOne(ctx, tx, m); err != nil {
			return applied, err
		}
		applied++
	}
	if err := tx.Commit(); err != nil {
		return applied, fmt.Errorf("migrate: commit: %w", err)
	}
	return applied, nil
}

func (r *Runner) applyOne(ctx context.Context, tx *sqlx.Tx, m Migration) error {
	for _, stmt := range splitStatements(m.SQL) {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %s: %w", m.Filename, err)
		}
	}
	insert := fmt.Sprintf("INSERT INTO %s (version, checksum) VALUES (%s, %s)",
		r.fqn, r.cfg.Dialect.Placeholder(1), r.cfg.Dialect.Placeholder(2))
	if _, err := tx.ExecContext(ctx, insert, m.Version, m.Checksum); err != nil {
		return fmt.Errorf("migrate: record %s: %w", m.Version, err)
	}
	return nil
}

// SplitStatements splits SQL on semicolons outside of quotes and dollar blocks.
func SplitStatements(sql string) []string {
	return splitStatements(sql)
}

// splitStatements splits SQL on semicolons outside of quotes and dollar blocks.
func splitStatements(sql string) []string {
	var stmts []string
	var b strings.Builder
	inSingle := false
	inDouble := false
	inDollar := false
	dollarTag := ""

	for i := 0; i < len(sql); i++ {
		ch := sql[i]

		if inDollar {
			b.WriteByte(ch)
			if ch == '$' && i+len(dollarTag) < len(sql) {
				if strings.HasPrefix(sql[i:], dollarTag) {
					b.WriteString(dollarTag[1:])
					i += len(dollarTag) - 1
					inDollar = false
					dollarTag = ""
				}
			}
			continue
		}

		if !inSingle && !inDouble && ch == '$' {
			tagEnd := strings.Index(sql[i+1:], "$")
			if tagEnd >= 0 {
				dollarTag = sql[i : i+tagEnd+2]
				inDollar = true
				b.WriteByte(ch)
				continue
			}
		}

		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
			b.WriteByte(ch)
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
			b.WriteByte(ch)
		case ';':
			if !inSingle && !inDouble {
				if s := strings.TrimSpace(b.String()); s != "" {
					stmts = append(stmts, s)
				}
				b.Reset()
			} else {
				b.WriteByte(ch)
			}
		default:
			b.WriteByte(ch)
		}
	}
	if s := strings.TrimSpace(b.String()); s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}

// OpenDB opens a database connection for migrations.
func OpenDB(driverName, dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// IsNoRows reports sql.ErrNoRows.
func IsNoRows(err error) bool {
	return err == sql.ErrNoRows
}
