package goql_test

import (
	"testing"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/pkg/goql"
)

func TestBuilderSelect(t *testing.T) {
	d := dialect.Postgres{}
	b := goql.NewBuilderFromDialect(d)
	query, args := b.Select("id", "email").From("users").WhereEq("email", "a@b.c").SQL()
	if query == "" {
		t.Fatal("empty query")
	}
	if len(args) != 1 {
		t.Fatalf("args: %#v", args)
	}
}

func TestNormalizePlaceholders(t *testing.T) {
	sql, err := goql.NormalizePlaceholders("SELECT 1 WHERE id = $1 AND x = $2", "mssql")
	if err != nil {
		t.Fatal(err)
	}
	if sql == "" {
		t.Fatal("empty")
	}
}
