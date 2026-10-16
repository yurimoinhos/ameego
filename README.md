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

# joins extra (obrigatório para views; opcional para sobrescrever FKs)
relations:
  - from: active_users
    from_column: role_id
    to: roles
    to_column: id
    join: left
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

## Engenharia reversa (`introspect`)

Traga um banco já existente para o ameego — tabelas, colunas, chaves primárias e
estrangeiras — lendo o catálogo do próprio engine (`information_schema` /
`pg_catalog` / `sys.*` / `PRAGMA`). Suporta postgres, mysql, mssql e sqlite.

```bash
# escreve migrations/000_introspected.sql (fonte da verdade continua em migrations)
ameego introspect --dsn "$DATABASE_URL"

# gera os pacotes Go direto, sem passar por migration
ameego introspect --dsn "$DATABASE_URL" --generate

# namespace específico e incluindo views como entidades read-only
ameego introspect --dsn "$DATABASE_URL" --schema myschema --views
```

Passe `--driver` quando o driver padrão do dialeto não estiver importado no seu
binário. O fluxo padrão escreve um `.sql` dialeto-fiel que, reaberto por
`ameego generate`, reproduz exatamente o mesmo schema — então `generate`
continua sendo o único ponto de geração.

## Builder tipado por schema+dialeto

Para cada tabela, o `generate` emite um `<Entidade>Query` cujos métodos só
existem porque a coluna (ou o recurso do dialeto) existe no schema de origem.
Usar uma coluna inexistente, um tipo errado, ou um recurso que o dialeto não tem
é **erro de compilação** — não panic em runtime.

```go
import users "example.com/app/generated/tables/users"

// Só colunas reais de users viram métodos; o valor é tipado pela coluna.
rows, err := users.Query().
    SelectAll().
    WhereIdEq(42).          // WhereId<Coluna>Eq(int64) — tipo errado não compila
    OrderByCreatedAt(true).
    Limit(10).
    All(ctx, db)
```

Métodos específicos de dialeto são gerados condicionalmente — por exemplo
`Top(n)`, `TableHint("NOLOCK")` e `Option("RECOMPILE")` existem apenas nos
pacotes gerados para o dialeto **mssql**. Chamá-los contra um schema gerado para
postgres/mysql/sqlite simplesmente não compila.

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

### Dados aninhados (tabelas e views)

FKs nas migrations (inline `REFERENCES`, `FOREIGN KEY` no `CREATE TABLE`, ou `ALTER TABLE … ADD FOREIGN KEY`) geram structs aninhados em `nested_gen.go`. Views não têm FK — declare o join em `ameego.yaml`:

```yaml
relations:
  - from: active_users    # tabela, view, ou nome da entidade (ActiveUser)
    from_column: role_id
    to: roles
    to_column: id
    join: left            # inner | left | right | full (full não existe no MySQL)
    field: Role           # nome do campo aninhado (opcional)
```

```go
type UserWithRole struct {
    Id    string     `db:"id"`
    Email string     `db:"email"`
    // ...
    Role  roles.Role `db:"roles."`  // sqlx prefix → roles.id, roles.name
}
```

O helper gerado monta o `SELECT` + `JOIN` com as colunas das structs:

```go
var rows []users.UserWithRole
err := usersRepo.SelectQuery(ctx, &rows, usersRepo.QueryWithRole())

// join explícito (inner/left/right; full só postgres/mssql/sqlite)
err = usersRepo.SelectQuery(ctx, &rows,
    usersRepo.QueryWithRoleJoin(goql.JoinLeft).
        Where(users.Users.Ref.Active, goql.Equals, true),
)

// join manual com as colunas geradas
err = usersRepo.SelectQuery(ctx, &rows,
    usersRepo.Query().
        Select(users.Users.Cols()...).
        Join(goql.JoinLeft, roles.Roles.Ref.Id, users.Users.Ref.RoleId).
        Select(roles.Roles.ColsAs("roles")...),
)
```

Colunas da relação joinada recebem alias `roles.id`, `roles.name`, etc., mapeados pelo prefixo `db:"roles."`. O quoting (`"…"`, `[…]`, `` `…` ``) segue o dialect (postgres, mssql, mysql, sqlite).

Views geram a mesma DSL (`ActiveUsers.Ref`, `Cols()`, `QueryWithRole`) mas **sem** Insert/Update/Delete.

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
| `tables/<name>/entity_gen.go` | Structs com tags `db` |
| `tables/<name>/repository_gen.go` | CRUD + SQL pré-compilado |
| `tables/<name>/queries_gen.go` | Helpers Exists/ByID |
| `tables/<name>/custom_queries_*.go` | Queries tipadas a partir de `.sql` |
| `postgres/domains_gen.go` | Tipos DOMAIN (só `dialect: postgres`) |
| `tables/<name>/mysql_features_gen.go` | ENGINE / TEMPORARY (só MySQL) |
| `tables/<name>/mssql_features_gen.go` | T-SQL TOP / VECTOR (só MSSQL) |
| `doc_gen.go` | `go:generate` |

## Features nativas por dialecto (sem abstração)

Cada banco gera **APIs concretas** do próprio dialecto — não há interface comum de “features”.

### PostgreSQL — `DOMAIN`

```sql
CREATE DOMAIN user_email AS VARCHAR(255) CHECK (VALUE ~* '^.+@.+$');
CREATE TABLE accounts (email user_email NOT NULL);
```

```go
// generated/postgres/domains_gen.go
type UserEmail string

// tables/accounts
type Account struct {
    Email postgres.UserEmail `db:"email"`
}
```

### MySQL — ENGINE e tabelas temporárias

```sql
CREATE TABLE products (...) ENGINE=InnoDB;
CREATE TEMPORARY TABLE cart_items (...) ENGINE=MEMORY;
```

```go
products.Engine()                 // "InnoDB"
products.EnsureEngine(ctx)

cartItems.CreateTemporary(ctx)  // CREATE TEMPORARY TABLE …
cartItems.DropTemporary(ctx)
```

### MSSQL — T-SQL e `VECTOR`

```sql
CREATE TABLE products (
    id UNIQUEIDENTIFIER PRIMARY KEY,
    embedding VECTOR(8) NULL
);
```

```go
type Product struct {
    Embedding *mssql.Vector `db:"embedding"`
}

products.QueryTop(10) // SELECT TOP (10) …
products.QueryWithHint("NOLOCK")
products.OrderByEmbeddingDistance(q, embedding, mssql.DistanceCosine)
```

Exemplos: `examples/postgres`, `examples/mysql`, `examples/mssql`.

## Dialectos

- `postgres` — `$1`, `"col"`, DOMAIN types
- `mssql` — `@p1`, `[col]`, T-SQL TOP / hints / VECTOR
- `mysql` — `?`, `` `col` ``, ENGINE / TEMPORARY
- `sqlite` — `?`, `"col"`

## Arquitetura

```
cmd/ameego/            CLI (ameego.yaml + flags)
internal/
  config/              Loader ameego.yaml
  parser/              Migrations + query SQL + DOMAIN/ENGINE/VECTOR
  dialect/             Placeholders e MapType por banco
  codegen/             SQL Builder usado no codegen
  generator/templates/ Go templates (incl. features nativas)
pkg/goql/              Runtime DSL
pkg/goql/postgres|mysql|mssql/  Helpers concretos por banco
examples/{basic,postgres,mysql,mssql}/
```

## Licença

MIT
