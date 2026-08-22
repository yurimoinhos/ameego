package parser_test

import (
	"path/filepath"
	"testing"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/parser"
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
