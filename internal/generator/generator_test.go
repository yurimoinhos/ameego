package generator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/generator"
	"github.com/yurimoinhos/ameego/internal/parser"
)

func TestGenerate_PerTablePackages(t *testing.T) {
	migrations := filepath.Join("..", "..", "examples", "basic", "migrations")
	out := t.TempDir()

	schema, err := parser.ParseMigrations(parser.Options{MigrationsDir: migrations})
	if err != nil {
		t.Fatal(err)
	}
	d := dialect.Postgres{}
	if err := generator.Generate(generator.Config{
		Package:          "store",
		OutputDir:        out,
		Dialect:          d,
		Schema:           schema,
		WithQueries:      true,
		WithEntities:     true,
		WithRepositories: true,
	}); err != nil {
		t.Fatal(err)
	}

	userEntity := filepath.Join(out, "tables", "users", "entity_gen.go")
	data, err := os.ReadFile(userEntity)
	if err != nil {
		t.Fatalf("missing per-table entity: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "package users") {
		t.Fatalf("expected package users, got: %s", src[:min(80, len(src))])
	}
	if !strings.Contains(src, "type User struct") {
		t.Fatal("expected User struct")
	}

	userRepo := filepath.Join(out, "tables", "users", "repository_gen.go")
	repoData, err := os.ReadFile(userRepo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(repoData), "selectAllSQL") {
		t.Fatal("expected precompiled SQL constants in table package")
	}

	doc, err := os.ReadFile(filepath.Join(out, "doc_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), "package store") {
		t.Fatal("expected root doc package")
	}
}

func TestGenerate_CustomQueriesPerTable(t *testing.T) {
	migrations := filepath.Join("..", "..", "examples", "basic", "migrations")
	queriesFile := filepath.Join("..", "..", "examples", "basic", "queries", "users.sql")
	out := t.TempDir()

	schema, err := parser.ParseMigrations(parser.Options{MigrationsDir: migrations})
	if err != nil {
		t.Fatal(err)
	}
	queries, err := parser.ParseQueryFile(queriesFile)
	if err != nil {
		t.Fatal(err)
	}
	d := dialect.Postgres{}
	if err := generator.Generate(generator.Config{
		Package:          "store",
		OutputDir:        out,
		Dialect:          d,
		Schema:           schema,
		WithEntities:     true,
		WithRepositories: true,
		QueryBindings: []generator.QueryBinding{{
			Entity:  "User",
			Queries: queries,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	custom, err := os.ReadFile(filepath.Join(out, "tables", "users", "custom_queries_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(custom), "FindByEmail") {
		t.Fatal("expected FindByEmail in users package")
	}
}

func TestGenerate_NestedJoinHelpersAndViews(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE roles (
    id VARCHAR(26) PRIMARY KEY,
    name VARCHAR(100) NOT NULL
);
CREATE TABLE users (
    id VARCHAR(26) PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    role_id VARCHAR(26) REFERENCES roles(id)
);
CREATE VIEW active_users AS
SELECT id, email, role_id FROM users;
`
	if err := os.WriteFile(filepath.Join(dir, "001.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	schema, err := parser.ParseMigrations(parser.Options{MigrationsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := generator.Generate(generator.Config{
		Package:          "store",
		OutputDir:        out,
		Dialect:          dialect.Postgres{},
		Schema:           schema,
		ModulePath:       "example.com/store",
		WithEntities:     true,
		WithRepositories: true,
		Relations: []generator.RelationSpec{{
			From:       "active_users",
			FromColumn: "role_id",
			To:         "roles",
			ToColumn:   "id",
			Join:       "left",
			Field:      "Role",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	nested, err := os.ReadFile(filepath.Join(out, "tables", "users", "nested_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(nested)
	if !strings.Contains(src, "type UserWithRole struct") {
		t.Fatal("expected UserWithRole")
	}
	if !strings.Contains(src, "roles.Role") || !strings.Contains(src, `db:"roles."`) {
		t.Fatalf("expected nested Role field, got:\n%s", src)
	}
	if !strings.Contains(src, "example.com/store/tables/roles") {
		t.Fatal("expected roles import")
	}
	if !strings.Contains(src, "func QueryWithRole") {
		t.Fatal("expected QueryWithRole helper")
	}

	refs, err := os.ReadFile(filepath.Join(out, "tables", "users", "refs_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(refs), "func (r userRef) Cols()") {
		t.Fatal("expected Cols()")
	}
	if !strings.Contains(string(refs), "func (userRef) ColsAs") {
		t.Fatal("expected ColsAs")
	}

	viewEntity, err := os.ReadFile(filepath.Join(out, "tables", "active_users", "entity_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(viewEntity), "maps to view active_users") {
		t.Fatal("expected view entity comment")
	}

	viewNested, err := os.ReadFile(filepath.Join(out, "tables", "active_users", "nested_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(viewNested), "type ActiveUserWithRole struct") {
		t.Fatalf("expected ActiveUserWithRole, got:\n%s", viewNested)
	}

	viewRepo, err := os.ReadFile(filepath.Join(out, "tables", "active_users", "repository_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	repoSrc := string(viewRepo)
	if strings.Contains(repoSrc, "func (r *ActiveUserRepository) Insert") {
		t.Fatal("views must not get Insert")
	}
	if !strings.Contains(repoSrc, "func (r *ActiveUserRepository) QueryWithRole()") {
		t.Fatal("expected view QueryWithRole")
	}
}

func TestGenerate_MySQLRejectsFullRelation(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE roles (id INT PRIMARY KEY);
CREATE TABLE users (id INT PRIMARY KEY, role_id INT REFERENCES roles(id));
`
	if err := os.WriteFile(filepath.Join(dir, "001.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	schema, err := parser.ParseMigrations(parser.Options{MigrationsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	err = generator.Generate(generator.Config{
		Package:      "store",
		OutputDir:    t.TempDir(),
		Dialect:      dialect.MySQL{},
		Schema:       schema,
		WithEntities: true,
		Relations: []generator.RelationSpec{{
			From:       "users",
			FromColumn: "role_id",
			To:         "roles",
			ToColumn:   "id",
			Join:       "full",
		}},
	})
	if err == nil {
		t.Fatal("expected mysql full join error")
	}
}

func TestGenerate_PostgresDomains(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE DOMAIN user_email AS VARCHAR(255) CHECK (VALUE ~* '^.+@.+$');
CREATE TABLE accounts (
    id SERIAL PRIMARY KEY,
    email user_email NOT NULL
);
`
	if err := os.WriteFile(filepath.Join(dir, "001.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	schema, err := parser.ParseMigrations(parser.Options{MigrationsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := generator.Generate(generator.Config{
		Package:          "store",
		OutputDir:        out,
		Dialect:          dialect.Postgres{},
		Schema:           schema,
		ModulePath:       "example.com/store",
		WithEntities:     true,
		WithRepositories: true,
	}); err != nil {
		t.Fatal(err)
	}
	domains, err := os.ReadFile(filepath.Join(out, "postgres", "domains_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(domains), "type UserEmail string") {
		t.Fatalf("domains: %s", domains)
	}
	entity, err := os.ReadFile(filepath.Join(out, "tables", "accounts", "entity_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(entity)
	if !strings.Contains(src, "postgres.UserEmail") {
		t.Fatalf("expected domain type on entity: %s", src)
	}
	if !strings.Contains(src, "example.com/store/postgres") {
		t.Fatalf("expected postgres import: %s", src)
	}
}

func TestGenerate_MySQLEngineAndTemporary(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE users (
    id INT PRIMARY KEY,
    name VARCHAR(100) NOT NULL
) ENGINE=InnoDB;

CREATE TEMPORARY TABLE cart_items (
    id INT PRIMARY KEY,
    sku VARCHAR(64) NOT NULL
) ENGINE=MEMORY;
`
	if err := os.WriteFile(filepath.Join(dir, "001.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	schema, err := parser.ParseMigrations(parser.Options{MigrationsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := generator.Generate(generator.Config{
		Package:          "store",
		OutputDir:        out,
		Dialect:          dialect.MySQL{},
		Schema:           schema,
		WithEntities:     true,
		WithRepositories: true,
	}); err != nil {
		t.Fatal(err)
	}
	usersFeat, err := os.ReadFile(filepath.Join(out, "tables", "users", "mysql_features_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(usersFeat), "TableEngine =") || !strings.Contains(string(usersFeat), "InnoDB") {
		t.Fatalf("users features: %s", usersFeat)
	}
	cartFeat, err := os.ReadFile(filepath.Join(out, "tables", "cart_items", "mysql_features_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(cartFeat)
	if !strings.Contains(src, "CreateTemporary") || !strings.Contains(src, "CREATE TEMPORARY TABLE") {
		t.Fatalf("temp features: %s", src)
	}
}

func TestGenerate_MSSQLVectorAndTSQL(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE documents (
    id INT NOT NULL PRIMARY KEY,
    embedding VECTOR(8) NOT NULL
);
`
	if err := os.WriteFile(filepath.Join(dir, "001.sql"), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	schema, err := parser.ParseMigrations(parser.Options{MigrationsDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := generator.Generate(generator.Config{
		Package:          "store",
		OutputDir:        out,
		Dialect:          dialect.MSSQL{},
		Schema:           schema,
		WithEntities:     true,
		WithRepositories: true,
	}); err != nil {
		t.Fatal(err)
	}
	entity, err := os.ReadFile(filepath.Join(out, "tables", "documents", "entity_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(entity), "mssql.Vector") {
		t.Fatalf("entity: %s", entity)
	}
	feat, err := os.ReadFile(filepath.Join(out, "tables", "documents", "mssql_features_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(feat)
	if !strings.Contains(src, "QueryTop") || !strings.Contains(src, "OrderByEmbeddingDistance") {
		t.Fatalf("mssql features: %s", src)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
