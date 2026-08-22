package dialect

import (
	"fmt"
	"strings"

	"github.com/yurimoinhos/ameego/internal/schema"
)

// Dialect abstracts database-specific SQL and type mapping.
type Dialect interface {
	Name() string
	Placeholder(n int) string
	QuoteIdent(name string) string
	MapType(sqlType string, nullable bool) string
	AutoIncrClause(column string) string
}

// ParseDialect returns a dialect by name.
func ParseDialect(name string) (Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "postgres", "postgresql", "pg":
		return Postgres{}, nil
	case "mssql", "sqlserver", "sql-server":
		return MSSQL{}, nil
	case "mysql", "mariadb":
		return MySQL{}, nil
	case "sqlite", "sqlite3":
		return SQLite{}, nil
	default:
		return nil, fmt.Errorf("unsupported dialect %q (use: postgres, mssql, mysql, sqlite)", name)
	}
}

// ApplyGoTypes maps SQL column types to Go types using the dialect.
func ApplyGoTypes(d Dialect, s *schema.Schema) {
	for ti := range s.Tables {
		for ci := range s.Tables[ti].Columns {
			col := &s.Tables[ti].Columns[ci]
			col.GoType = d.MapType(col.SQLType, col.Nullable)
		}
	}
}
