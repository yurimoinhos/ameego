// Command ameego-codegen generates typed database access code from SQL migrations.
//
// # Overview
//
// Ameego is a compile-time SQL ORM for Go built on [sqlx] — no reflection at runtime.
// The CLI reads ameego.yaml (or goql.yaml for backward compatibility) and emits
// entities, repositories, and typed query helpers.
//
// Install:
//
//	go install github.com/yurimoinhos/ameego/cmd/ameego@latest
//
// Legacy alias (binary name goql):
//
//	go install github.com/yurimoinhos/ameego/cmd/goql@latest
//
// Generate from the project root (where goql.yaml lives):
//
//	ameego generate
//
// Or without a global install:
//
//	go run github.com/yurimoinhos/ameego/cmd/ameego generate
//
// # Configuration
//
// By default the CLI reads goql.yaml from the current directory.
// Flags override values from the config file.
//
// # Runtime query DSL
//
// Generated code uses github.com/yurimoinhos/ameego/pkg/goql for deferred SELECT
// compilation. Pagination and subqueries are enforced at build time:
//
//   - Paginate(page, size) — page-based (1-indexed); mutually exclusive with Offset/Limit
//   - Offset(n) / Limit(n) — row skip/take; cannot combine with Paginate
//   - FromSub — subquery as FROM source; mutually exclusive with From
//   - WhereIn / WhereNotIn / WhereExists / WhereNotExists — correlated subqueries
//   - SelectSub — scalar subquery in the SELECT list
//
// # Release notes
//
// ## v0.5.0
//
//   - Dialect-native features via codegen (no shared feature interface).
//   - PostgreSQL: CREATE DOMAIN → generated/postgres domain types + column mapping.
//   - MySQL: ENGINE=… and CREATE TEMPORARY TABLE → mysql_features_gen.go helpers.
//   - MSSQL: T-SQL QueryTop / QueryWithHint / OPTION, VECTOR columns → mssql.Vector.
//   - Runtime helpers in pkg/goql/postgres, pkg/goql/mysql, pkg/goql/mssql.
//   - Examples: examples/postgres, examples/mysql, examples/mssql.
//
// ## v0.3.0
//
//   - Paginate(page, size int) for page-based pagination (1-indexed page).
//   - Offset and Limit remain available but cannot be mixed with Paginate.
//   - Subqueries: FromSub, SelectSub, WhereIn, WhereNotIn, WhereExists, WhereNotExists.
//   - TableAs(name, alias) for column refs against subquery FROM aliases.
//   - Dialect-native pagination (LIMIT/OFFSET vs MSSQL OFFSET/FETCH).
//   - MSSQL pagination requires OrderBy.
//   - CLI installable via go install github.com/yurimoinhos/ameego/cmd/ameego@latest
//   - Legacy cmd/goql path kept (installs goql binary).
//
// ## v0.2.0
//
//   - Typed column refs (Entity.Ref.Column) and deferred query compilation.
//   - Nested join mapping with sqlx db struct prefixes.
//   - Table and view relationships (migration FKs + ameego.yaml relations).
//   - Generated QueryWith* helpers and Cols()/ColsAs() for typed joins.
//   - Custom SQL queries from .sql files with dialect placeholder normalization.
//   - MSSQL, PostgreSQL, MySQL, and SQLite dialect support.
//
// ## v0.1.0
//
//   - Initial codegen from SQL migrations: entities, repositories, query helpers.
//
// [sqlx]: https://github.com/jmoiron/sqlx
package main
