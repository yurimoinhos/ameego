package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yurimoinhos/ameego/internal/parser"
)

func TestParsePostgresDomain(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE DOMAIN user_email AS VARCHAR(255) CHECK (VALUE ~* '^.+@.+$');
CREATE DOMAIN positive_int AS INTEGER NOT NULL CHECK (VALUE > 0);
CREATE TABLE accounts (
    id SERIAL PRIMARY KEY,
    email user_email NOT NULL,
    score positive_int
);
`
	if err := os.WriteFile(filepath.Join(dir, "001.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := parser.ParseMigrations(parser.Options{MigrationsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Domains) != 2 {
		t.Fatalf("domains: %#v", s.Domains)
	}
	email, ok := s.DomainNamed("user_email")
	if !ok || email.BaseType != "VARCHAR(255)" || email.Check == "" {
		t.Fatalf("user_email: %+v", email)
	}
	pos, ok := s.DomainNamed("positive_int")
	if !ok || !pos.NotNull || pos.BaseType != "INTEGER" {
		t.Fatalf("positive_int: %+v", pos)
	}
	var accounts bool
	for _, table := range s.Tables {
		if table.Name != "accounts" {
			continue
		}
		accounts = true
		col, ok := table.ColumnNamed("email")
		if !ok || col.Domain != "user_email" {
			t.Fatalf("email domain: %+v", col)
		}
		score, ok := table.ColumnNamed("score")
		if !ok || score.Domain != "positive_int" {
			t.Fatalf("score domain: %+v", score)
		}
	}
	if !accounts {
		t.Fatal("accounts table missing")
	}
}

func TestParseMySQLEngineAndTemporary(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE users (
    id INT PRIMARY KEY,
    name VARCHAR(100) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TEMPORARY TABLE cart_items (
    id INT PRIMARY KEY,
    sku VARCHAR(64) NOT NULL
) ENGINE=MEMORY;
`
	if err := os.WriteFile(filepath.Join(dir, "001.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := parser.ParseMigrations(parser.Options{MigrationsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	var users, cart bool
	for _, table := range s.Tables {
		switch table.Name {
		case "users":
			users = true
			if table.Engine != "InnoDB" {
				t.Fatalf("users engine: %q", table.Engine)
			}
			if table.Temporary {
				t.Fatal("users must not be temporary")
			}
		case "cart_items":
			cart = true
			if !table.Temporary {
				t.Fatal("cart_items should be temporary")
			}
			if table.Engine != "MEMORY" {
				t.Fatalf("cart_items engine: %q", table.Engine)
			}
		}
	}
	if !users || !cart {
		t.Fatalf("missing tables users=%v cart=%v", users, cart)
	}
}

func TestParseMSSQLVector(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE documents (
    id UNIQUEIDENTIFIER NOT NULL PRIMARY KEY,
    title NVARCHAR(200) NOT NULL,
    embedding VECTOR(1536) NOT NULL
);
`
	if err := os.WriteFile(filepath.Join(dir, "001.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := parser.ParseMigrations(parser.Options{MigrationsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range s.Tables {
		if table.Name != "documents" {
			continue
		}
		col, ok := table.ColumnNamed("embedding")
		if !ok {
			t.Fatal("embedding missing")
		}
		if col.VectorDim != 1536 {
			t.Fatalf("vector dim: %d", col.VectorDim)
		}
		return
	}
	t.Fatal("documents not found")
}
