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

func TestGenerate_Postgres(t *testing.T) {
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

	for _, name := range []string{"entities_gen.go", "repositories_gen.go", "queries_gen.go", "doc_gen.go"} {
		path := filepath.Join(out, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
		src := string(data)
		if !strings.Contains(src, "package store") {
			t.Fatalf("%s: wrong package", name)
		}
	}
	entities, _ := os.ReadFile(filepath.Join(out, "entities_gen.go"))
	if !strings.Contains(string(entities), "type User struct") {
		t.Fatal("expected User struct")
	}
	repos, _ := os.ReadFile(filepath.Join(out, "repositories_gen.go"))
	if !strings.Contains(string(repos), "userSelectAllSQL") {
		t.Fatal("expected builder-generated SQL constants")
	}
}

func TestGenerate_CustomQueries(t *testing.T) {
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
	custom, err := os.ReadFile(filepath.Join(out, "custom_queries_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(custom), "FindByEmail") {
		t.Fatal("expected FindByEmail method")
	}
}
