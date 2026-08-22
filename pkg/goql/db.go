// Package goql provides a compile-time typed database access layer built on sqlx.
// All entity mapping is generated at build time — no reflection at runtime.
package goql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/jmoiron/sqlx"
)

// DB wraps sqlx.DB with dialect-aware SQL helpers.
type DB struct {
	inner   sqlxQueryer
	dialect dialect.Dialect
}

type sqlxQueryer interface {
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Open connects to the database using the given driver and DSN.
func Open(driverName, dsn, dialectName string) (*DB, error) {
	d, err := dialect.ParseDialect(dialectName)
	if err != nil {
		return nil, err
	}
	inner, err := sqlx.Connect(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("goql: connect: %w", err)
	}
	return &DB{inner: inner, dialect: d}, nil
}

// FromSQLX wraps an existing *sqlx.DB.
func FromSQLX(db *sqlx.DB, dialectName string) (*DB, error) {
	d, err := dialect.ParseDialect(dialectName)
	if err != nil {
		return nil, err
	}
	return &DB{inner: db, dialect: d}, nil
}

// WithTx returns a DB bound to a transaction.
func (db *DB) WithTx(tx *sqlx.Tx) *DB {
	return &DB{inner: tx, dialect: db.dialect}
}

// Begin starts a transaction.
func (db *DB) BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error) {
	type beginTx interface {
		BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)
	}
	b, ok := db.inner.(beginTx)
	if !ok {
		return nil, fmt.Errorf("goql: underlying driver does not support transactions")
	}
	return b.BeginTxx(ctx, opts)
}

// Placeholder returns a dialect-specific bind placeholder.
func (db *DB) Placeholder(n int) string {
	return db.dialect.Placeholder(n)
}

// QuoteIdent quotes an identifier for the active dialect.
func (db *DB) QuoteIdent(name string) string {
	return db.dialect.QuoteIdent(name)
}

// QuoteIdentList quotes and joins identifiers.
func (db *DB) QuoteIdentList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = db.QuoteIdent(n)
	}
	return strings.Join(quoted, ", ")
}

// DialectName returns the configured dialect name.
func (db *DB) DialectName() string {
	return db.dialect.Name()
}

// GetContext scans a single row into dest using sqlx struct tags.
func (db *DB) GetContext(ctx context.Context, dest any, query string, args ...any) error {
	return db.inner.GetContext(ctx, dest, query, args...)
}

// SelectContext scans rows into dest using sqlx struct tags.
func (db *DB) SelectContext(ctx context.Context, dest any, query string, args ...any) error {
	return db.inner.SelectContext(ctx, dest, query, args...)
}

// ExecContext executes a statement.
func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.inner.ExecContext(ctx, query, args...)
}

// ErrCompositePK is returned when composite PK CRUD is not generated.
var ErrCompositePK = errors.New("goql: composite primary key not supported for this operation")

// IsNotFound reports whether err is sql.ErrNoRows.
func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// DriverForDialect returns the recommended database/sql driver import path.
func DriverForDialect(dialectName string) (driverName, importPath string, err error) {
	switch strings.ToLower(dialectName) {
	case "postgres", "postgresql", "pg":
		return "pgx", "github.com/jackc/pgx/v5/stdlib", nil
	case "mssql", "sqlserver", "sql-server":
		return "sqlserver", "github.com/microsoft/go-mssqldb", nil
	case "mysql", "mariadb":
		return "mysql", "github.com/go-sql-driver/mysql", nil
	case "sqlite", "sqlite3":
		return "sqlite3", "github.com/mattn/go-sqlite3", nil
	default:
		return "", "", fmt.Errorf("unknown dialect %q", dialectName)
	}
}
