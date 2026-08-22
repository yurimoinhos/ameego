package parser_test

import (
	"testing"

	"github.com/yurimoinhos/ameego/internal/parser"
)

func TestParseQuerySQL(t *testing.T) {
	sql := `-- name: FindByEmail :one
-- params: email
SELECT id, email FROM users WHERE email = $1;

-- name: ListActive :many
-- params: active
SELECT id FROM users WHERE active = $1;
`
	queries, err := parser.ParseQuerySQL(sql, "test.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(queries))
	}
	if queries[0].Name != "FindByEmail" || queries[0].ResultKind != parser.QueryOne {
		t.Fatalf("first query: %#v", queries[0])
	}
	if len(queries[0].ParamNames) != 1 || queries[0].ParamNames[0] != "email" {
		t.Fatalf("params: %#v", queries[0].ParamNames)
	}
}

func TestResolveParamTypes(t *testing.T) {
	types := parser.ResolveParamTypes([]string{"email", "active"}, map[string]string{
		"email":  "string",
		"active": "bool",
	})
	if types[0] != "string" || types[1] != "bool" {
		t.Fatalf("got %#v", types)
	}
}
