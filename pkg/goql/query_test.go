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
	if !strings.Contains(sql, "LIMIT 10") {
		t.Fatalf("expected limit: %s", sql)
	}
	if len(args) != 1 || args[0] != true {
		t.Fatalf("args: %#v", args)
	}
}

func TestPaginate(t *testing.T) {
	users := goql.TableOf("users")
	q := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(users, "id")).
		From(users).
		OrderBy(goql.ColOf(users, "id"), false).
		Paginate(2, 25)

	sql, _ := q.CompileForTest()
	if !strings.Contains(sql, "LIMIT 25 OFFSET 25") {
		t.Fatalf("expected page 2 size 25: %s", sql)
	}
}

func TestPaginateExclusiveWithLimit(t *testing.T) {
	users := goql.TableOf("users")
	q := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(users, "id")).
		From(users).
		Paginate(1, 10)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic mixing Paginate and Limit")
		}
	}()
	_ = q.Limit(5)
}

func TestPaginateExclusiveWithOffset(t *testing.T) {
	users := goql.TableOf("users")
	q := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(users, "id")).
		From(users).
		Offset(5)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic mixing Offset and Paginate")
		}
	}()
	_ = q.Paginate(1, 10)
}

func TestMSSQLPaginateRequiresOrderBy(t *testing.T) {
	users := goql.TableOf("users")
	q := goql.NewQueryFromDialect(dialect.MSSQL{}).
		Select(goql.ColOf(users, "id")).
		From(users).
		Paginate(1, 10)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for MSSQL pagination without OrderBy")
		}
	}()
	q.CompileForTest()
}

func TestFromSubquery(t *testing.T) {
	users := goql.TableOf("users")
	inner := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(users, "id"), goql.ColOf(users, "email")).
		From(users).
		Where(goql.ColOf(users, "active"), goql.Equals, true)

	outer := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(goql.TableAs("users", "active_users"), "id")).
		FromSub(goql.AsSubquery(inner, "active_users"))

	sql, args := outer.CompileForTest()
	if !strings.Contains(sql, "FROM (SELECT") {
		t.Fatalf("expected subquery from: %s", sql)
	}
	if !strings.Contains(sql, `AS "active_users"`) {
		t.Fatalf("expected alias: %s", sql)
	}
	if len(args) != 1 || args[0] != true {
		t.Fatalf("args: %#v", args)
	}
}

func TestWhereInSubquery(t *testing.T) {
	users := goql.TableOf("users")
	orders := goql.TableOf("orders")

	sub := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(orders, "user_id")).
		From(orders).
		Where(goql.ColOf(orders, "status"), goql.Equals, "paid")

	q := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(users, "id")).
		From(users).
		WhereIn(goql.ColOf(users, "id"), sub)

	sql, args := q.CompileForTest()
	if !strings.Contains(sql, `IN (SELECT`) {
		t.Fatalf("expected IN subquery: %s", sql)
	}
	if len(args) != 1 || args[0] != "paid" {
		t.Fatalf("args: %#v", args)
	}
}

func TestFromExclusiveWithFromSub(t *testing.T) {
	users := goql.TableOf("users")
	inner := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(users, "id")).
		From(users)

	q := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(users, "id")).
		From(users)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic mixing From and FromSub")
		}
	}()
	_ = q.FromSub(goql.AsSubquery(inner, "u"))
}

func TestQueryCompileNestedJoinAllDialects(t *testing.T) {
	users := goql.TableOf("users")
	roles := goql.TableOf("roles")
	cases := []struct {
		d     dialect.Dialect
		join  string
		alias string
	}{
		{dialect.Postgres{}, `LEFT JOIN "roles" AS "roles"`, `"roles"."id" AS "roles.id"`},
		{dialect.MSSQL{}, `LEFT JOIN [roles] AS [roles]`, `[roles].[id] AS [roles.id]`},
		{dialect.MySQL{}, "LEFT JOIN `roles` AS `roles`", "`roles`.`id` AS `roles.id`"},
		{dialect.SQLite{}, `LEFT JOIN "roles" AS "roles"`, `"roles"."id" AS "roles.id"`},
	}
	for _, tc := range cases {
		q := goql.NewQueryFromDialect(tc.d).
			Select(goql.ColOf(users, "id"), goql.ColOf(roles, "id")).
			From(users).
			Join(goql.JoinLeft, goql.ColOf(roles, "id"), goql.ColOf(users, "role_id"))
		sql, _ := q.CompileForTest()
		if !strings.Contains(sql, tc.join) {
			t.Fatalf("%s missing join %q in %s", tc.d.Name(), tc.join, sql)
		}
		if !strings.Contains(sql, tc.alias) {
			t.Fatalf("%s missing nested alias %q in %s", tc.d.Name(), tc.alias, sql)
		}
	}
}

func TestMySQLRejectsFullJoin(t *testing.T) {
	users := goql.TableOf("users")
	roles := goql.TableOf("roles")
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for MySQL FULL JOIN")
		}
	}()
	_ = goql.NewQueryFromDialect(dialect.MySQL{}).
		Select(goql.ColOf(users, "id")).
		From(users).
		Join(goql.JoinFull, goql.ColOf(roles, "id"), goql.ColOf(users, "role_id"))
}

func TestJoinSelfJoinUsesAlias(t *testing.T) {
	users := goql.TableOf("users")
	manager := goql.TableAs("users", "manager")
	q := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(users, "id"), goql.ColOf(manager, "id")).
		From(users).
		Join(goql.JoinLeft, goql.ColOf(manager, "id"), goql.ColOf(users, "manager_id"))
	sql, _ := q.CompileForTest()
	if !strings.Contains(sql, `LEFT JOIN "users" AS "manager"`) {
		t.Fatalf("expected self-join alias: %s", sql)
	}
	if !strings.Contains(sql, `"manager"."id" AS "manager.id"`) {
		t.Fatalf("expected nested manager alias: %s", sql)
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
