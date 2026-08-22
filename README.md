# Goql

Alternativa ao [sqlc](https://sqlc.dev) para Go: codegen de entidades e repositórios tipados a partir de **migrations SQL**, usando [sqlx](https://github.com/jmoiron/sqlx) — **sem reflection em runtime**. Tudo é resolvido em compile time via structs e métodos gerados.

Configuração via **`goql.yaml`**, queries customizadas via arquivos **`.sql`**, e SQL nativo gerado por **SQL Builder** em codegen (CRUD) e runtime (queries dinâmicas).

## Instalação

```bash
go install github.com/yurimoinhos/ameego/cmd/goql@latest
```

## Configuração (`goql.yaml`)

```yaml
version: "1"

dialect: postgres          # postgres | mssql | mysql | sqlite
migrations: ./migrations   # pasta com CREATE TABLE das migrations
output: ./generated        # saída do codegen
package: generated         # package Go gerado

generate:
  entities: true           # structs com tags db
  repositories: true       # CRUD tipado por entidade
  query_helpers: true      # helpers Exists/ByID

# queries adicionais por repositório
queries:
  - entity: User           # struct gerada (nome PascalCase)
    file: ./queries/users.sql
  - entity: Order
    file: ./queries/orders.sql
```

## Geração

```bash
# lê goql.yaml do diretório atual
goql generate

# config explícita
goql generate --config ./examples/basic/goql.yaml

# flags sobrescrevem o yaml
goql generate --config goql.yaml --dialect mssql --out ./store
```

Regenerar via `go generate`:

```bash
go generate ./generated/...
```

## Queries customizadas (`.sql`)

Cada arquivo referenciado em `goql.yaml` declara queries nomeadas:

```sql
-- name: FindByEmail :one
-- params: email
SELECT id, email, name, active, created_at
FROM users
WHERE email = $1
LIMIT 1;

-- name: ListActive :many
-- params: active
SELECT id, email, name, active, created_at
FROM users
WHERE active = $1
ORDER BY created_at DESC;

-- name: DeactivateByEmail :execrows
-- params: email
UPDATE users SET active = false WHERE email = $1;

-- name: CountByStatus :scalar
-- params: status
SELECT COUNT(*) FROM orders WHERE status = $1;
```

### Tipos de retorno

| Anotação | Método gerado |
|---|---|
| `:one` | retorna struct da entidade |
| `:many` | retorna `[]Struct` |
| `:exec` | retorna `error` |
| `:execrows` | retorna `(int64, error)` (rows affected) |
| `:scalar` | retorna `(int64, error)` (COUNT, etc.) |

Parâmetros em `-- params:` são mapeados para tipos Go via colunas da entidade (ex.: `email` → `string`, `active` → `bool`).

O SQL é normalizado para o dialect configurado (`$1` → `@p1` no MSSQL, `?` no MySQL).

## SQL Builder (alta performance)

### Codegen (CRUD)

O CRUD usa SQL nativo **pré-compilado** via builder interno — constantes geradas, zero alocação de builder no hot path:

```go
const userSelectByIDSQL = `SELECT "id", "email", ... FROM "users" WHERE "id" = $1`

func (r *UserRepository) GetByID(ctx context.Context, id string) (User, error) {
    var row User
    err := r.db.GetContext(ctx, &row, userSelectByIDSQL, id)
    return row, err
}
```

### Runtime (queries dinâmicas)

Cada repositório expõe `Builder()` para compor SQL nativo sem reflection:

```go
query, args := r.Builder().
    Select("id", "email").
    From("users").
    WhereEq("active", true).
    OrderBy("created_at", true).
    Limit(10).
    SQL()

var rows []User
err := r.db.SelectContext(ctx, &rows, query, args...)
```

## Uso

```go
import (
    _ "github.com/jackc/pgx/v5/stdlib"

    "github.com/yurimoinhos/ameego/pkg/goql"
    "your/module/generated"
)

db, _ := goql.Open("pgx", dsn, "postgres")
users := generated.NewUserRepository(db)

user, err := users.FindByEmail(ctx, "a@b.c")       // query customizada
active, err := users.ListActive(ctx, true)          // query customizada
row, err := users.GetByID(ctx, "01JABC...")         // CRUD gerado
```

## Arquivos gerados

| Arquivo | Conteúdo |
|---|---|
| `entities_gen.go` | Structs com tags `db` |
| `repositories_gen.go` | CRUD + SQL Builder + const SQL |
| `queries_gen.go` | Helpers Exists/ByID |
| `custom_queries_sql_gen.go` | Const SQL das queries customizadas |
| `custom_queries_gen.go` | Métodos tipados no repositório |
| `doc_gen.go` | `go:generate` |

## Dialectos

- `postgres` — `$1`, `"col"`
- `mssql` — `@p1`, `[col]`
- `mysql` / `sqlite` — `?`, identificadores quoted

## Arquitetura

```
cmd/goql/              CLI (goql.yaml + flags)
internal/
  config/              Loader goql.yaml
  parser/              Migrations + query SQL files
  dialect/             Tipos e placeholders por banco
  codegen/             SQL Builder usado no codegen
  generator/templates/ Go templates
pkg/goql/              Runtime: DB wrapper + SQL Builder
examples/basic/        goql.yaml + migrations + queries
```

## Licença

MIT
