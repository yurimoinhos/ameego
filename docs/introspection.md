---
title: Reverse engineering
---

# Reverse engineering an existing database

[← Back to home](index.md)

`ameego introspect` reads a live database's catalog
(`information_schema` / `pg_catalog` / `sys.*` / SQLite `PRAGMA`) and reconstructs
the schema — tables, columns, primary keys, foreign keys, and PostgreSQL domains.
Supported dialects: **PostgreSQL, MySQL/MariaDB, SQL Server, SQLite**.

The reverse-engineered schema is the **same internal representation** the
migration parser produces, so it feeds the exact same codegen pipeline — the
structs you get from an existing database are identical to the ones you would get
from migrations.

## Default: write a migration file

By default, `introspect` renders the schema back into a dialect-faithful
`CREATE TABLE` migration. This keeps `ameego generate` as the single source of
truth for codegen:

```bash
ameego introspect --dsn "$DATABASE_URL"
# → writes migrations/000_introspected.sql
# → then: review it and run `ameego generate`
```

Tables are emitted in foreign-key dependency order, so referenced tables precede
their referrers.

## Generate directly

Skip the migration file and generate Go packages in one step:

```bash
ameego introspect --dsn "$DATABASE_URL" --generate
```

## Options

| Flag          | Meaning                                                              |
| ------------- | -------------------------------------------------------------------- |
| `--dsn`       | Connection string (defaults to `$DATABASE_URL`).                     |
| `--dialect`   | Override the dialect (otherwise taken from `ameego.yaml`).           |
| `--schema`    | Namespace to read — Postgres schema, SQL Server schema, or MySQL database. Defaults: `public` / `dbo` / active database. |
| `--views`     | Also reverse-engineer views as read-only entities.                   |
| `--driver`    | `database/sql` driver name, when the dialect default is not imported. |
| `--generate`  | Generate Go packages directly instead of writing a `.sql` file.      |
| `--gen-out`   | Override the output directory when using `--generate`.               |
| `--out`       | Override the output `.sql` path (default `<migrations>/000_introspected.sql`). |

## Drivers

The connection uses a `database/sql` driver that must be registered in your
binary. ameego infers a sensible default per dialect, but if that driver is not
imported you will see a connection error — pass `--driver` with a driver you
have imported, for example:

```bash
ameego introspect --dsn "$DATABASE_URL" --driver pgx
```

## What is captured

- **Tables & views** (views only with `--views`).
- **Columns** — SQL type, nullability, default, and auto-increment / identity.
- **Primary keys** — single and composite.
- **Foreign keys** — turned into join relations, exactly like FK parsing from
  migrations.
- **PostgreSQL domains** — read from `information_schema.domains`.

Views are reconstructed as read-only entities and are not emitted as
`CREATE TABLE` in the default migration output (they are not reproducible that
way); use `--generate` if you want builders for them.
