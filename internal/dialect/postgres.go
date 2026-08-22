package dialect

import (
	"fmt"
	"strings"
)

// Postgres implements the PostgreSQL dialect.
type Postgres struct{}

func (Postgres) Name() string { return "postgres" }

func (Postgres) Placeholder(n int) string { return fmt.Sprintf("$%d", n) }

func (Postgres) QuoteIdent(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

func (Postgres) AutoIncrClause(column string) string {
	return fmt.Sprintf(`RETURNING %s`, Postgres{}.QuoteIdent(column))
}

func (Postgres) MapType(sqlType string, nullable bool) string {
	t := normalizeSQLType(sqlType)
	goType := mapPostgresType(t)
	return wrapNullable(goType, nullable)
}

func mapPostgresType(t string) string {
	switch {
	case strings.HasPrefix(t, "varchar"), strings.HasPrefix(t, "char"),
		strings.HasPrefix(t, "text"), strings.HasPrefix(t, "uuid"),
		strings.HasPrefix(t, "json"), strings.HasPrefix(t, "jsonb"),
		t == "name", t == "bpchar":
		return "string"
	case t == "bool", t == "boolean":
		return "bool"
	case strings.HasPrefix(t, "int2"), t == "smallint":
		return "int16"
	case strings.HasPrefix(t, "int4"), t == "integer", t == "int":
		return "int32"
	case strings.HasPrefix(t, "int8"), t == "bigint", t == "bigserial":
		return "int64"
	case t == "serial":
		return "int32"
	case t == "smallserial":
		return "int16"
	case strings.HasPrefix(t, "float4"), t == "real":
		return "float32"
	case strings.HasPrefix(t, "float8"), t == "double precision":
		return "float64"
	case strings.HasPrefix(t, "numeric"), strings.HasPrefix(t, "decimal"), strings.HasPrefix(t, "money"):
		return "string" // use shopspring/decimal at call site if needed; default string for precision
	case strings.HasPrefix(t, "timestamp"), strings.HasPrefix(t, "timestamptz"),
		t == "date", strings.HasPrefix(t, "time"):
		return "time.Time"
	case t == "bytea":
		return "[]byte"
	default:
		return "string"
	}
}
