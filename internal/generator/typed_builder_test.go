package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/generator"
	"github.com/yurimoinhos/ameego/internal/parser"
)

// TestGenerate_TypedBuilder verifies the A2 schema+dialect-aware query builder:
// per-column methods exist, and MSSQL-only methods are generated only for mssql.
func TestGenerate_TypedBuilder(t *testing.T) {
	migrations := filepath.Join("..", "..", "examples", "basic", "migrations")
	schema, err := parser.ParseMigrations(parser.Options{MigrationsDir: migrations})
	if err != nil {
		t.Fatal(err)
	}

	// Postgres: no MSSQL-only methods.
	pgOut := t.TempDir()
	if err := generator.Generate(generator.Config{
		Package: "store", OutputDir: pgOut, Dialect: dialect.Postgres{},
		Schema: schema, WithQueries: true, WithEntities: true, WithRepositories: true,
	}); err != nil {
		t.Fatal(err)
	}
	pgBuilder := readFile(t, filepath.Join(pgOut, "tables", "users", "typed_builder_gen.go"))
	if !strings.Contains(pgBuilder, "type UserQuery struct") {
		t.Fatal("postgres: expected UserQuery typed builder")
	}
	if !strings.Contains(pgBuilder, "func (b *UserQuery) WhereIdEq(") {
		t.Fatal("postgres: expected per-column WhereIdEq method")
	}
	if !strings.Contains(pgBuilder, "dialect.Postgres{}") {
		t.Fatal("postgres: expected Postgres dialect instance")
	}
	if strings.Contains(pgBuilder, ") Top(") || strings.Contains(pgBuilder, "MSSQLTop") {
		t.Fatal("postgres: MSSQL-only Top() must NOT be generated")
	}

	// MSSQL: Top/TableHint/Option present.
	msOut := t.TempDir()
	if err := generator.Generate(generator.Config{
		Package: "store", OutputDir: msOut, Dialect: dialect.MSSQL{},
		Schema: schema, WithQueries: true, WithEntities: true, WithRepositories: true,
	}); err != nil {
		t.Fatal(err)
	}
	msBuilder := readFile(t, filepath.Join(msOut, "tables", "users", "typed_builder_gen.go"))
	if !strings.Contains(msBuilder, "func (b *UserQuery) Top(n int)") {
		t.Fatal("mssql: expected gated Top() method")
	}
	if !strings.Contains(msBuilder, "func (b *UserQuery) TableHint(") {
		t.Fatal("mssql: expected gated TableHint() method")
	}
	if !strings.Contains(msBuilder, "dialect.MSSQL{}") {
		t.Fatal("mssql: expected MSSQL dialect instance")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
