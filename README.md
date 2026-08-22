# Ameego

Alternativa ao [sqlc](https://sqlc.dev) para Go: codegen de entidades e repositórios tipados a partir de **migrations SQL**, usando [sqlx](https://github.com/jmoiron/sqlx) — **sem reflection em runtime**.

**Módulo:** `github.com/yurimoinhos/ameego`

Configuração via **`goql.yaml`**, queries customizadas via arquivos **`.sql`**, e SQL nativo gerado por **SQL Builder** em codegen (CRUD) e runtime (queries dinâmicas).

## Instalação

O CLI é um binário Go instalável com `go install`. O Go compila o pacote
`main` e coloca o executável em `$GOBIN` (ou `$GOPATH/bin`).

```bash
# binário ameego (recomendado)
go install github.com/yurimoinhos/ameego/cmd/ameego@latest

# alias legado — instala como goql
go install github.com/yurimoinhos/ameego/cmd/goql@latest
```

Confirme que o diretório de binários está no `PATH`:

```bash
# zsh/bash — adicione ao ~/.zshrc ou ~/.bashrc se necessário
export PATH="$(go env GOPATH)/bin:$PATH"

ameego version
ameego generate
```

Desenvolvimento local (a partir deste repositório):

```bash
make install          # go install ./cmd/ameego
make install-goql     # go install ./cmd/goql
```

Sem instalar globalmente, use `go run`:

```bash
go run github.com/yurimoinhos/ameego/cmd/ameego generate
```

Documentação da CLI (incl. release notes): [pkg.go.dev/github.com/yurimoinhos/ameego/cmd/goql](https://pkg.go.dev/github.com/yurimoinhos/ameego/cmd/goql)

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

## DSL de consulta (`Users.Ref.Id`)

Referências tipadas geradas em `refs_gen.go` — **sem struct separada**:

```go
generated.Users.Ref.Id
generated.Users.Ref.Email
generated.Roles.Ref.Name
generated.Users.Table()
```

O SQL **não** é materializado com `.SQL()`. A compilação ocorre em `SelectQuery` / `GetQuery`:

```go
var rows []generated.UserWithRole

err := users.SelectQuery(ctx, &rows,
    users.Query().
        Select(
            generated.Users.Ref.Id,
            generated.Users.Ref.Email,
            generated.Roles.Ref.Id,
            generated.Roles.Ref.Name,
        ).
        Join(goql.JoinLeft, generated.Roles.Ref.Id, generated.Users.Ref.RoleId).
        Where(generated.Users.Ref.Active, goql.Equals, true).
        OrderBy(generated.Users.Ref.CreatedAt, true).
        Paginate(1, 10),   // página 1, 10 itens — exclusivo com Offset/Limit
)
```

### Paginação

Três modos, **mutuamente exclusivos**:

```go
// Por página (1-indexed) — recomendado para APIs
q.Paginate(page, size)

// Por offset/limit — não combinar com Paginate
q.Offset(20).Limit(10)
```

No **MSSQL**, paginação exige `OrderBy`. O SQL gerado usa `OFFSET … ROWS FETCH NEXT … ROWS ONLY`.

### Subqueries

```go
inner := db.Query().
    Select(generated.Users.Ref.Id).
    From(generated.Users.Table()).
    Where(generated.Users.Ref.Active, goql.Equals, true)

rows, err := db.SelectQuery(ctx, &dest,
    db.Query().
        Select(goql.ColOf(goql.TableAs("users", "active_users"), "id")).
        FromSub(goql.AsSubquery(inner, "active_users")).
        WhereIn(generated.Users.Ref.Id, inner),
)
```

| Método | Uso |
|---|---|
| `FromSub` | `FROM (SELECT …) AS alias` — exclusivo com `From` |
| `SelectSub` | coluna escalar `(SELECT …) AS alias` |
| `WhereIn` / `WhereNotIn` | `col IN (SELECT …)` |
| `WhereExists` / `WhereNotExists` | `EXISTS (SELECT …)` |
| `TableAs(name, alias)` | refs de coluna contra alias de subquery |

### Dados aninhados

FKs nas migrations geram structs aninhados em `nested_gen.go`:

```go
type UserWithRole struct {
    Id    string `db:"id"`
    Email string `db:"email"`
    // ...
    Role  Role   `db:"roles."`  // sqlx prefix → roles.id, roles.name
}
```

Colunas da tabela joinada recebem alias `roles.id`, `roles.name`, etc., mapeados pelo prefixo `db:"roles."`.

### Comparadores e joins

```go
goql.Equals       goql.NotEquals
goql.GreaterThan  goql.LessThan
goql.Like         goql.IsNull

goql.JoinInner    goql.JoinLeft
goql.JoinRight    goql.JoinFull
```

## SQL Builder (CRUD interno)

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

### Runtime (queries dinâmicas — legado INSERT)

O builder antigo `Select().From().SQL()` foi substituído pela DSL acima. `Builder()` permanece apenas para INSERT programático se necessário.

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
cmd/goql/              CLI ameego (goql.yaml + flags)
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
