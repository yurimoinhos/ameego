package goql_test

import (
	"strings"
	"testing"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/pkg/goql"
)

func TestMSSQLTopAndHint(t *testing.T) {
	users := goql.TableOf("users")
	q := goql.NewQueryFromDialect(dialect.MSSQL{}).
		Select(goql.ColOf(users, "id")).
		From(users).
		MSSQLTop(5).
		MSSQLTableHint("NOLOCK").
		MSSQLOption("RECOMPILE")

	sql, _ := q.CompileForTest()
	if !strings.Contains(sql, "SELECT TOP (5) ") {
		t.Fatalf("missing TOP: %s", sql)
	}
	if !strings.Contains(sql, "WITH (NOLOCK)") {
		t.Fatalf("missing hint: %s", sql)
	}
	if !strings.Contains(sql, "OPTION (RECOMPILE)") {
		t.Fatalf("missing option: %s", sql)
	}
}

func TestMSSQLTopRejectedOnPostgres(t *testing.T) {
	users := goql.TableOf("users")
	q := goql.NewQueryFromDialect(dialect.Postgres{}).
		Select(goql.ColOf(users, "id")).
		From(users)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = q.MSSQLTop(1)
}
