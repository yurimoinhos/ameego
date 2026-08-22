package dialect

import (
	"fmt"
	"strings"
)

// MSSQL implements the Microsoft SQL Server dialect.
type MSSQL struct{}

func (MSSQL) Name() string { return "mssql" }

func (MSSQL) Placeholder(n int) string { return fmt.Sprintf("@p%d", n) }

func (MSSQL) QuoteIdent(name string) string { return "[" + strings.ReplaceAll(name, "]", "]]") + "]" }

func (MSSQL) AutoIncrClause(column string) string {
	return fmt.Sprintf(`OUTPUT INSERTED.%s`, MSSQL{}.QuoteIdent(column))
}

func (MSSQL) MapType(sqlType string, nullable bool) string {
	t := normalizeSQLType(sqlType)
	goType := mapMSSQLType(t)
	return wrapNullable(goType, nullable)
}

func mapMSSQLType(t string) string {
	switch {
	case strings.HasPrefix(t, "nvarchar"), strings.HasPrefix(t, "varchar"),
		strings.HasPrefix(t, "nchar"), strings.HasPrefix(t, "char"),
		strings.HasPrefix(t, "text"), strings.HasPrefix(t, "ntext"),
		t == "uniqueidentifier", strings.HasPrefix(t, "xml"):
		return "string"
	case t == "bit":
		return "bool"
	case t == "tinyint":
		return "uint8"
	case t == "smallint":
		return "int16"
	case t == "int":
		return "int32"
	case t == "bigint":
		return "int64"
	case t == "real":
		return "float32"
	case t == "float":
		return "float64"
	case strings.HasPrefix(t, "decimal"), strings.HasPrefix(t, "numeric"), strings.HasPrefix(t, "money"):
		return "string"
	case strings.HasPrefix(t, "datetime"), strings.HasPrefix(t, "datetime2"),
		strings.HasPrefix(t, "smalldatetime"), strings.HasPrefix(t, "date"),
		strings.HasPrefix(t, "time"), strings.HasPrefix(t, "datetimeoffset"):
		return "time.Time"
	case t == "varbinary", t == "binary", t == "image":
		return "[]byte"
	default:
		return "string"
	}
}
