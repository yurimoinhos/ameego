package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/parser"
	"github.com/yurimoinhos/ameego/internal/schema"
)

func TestOrdersPrimaryKeyNotNullable(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "basic", "migrations")
	schema, err := parser.ParseMigrations(parser.Options{MigrationsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	dialect.ApplyGoTypes(dialect.Postgres{}, schema)

	for _, table := range schema.Tables {
		if table.Name != "orders" {
			continue
		}
		for _, col := range table.Columns {
			if col.Name == "id" {
				if col.Nullable {
					t.Fatalf("orders.id should not be nullable")
				}
				if col.GoType != "int64" {
					t.Fatalf("orders.id go type: got %q want int64", col.GoType)
				}
				if !col.IsPrimary {
					t.Fatal("orders.id should be primary")
				}
				return
			}
		}
	}
	t.Fatal("orders table or id column not found")
}

func TestParseTableLevelForeignKey(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE authors (
    id INT PRIMARY KEY,
    name TEXT NOT NULL
);
CREATE TABLE books (
    id INT PRIMARY KEY,
    author_id INT NOT NULL,
    title TEXT NOT NULL,
    CONSTRAINT fk_books_author FOREIGN KEY (author_id) REFERENCES authors(id)
);
ALTER TABLE books ADD CONSTRAINT fk_books_author2 FOREIGN KEY (author_id) REFERENCES authors(id);
`
	if err := os.WriteFile(filepath.Join(dir, "001.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := parser.ParseMigrations(parser.Options{MigrationsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	var books schema.Table
	for _, table := range s.Tables {
		if table.Name == "books" {
			books = table
		}
	}
	if books.Name == "" {
		t.Fatal("books table not found")
	}
	col, ok := books.ColumnNamed("author_id")
	if !ok {
		t.Fatal("author_id not found")
	}
	if col.RefTable != "authors" || col.RefColumn != "id" {
		t.Fatalf("FK: got %s(%s)", col.RefTable, col.RefColumn)
	}
	if len(s.Relations) == 0 {
		t.Fatal("expected schema relations from FK")
	}
}

func TestParseViewColumnsAndTypes(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE users (
    id VARCHAR(26) PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL
);
CREATE VIEW active_users AS
SELECT id, email, created_at
FROM users
WHERE active = true;
CREATE VIEW all_users AS
SELECT * FROM users;
`
	if err := os.WriteFile(filepath.Join(dir, "001.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := parser.ParseMigrations(parser.Options{MigrationsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	dialect.ApplyGoTypes(dialect.Postgres{}, s)

	var active, all schema.Table
	for _, table := range s.Tables {
		switch table.Name {
		case "active_users":
			active = table
		case "all_users":
			all = table
		}
	}
	if !active.IsView() {
		t.Fatalf("active_users kind: %q", active.Kind)
	}
	if len(active.Columns) != 3 {
		t.Fatalf("active_users columns: %#v", active.Columns)
	}
	email, ok := active.ColumnNamed("email")
	if !ok || email.GoType != "string" {
		t.Fatalf("email type: %+v", email)
	}
	if !all.IsView() || len(all.Columns) != 4 {
		t.Fatalf("all_users: kind=%s cols=%d", all.Kind, len(all.Columns))
	}
}
