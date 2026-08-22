package goql_test

import (
	"strings"
	"testing"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/pkg/goql"
)

func TestQueryCompileNestedJoin(t *testing.T) {
	users := goql.TableOf("users")
	roles := goql.TableOf("roles")

	q := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(
			goql.ColOf(users, "id"),
			goql.ColOf(users, "email"),
			goql.ColOf(roles, "id"),
			goql.ColOf(roles, "name"),
		).
		From(users).
		Join(goql.JoinLeft, goql.ColOf(roles, "id"), goql.ColOf(users, "role_id")).
		Where(goql.ColOf(users, "active"), goql.Equals, true).
		Limit(10)

	sql, args := q.CompileForTest()
	if !strings.Contains(sql, "LEFT JOIN") {
		t.Fatalf("expected join: %s", sql)
	}
	if !strings.Contains(sql, `"roles"."id" AS "roles.id"`) {
		t.Fatalf("expected nested alias: %s", sql)
	}
	if len(args) != 1 || args[0] != true {
		t.Fatalf("args: %#v", args)
	}
}

func TestUsersRefPattern(t *testing.T) {
	users := goql.TableOf("users")
	id := goql.ColOf(users, "id")
	if id.Name() != "id" {
		t.Fatalf("got %q", id.Name())
	}
	if users.Name() != "users" {
		t.Fatalf("got %q", users.Name())
	}
}
