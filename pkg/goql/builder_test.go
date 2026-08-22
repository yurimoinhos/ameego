package goql_test

import (
	"testing"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/pkg/goql"
)

func TestNormalizePlaceholders(t *testing.T) {
	sql, err := goql.NormalizePlaceholders("SELECT 1 WHERE id = $1 AND x = $2", "mssql")
	if err != nil {
		t.Fatal(err)
	}
	if sql == "" {
		t.Fatal("empty")
	}
}

func TestInsertBuilder(t *testing.T) {
	b := goql.NewBuilderFromDialect(dialect.Postgres{})
	sql, args := b.Insert("users", []string{"id", "email"}).Values("1", "a@b.c").SQL()
	if sql == "" || len(args) != 2 {
		t.Fatalf("sql=%q args=%#v", sql, args)
	}
}
