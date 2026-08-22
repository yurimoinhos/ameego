package migrate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yurimoinhos/ameego/internal/migrate"
)

func TestLoadMigrationsSorted(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "010_second.sql", "SELECT 2;")
	writeFile(t, dir, "002_first.sql", "SELECT 1;")
	writeFile(t, dir, "100_third.sql", "SELECT 3;")

	ms, err := migrate.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(ms))
	}
	if ms[0].Version != "002" || ms[2].Version != "100" {
		t.Fatalf("order: %s, %s, %s", ms[0].Version, ms[1].Version, ms[2].Version)
	}
	if ms[0].Checksum == "" {
		t.Fatal("expected checksum")
	}
}

func TestSplitStatementsDollarQuote(t *testing.T) {
	sql := `CREATE FUNCTION fn() RETURNS void AS $$
BEGIN
  SELECT 1;
END;
$$ LANGUAGE plpgsql;
CREATE TABLE t (id INT);`

	stmts := migrate.SplitStatements(sql)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d: %#v", len(stmts), stmts)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
