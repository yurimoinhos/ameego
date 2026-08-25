package config_test

import (
	"path/filepath"
	"testing"

	"github.com/yurimoinhos/ameego/internal/config"
)

func TestLoadExampleConfig(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "basic", "goql.yaml")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dialect != "postgres" {
		t.Fatalf("dialect: got %q", cfg.Dialect)
	}
	if len(cfg.Queries) != 2 {
		t.Fatalf("expected 2 query bindings, got %d", len(cfg.Queries))
	}
	migrations, output, queries, _, err := cfg.ResolvePaths()
	if err != nil {
		t.Fatal(err)
	}
	if migrations == "" || output == "" {
		t.Fatal("expected resolved paths")
	}
	if len(queries) != 2 {
		t.Fatalf("expected 2 resolved queries, got %d", len(queries))
	}
}

func TestLoadAmeegoRelations(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "basic", "ameego.yaml")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(cfg.Relations))
	}
	if cfg.Relations[0].From != "active_users" {
		t.Fatalf("from: %q", cfg.Relations[0].From)
	}
}
