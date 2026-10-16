---
title: Typed query builder
---

# Typed query builder

[← Back to home](index.md)

For every table, `ameego generate` emits a `<Entity>Query` builder in that
table's package (`typed_builder_gen.go`). Every method on it exists **only
because** the column — or the dialect feature — it maps to exists in the source
schema for the generated dialect. The practical effect:

- a method for a column that does not exist is **not generated**, so referencing
  it does not compile;
- each predicate is **typed by the column's Go type**, so passing the wrong type
  does not compile;
- SQL Server-only operators are generated **only** for the mssql dialect, so
  calling them against a Postgres/MySQL/SQLite schema does not compile.

This is the "A2" approach: safety comes from the *absence* of a method, which is
the strongest guarantee Go gives you — a true compile error, not a runtime panic.

## Starting a query

```go
import users "example.com/app/generated/tables/users"

q := users.Query()        // bound to the dialect this package was generated for
```

## Selecting

```go
users.Query().SelectAll()                       // every column
users.Query().Select(users.Users.Ref.Id,        // specific typed columns
                     users.Users.Ref.Email)
```

## Per-column predicates

For a column `created_at` of Go type `time.Time`, ameego generates a typed
method set. The value parameter is always the column's own Go type, so a mismatch
is a compile error:

| Method                         | SQL                              |
| ------------------------------ | -------------------------------- |
| `WhereCreatedAtEq(v)`          | `created_at = ?`                 |
| `WhereCreatedAtNe(v)`          | `created_at <> ?`                |
| `WhereCreatedAtGt(v)`          | `created_at > ?`                 |
| `WhereCreatedAtGte(v)`         | `created_at >= ?`                |
| `WhereCreatedAtLt(v)`          | `created_at < ?`                 |
| `WhereCreatedAtLte(v)`         | `created_at <= ?`                |
| `WhereCreatedAtLike(p)`        | `created_at LIKE ?`              |
| `WhereCreatedAtIn(v...)`       | `created_at IN (?, ?, …)`        |
| `WhereCreatedAtNotIn(v...)`    | `created_at NOT IN (?, ?, …)`    |
| `WhereCreatedAtIsNull()`       | `created_at IS NULL`             |
| `WhereCreatedAtIsNotNull()`    | `created_at IS NOT NULL`         |
| `OrderByCreatedAt(desc bool)`  | `ORDER BY created_at [DESC]`     |

```go
rows, err := users.Query().
    SelectAll().
    WhereAgeGte(18).
    WhereNameLike("A%").
    WhereStatusIn("active", "trial").   // variadic, each typed as the column
    OrderByCreatedAt(true).
    Limit(20).
    All(ctx, db)
```

### `IN` with a value list

`WhereXIn` / `WhereXNotIn` are variadic over the column's Go type and expand to a
placeholder list. An **empty** list is handled safely rather than silently
matching everything:

- `WhereXIn()` with no values compiles to `1 = 0` (matches no rows);
- `WhereXNotIn()` with no values compiles to `1 = 1` (matches all rows).

## Pagination

```go
users.Query().SelectAll().Limit(10).Offset(20)
users.Query().SelectAll().Paginate(3, 25)   // 1-indexed page, page size
```

## Dialect-gated methods (SQL Server only)

These are generated **only** in packages produced for the `mssql` dialect.
Against any other dialect the methods do not exist, so the code does not compile:

```go
products.Query().
    Top(100).                 // SELECT TOP (100)
    TableHint("NOLOCK").       // WITH (NOLOCK)
    SelectAll().
    Option("RECOMPILE")        // OPTION (RECOMPILE)
```

## Escape hatch

When you need something beyond the generated surface, fall through to the raw
`goql` DSL — the builder is a thin, typed layer over it:

```go
q := users.Query().SelectAll()
q.Where(users.Users.Ref.Email, goql.Like, "%@example.com")
query := q.Build()            // *goql.Query for composition / subqueries
```

## Executing

```go
list, err := users.Query().SelectAll().All(ctx, db)   // []User
one,  err := users.Query().WhereIdEq(42).One(ctx, db)  // User
raw      := users.Query().SelectAll().Build()          // *goql.Query
```
