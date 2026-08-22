package parser_test

import (
	"strings"
	"testing"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/parser"
)

func TestParseRoutineFunction(t *testing.T) {
	sql := `-- name: CountActiveUsers :function
-- dbname: count_active_users
-- returns: int64
-- params: active
CREATE OR REPLACE FUNCTION count_active_users(p_active BOOLEAN)
RETURNS BIGINT AS $$ SELECT COUNT(*) FROM users WHERE active = p_active $$ LANGUAGE sql;`

	routines, err := parser.ParseRoutineSQL(sql, "routines.sql", dialect.Postgres{})
	if err != nil {
		t.Fatal(err)
	}
	if len(routines) != 1 {
		t.Fatalf("expected 1 routine, got %d", len(routines))
	}
	r := routines[0]
	if r.QualifiedName != "count_active_users" {
		t.Fatalf("dbname: %q", r.QualifiedName)
	}
	if !strings.HasPrefix(r.InvokeSQL, "SELECT") {
		t.Fatalf("invoke should delegate to DB: %q", r.InvokeSQL)
	}
	if !strings.Contains(r.InvokeSQL, "count_active_users") {
		t.Fatalf("invoke missing function name: %q", r.InvokeSQL)
	}
}

func TestParseRoutineProcedure(t *testing.T) {
	sql := `-- name: DeactivateUser :procedure
-- dbname: deactivate_user
-- params: email
CREATE OR REPLACE PROCEDURE deactivate_user(p_email VARCHAR(255))
LANGUAGE SQL AS $$ UPDATE users SET active = false WHERE email = p_email $$;`

	routines, err := parser.ParseRoutineSQL(sql, "routines.sql", dialect.Postgres{})
	if err != nil {
		t.Fatal(err)
	}
	if len(routines) != 1 {
		t.Fatal("expected 1 routine")
	}
	if !strings.HasPrefix(routines[0].InvokeSQL, "CALL") {
		t.Fatalf("procedure invoke: %q", routines[0].InvokeSQL)
	}
}

func TestParseRoutineTrigger(t *testing.T) {
	sql := `-- name: UsersUpdatedAt :trigger
CREATE TRIGGER users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();`

	routines, err := parser.ParseRoutineSQL(sql, "routines.sql", dialect.Postgres{})
	if err != nil {
		t.Fatal(err)
	}
	if len(routines) != 1 {
		t.Fatal("expected 1 trigger")
	}
	if routines[0].InvokeSQL != "" {
		t.Fatal("triggers should not have invoke SQL")
	}
}
