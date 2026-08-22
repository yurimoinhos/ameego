package dialect

import (
	"fmt"
	"strings"
)

// MySQL implements the MySQL/MariaDB dialect.
type MySQL struct{}

func (MySQL) Name() string { return "mysql" }

func (MySQL) Placeholder(n int) string { return "?" }

func (MySQL) QuoteIdent(name string) string { return "`" + strings.ReplaceAll(name, "`", "``") + "`" }

func (MySQL) AutoIncrClause(column string) string {
	return fmt.Sprintf(`/* last_insert_id for %s */`, column)
}

func (MySQL) MapType(sqlType string, nullable bool) string {
	t := normalizeSQLType(sqlType)
	goType := mapMySQLType(t)
	return wrapNullable(goType, nullable)
}

func mapMySQLType(t string) string {
	switch {
	case strings.HasPrefix(t, "varchar"), strings.HasPrefix(t, "char"),
		strings.HasPrefix(t, "text"), strings.HasPrefix(t, "enum"),
		strings.HasPrefix(t, "set"), t == "json":
		return "string"
	case t == "bool", t == "boolean":
		return "bool"
	case t == "tinyint":
		return "int8"
	case t == "smallint":
		return "int16"
	case t == "mediumint", t == "int":
		return "int32"
	case t == "bigint":
		return "int64"
	case t == "float":
		return "float32"
	case t == "double", t == "real":
		return "float64"
	case strings.HasPrefix(t, "decimal"), strings.HasPrefix(t, "numeric"):
		return "string"
	case strings.HasPrefix(t, "datetime"), strings.HasPrefix(t, "timestamp"),
		t == "date", strings.HasPrefix(t, "time"):
		return "time.Time"
	case strings.HasPrefix(t, "blob"), strings.HasPrefix(t, "binary"):
		return "[]byte"
	default:
		return "string"
	}
}
