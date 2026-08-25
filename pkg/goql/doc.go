// Package goql is the Ameego runtime: a compile-time typed database access layer on sqlx.
// Code generation is provided by github.com/yurimoinhos/ameego/cmd/ameego.
//
// Query building is deferred — SQL is compiled when SelectQuery or GetQuery runs.
// Use Paginate for page-based paging, or Offset/Limit (never both styles on one query).
// Subqueries are supported via FromSub, SelectSub, and WhereIn/WhereExists helpers.
//
// Dialect-native helpers live in concrete subpackages (no shared feature interface):
//   - github.com/yurimoinhos/ameego/pkg/goql/postgres — DOMAIN SQL helpers
//   - github.com/yurimoinhos/ameego/pkg/goql/mysql — ENGINE / TEMPORARY helpers
//   - github.com/yurimoinhos/ameego/pkg/goql/mssql — T-SQL TOP / VECTOR
package goql
