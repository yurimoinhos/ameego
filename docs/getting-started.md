---
title: Getting started
---

# Getting started

[← Back to home](index.md)

## 1. Install

```bash
go install github.com/yurimoinhos/ameego/cmd/ameego@latest
```

## 2. Configure `ameego.yaml`

ameego reads `ameego.yaml` from the current directory. CLI flags override values
from the file.

```yaml
version: "1"
dialect: postgres          # postgres | mysql | mssql | sqlite
migrations: ./migrations
output: ./generated
package: store
module: example.com/app/generated

generate:
  entities: true
  repositories: true
  query_helpers: true      # also emits the typed query builder

migrate:
  schema: public
  table: ameego_migrations
```

## 3. Write migrations

Plain `.sql` files in the migrations directory. ameego parses `CREATE TABLE`,
`ALTER TABLE ... ADD`, primary keys, foreign keys, views and PostgreSQL domains.

```sql
-- migrations/001_create_users.sql
CREATE TABLE users (
    id         BIGSERIAL PRIMARY KEY,
    email      VARCHAR(255) NOT NULL,
    name       TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## 4. Generate

```bash
ameego generate
# or with explicit config / overrides:
ameego generate --config ./ameego.yaml --dialect mssql --out ./store
```

Output layout:

```
generated/
  doc_gen.go                 # root package (go:generate)
  tables/<table>/            # one package per table or view
    entity_gen.go            #   the row struct
    repository_gen.go        #   CRUD + precompiled SQL
    typed_builder_gen.go     #   the schema+dialect-aware query builder
  functions/<name>/          # one package per DB function
  procedures/<name>/         # one package per DB procedure
```

## 5. Query

```go
import (
    "example.com/app/generated/store"
    users "example.com/app/generated/tables/users"
    "github.com/yurimoinhos/ameego/pkg/goql"
)

db, _ := goql.Open("pgx", dsn, "postgres")

u, err := users.Query().
    SelectAll().
    WhereEmailEq("ada@example.com").
    One(ctx, db)
```

See the [typed query builder](typed-builder.md) for the full method surface, and
[reverse engineering](introspection.md) to bootstrap from an existing database.
