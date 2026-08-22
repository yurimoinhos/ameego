package dialect_test

import (
	"strings"
	"testing"

	"github.com/yurimoinhos/ameego/internal/dialect"
)

func TestPaginationClausePostgres(t *testing.T) {
	got := dialect.PaginationClause(dialect.Postgres{}, 10, 20)
	if got != "LIMIT 10 OFFSET 20" {
		t.Fatalf("got %q", got)
	}
}

func TestPaginationClauseMSSQL(t *testing.T) {
	got := dialect.PaginationClause(dialect.MSSQL{}, 10, 20)
	if !strings.Contains(got, "OFFSET 20 ROWS FETCH NEXT 10 ROWS ONLY") {
		t.Fatalf("got %q", got)
	}
}
