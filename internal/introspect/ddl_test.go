package introspect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/parser"
	"github.com/yurimoinhos/ameego/internal/schema"
)

// TestEmitDDLRoundTrip verifies the reverse-engineering loop closes: a schema
// rendered to DDL and re-parsed by the migration parser reproduces the same
// tables, columns, primary keys, and foreign-key relations. This is the core
// guarantee that `ameego introspect` output feeds `ameego generate` cleanly.
func TestEmitDDLRoundTrip(t *testing.T) {
	src := &schema.Schema{
		Tables: []schema.Table{
			{
				Name: "users",
				Kind: schema.KindTable,
				Columns: []schema.Column{
					{Name: "id", SQLType: "bigint", Nullable: false, IsPrimary: true},
					{Name: "email", SQLType: "varchar(255)", Nullable: false},
					{Name: "name", SQLType: "text", Nullable: true},
				},
				PrimaryKey: []string{"id"},
			},
			{
				Name: "posts",
				Kind: schema.KindTable,
				Columns: []schema.Column{
					{Name: "id", SQLType: "bigint", Nullable: false, IsPrimary: true},
					{Name: "user_id", SQLType: "bigint", Nullable: false, RefTable: "users", RefColumn: "id"},
					{Name: "title", SQLType: "varchar(200)", Nullable: false},
				},
				PrimaryKey: []string{"id"},
			},
		},
	}
	finalize(src)

	d := dialect.Postgres{}
	ddl := EmitDDL(d, src)

	// FK dependency ordering: users must precede posts.
	if idxUsers, idxPosts := indexOf(ddl, "CREATE TABLE \"users\""), indexOf(ddl, "CREATE TABLE \"posts\""); idxUsers < 0 || idxPosts < 0 || idxUsers > idxPosts {
		t.Fatalf("expected users before posts in DDL, got:\n%s", ddl)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "000_introspected.sql"), []byte(ddl), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := parser.ParseMigrations(parser.Options{MigrationsDir: dir})
	if err != nil {
		t.Fatalf("re-parse emitted DDL: %v", err)
	}

	if len(got.Tables) != 2 {
		t.Fatalf("want 2 tables, got %d: %+v", len(got.Tables), got.Tables)
	}
	byName := map[string]schema.Table{}
	for _, tb := range got.Tables {
		byName[tb.Name] = tb
	}

	users, ok := byName["users"]
	if !ok {
		t.Fatal("users table missing after round-trip")
	}
	if len(users.PrimaryKey) != 1 || users.PrimaryKey[0] != "id" {
		t.Fatalf("users PK not preserved: %v", users.PrimaryKey)
	}
	if len(users.Columns) != 3 {
		t.Fatalf("users want 3 columns, got %d", len(users.Columns))
	}
	if name, ok := users.ColumnNamed("name"); !ok || !name.Nullable {
		t.Fatalf("users.name nullability lost: %+v", name)
	}
	if email, ok := users.ColumnNamed("email"); !ok || email.Nullable {
		t.Fatalf("users.email should be NOT NULL: %+v", email)
	}

	posts := byName["posts"]
	userID, ok := posts.ColumnNamed("user_id")
	if !ok {
		t.Fatal("posts.user_id missing")
	}
	if userID.RefTable != "users" || userID.RefColumn != "id" {
		t.Fatalf("posts.user_id FK lost: ref=%s.%s", userID.RefTable, userID.RefColumn)
	}

	// FK-derived relation present.
	foundRel := false
	for _, r := range got.Relations {
		if r.FromTable == "posts" && r.FromColumn == "user_id" && r.ToTable == "users" && r.ToColumn == "id" {
			foundRel = true
		}
	}
	if !foundRel {
		t.Fatalf("expected posts->users relation, got %+v", got.Relations)
	}
}

func TestEmitDDLDialectQuoting(t *testing.T) {
	s := &schema.Schema{
		Tables: []schema.Table{{
			Name:       "t",
			Kind:       schema.KindTable,
			Columns:    []schema.Column{{Name: "id", SQLType: "int", IsPrimary: true}},
			PrimaryKey: []string{"id"},
		}},
	}
	cases := map[dialect.Dialect]string{
		dialect.Postgres{}: "\"t\"",
		dialect.MySQL{}:    "`t`",
		dialect.MSSQL{}:    "[t]",
	}
	for d, want := range cases {
		ddl := EmitDDL(d, s)
		if indexOf(ddl, "CREATE TABLE "+want) < 0 {
			t.Errorf("dialect %s: want quoted ident %s in:\n%s", d.Name(), want, ddl)
		}
	}
}

func indexOf(haystack, needle string) int {
	return stringIndex(haystack, needle)
}

func stringIndex(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
