// Package goql is the Ameego runtime: a compile-time typed database access layer on sqlx.
// Code generation is provided by github.com/yurimoinhos/ameego/cmd/goql.
//
// Query building is deferred — SQL is compiled when SelectQuery or GetQuery runs.
// Use Paginate for page-based paging, or Offset/Limit (never both styles on one query).
// Subqueries are supported via FromSub, SelectSub, and WhereIn/WhereExists helpers.
package goql
