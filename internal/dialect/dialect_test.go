package dialect_test

import (
	"testing"

	"github.com/yurimoinhos/ameego/internal/dialect"
)

func TestMapType_Postgres(t *testing.T) {
	d := dialect.Postgres{}
	cases := map[string]string{
		"VARCHAR(26)":  "string",
		"BOOLEAN":      "bool",
		"BIGINT":       "int64",
		"TIMESTAMP":    "time.Time",
		"NUMERIC(12,2)": "string",
		"TEXT":         "string",
	}
	for sql, want := range cases {
		if got := d.MapType(sql, false); got != want {
			t.Fatalf("%s: got %q want %q", sql, got, want)
		}
	}
	if got := d.MapType("TEXT", true); got != "string" {
		t.Fatalf("nullable text: got %q", got)
	}
	if got := d.MapType("INT", true); got != "sql.NullInt32" {
		t.Fatalf("nullable int: got %q", got)
	}
}

func TestMapType_MSSQL(t *testing.T) {
	d := dialect.MSSQL{}
	if got := d.MapType("UNIQUEIDENTIFIER", false); got != "string" {
		t.Fatalf("got %q", got)
	}
	if got := d.MapType("BIT", false); got != "bool" {
		t.Fatalf("got %q", got)
	}
	if got := d.Placeholder(3); got != "@p3" {
		t.Fatalf("placeholder: got %q", got)
	}
}

func TestParseDialect(t *testing.T) {
	for _, name := range []string{"postgres", "mssql", "mysql", "sqlite"} {
		if _, err := dialect.ParseDialect(name); err != nil {
			t.Fatalf("dialect %s: %v", name, err)
		}
	}
}
