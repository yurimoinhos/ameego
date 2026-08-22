package dialect

import (
	"fmt"
	"strings"
)

// SQLite implements the SQLite dialect.
type SQLite struct{}

func (SQLite) Name() string { return "sqlite" }

func (SQLite) Placeholder(n int) string { return "?" }

func (SQLite) QuoteIdent(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

func (SQLite) AutoIncrClause(column string) string {
	return fmt.Sprintf(`RETURNING %s`, SQLite{}.QuoteIdent(column))
}

func (SQLite) MapType(sqlType string, nullable bool) string {
	t := normalizeSQLType(sqlType)
	goType := mapSQLiteType(t)
	return wrapNullable(goType, nullable)
}

func mapSQLiteType(t string) string {
	switch {
	case strings.Contains(t, "int"):
		return "int64"
	case strings.Contains(t, "char"), strings.Contains(t, "clob"),
		strings.Contains(t, "text"):
		return "string"
	case strings.Contains(t, "blob"):
		return "[]byte"
	case strings.Contains(t, "real"), strings.Contains(t, "floa"), strings.Contains(t, "doub"):
		return "float64"
	default:
		return "string"
	}
}
