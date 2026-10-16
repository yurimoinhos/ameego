---
title: ameego
---

# ameego

**Compile-time type-safe SQL codegen for Go.** ameego generates one Go package
per database table from your migrations — or straight from a live database — and
emits a schema-aware, dialect-aware query builder where an invalid column, a
wrong value type, or a feature your database does not support is a **compile
error**, not a runtime panic. Built on [sqlx](https://github.com/jmoiron/sqlx)
with **no reflection** at runtime.

Supported dialects: **PostgreSQL · MySQL/MariaDB · SQL Server · SQLite**.

## Why ameego

- **Errors at compile time.** Each generated table builder exposes a method only
  for columns that actually exist, typed by that column's Go type. Reference a
  column that is not there, or pass the wrong type, and your code does not build.
- **Simple to granular.** Write trivial lookups or deeply composed queries with
  the same fluent, typed API, falling through to the raw `goql` DSL when you need
  full control.
- **Dialect-faithful codegen.** Structs and builders are generated from the
  driver's dialect — SQL Server-only helpers (`Top`, `TableHint`, `Option`) are
  generated *only* for the mssql dialect, so misuse cannot compile.
- **Reverse engineering.** Point ameego at an existing database and it reads the
  catalog back into tables, columns, keys and relations.

## Documentation

- [Getting started](getting-started.md)
- [Typed query builder](typed-builder.md)
- [Reverse engineering an existing database](introspection.md)

## Install

```bash
go install github.com/yurimoinhos/ameego/cmd/ameego@latest
```

## At a glance

```go
import users "example.com/app/generated/tables/users"

rows, err := users.Query().
    SelectAll().
    WhereAgeGte(18).           // typed by the column's Go type
    WhereNameLike("A%").
    WhereStatusIn("active", "trial").
    OrderByCreatedAtDesc().
    Limit(20).
    All(ctx, db)
```

---

<small>ameego is MIT-licensed. Source on
[GitHub](https://github.com/yurimoinhos/ameego).</small>
