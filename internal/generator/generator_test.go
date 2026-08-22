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

func TestGenerate_PerTablePackages(t *testing.T) {
	migrations := filepath.Join("..", "..", "examples", "basic", "migrations")
	out := t.TempDir()

	schema, err := parser.ParseMigrations(parser.Options{MigrationsDir: migrations})
	if err != nil {
		t.Fatal(err)
	}
	d := dialect.Postgres{}
	if err := generator.Generate(generator.Config{
		Package:          "store",
		OutputDir:        out,
		Dialect:          d,
		Schema:           schema,
		WithQueries:      true,
		WithEntities:     true,
		WithRepositories: true,
	}); err != nil {
		t.Fatal(err)
	}

	userEntity := filepath.Join(out, "tables", "users", "entity_gen.go")
	data, err := os.ReadFile(userEntity)
	if err != nil {
		t.Fatalf("missing per-table entity: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "package users") {
		t.Fatalf("expected package users, got: %s", src[:min(80, len(src))])
	}
	if !strings.Contains(src, "type User struct") {
		t.Fatal("expected User struct")
	}

	userRepo := filepath.Join(out, "tables", "users", "repository_gen.go")
	repoData, err := os.ReadFile(userRepo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(repoData), "selectAllSQL") {
		t.Fatal("expected precompiled SQL constants in table package")
	}

	doc, err := os.ReadFile(filepath.Join(out, "doc_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), "package store") {
		t.Fatal("expected root doc package")
	}
}

func TestGenerate_CustomQueriesPerTable(t *testing.T) {
	migrations := filepath.Join("..", "..", "examples", "basic", "migrations")
	queriesFile := filepath.Join("..", "..", "examples", "basic", "queries", "users.sql")
	out := t.TempDir()

	schema, err := parser.ParseMigrations(parser.Options{MigrationsDir: migrations})
	if err != nil {
		t.Fatal(err)
	}
	queries, err := parser.ParseQueryFile(queriesFile)
	if err != nil {
		t.Fatal(err)
	}
	d := dialect.Postgres{}
	if err := generator.Generate(generator.Config{
		Package:          "store",
		OutputDir:        out,
		Dialect:          d,
		Schema:           schema,
		WithEntities:     true,
		WithRepositories: true,
		QueryBindings: []generator.QueryBinding{{
			Entity:  "User",
			Queries: queries,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	custom, err := os.ReadFile(filepath.Join(out, "tables", "users", "custom_queries_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(custom), "FindByEmail") {
		t.Fatal("expected FindByEmail in users package")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
