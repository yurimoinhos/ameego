package goql_test

import (
	"strings"
	"testing"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/pkg/goql"
)

func TestWhereInValues(t *testing.T) {
	users := goql.TableOf("users")
	q := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(users, "id")).
		From(users).
		WhereInValues(goql.ColOf(users, "id"), []any{1, 2, 3})

	sql, args := q.CompileForTest()
	if !strings.Contains(sql, `"users"."id" IN ($1, $2, $3)`) {
		t.Fatalf("expected IN with 3 placeholders: %s", sql)
	}
	if len(args) != 3 || args[0] != 1 || args[2] != 3 {
		t.Fatalf("args: %#v", args)
	}
}

func TestWhereNotInValuesPlaceholderNumbering(t *testing.T) {
	users := goql.TableOf("users")
	q := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(users, "id")).
		From(users).
		Where(goql.ColOf(users, "active"), goql.Equals, true).
		WhereNotInValues(goql.ColOf(users, "id"), []any{10, 20})

	sql, args := q.CompileForTest()
	// First predicate consumes $1, the NOT IN list continues at $2, $3.
	if !strings.Contains(sql, `"users"."active" = $1`) {
		t.Fatalf("expected active = $1: %s", sql)
	}
	if !strings.Contains(sql, `"users"."id" NOT IN ($2, $3)`) {
		t.Fatalf("expected NOT IN ($2, $3): %s", sql)
	}
	if len(args) != 3 {
		t.Fatalf("want 3 args, got %#v", args)
	}
}

func TestWhereInValuesEmpty(t *testing.T) {
	users := goql.TableOf("users")

	in := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(users, "id")).From(users).
		WhereInValues(goql.ColOf(users, "id"), nil)
	sql, args := in.CompileForTest()
	if !strings.Contains(sql, "1 = 0") {
		t.Fatalf("empty IN must never match: %s", sql)
	}
	if len(args) != 0 {
		t.Fatalf("empty IN should bind no args: %#v", args)
	}

	notIn := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(users, "id")).From(users).
		WhereNotInValues(goql.ColOf(users, "id"), nil)
	sql2, _ := notIn.CompileForTest()
	if !strings.Contains(sql2, "1 = 1") {
		t.Fatalf("empty NOT IN must always match: %s", sql2)
	}
}
