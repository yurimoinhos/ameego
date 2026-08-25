package mysql

import (
	"fmt"
	"strings"
)

// CreateTemporarySQL wraps a column definition body as CREATE TEMPORARY TABLE.
// columnsSQL is the inner CREATE TABLE body without surrounding parentheses.
func CreateTemporarySQL(table, columnsSQL string, engine string) string {
	sql := fmt.Sprintf("CREATE TEMPORARY TABLE %s (%s)", quoteIdent(table), strings.TrimSpace(columnsSQL))
	if suffix := CreateTableEngineSuffix(engine); suffix != "" {
		sql += suffix
	}
	return sql
}

// DropTemporarySQL builds DROP TEMPORARY TABLE IF EXISTS.
func DropTemporarySQL(table string) string {
	return fmt.Sprintf("DROP TEMPORARY TABLE IF EXISTS %s", quoteIdent(table))
}
