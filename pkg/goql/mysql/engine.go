// Package mysql exposes MySQL/MariaDB-native helpers used by Ameego codegen.
// These APIs are concrete — there is no cross-dialect feature interface.
package mysql

import (
	"fmt"
	"strings"
)

// Common storage engines.
const (
	EngineInnoDB  = "InnoDB"
	EngineMyISAM  = "MyISAM"
	EngineMemory  = "MEMORY"
	EngineCSV     = "CSV"
	EngineArchive = "ARCHIVE"
)

// AlterEngineSQL builds ALTER TABLE … ENGINE=….
func AlterEngineSQL(table, engine string) string {
	return fmt.Sprintf("ALTER TABLE %s ENGINE=%s", quoteIdent(table), engine)
}

// CreateTableEngineSuffix returns the ENGINE= clause for CREATE TABLE.
func CreateTableEngineSuffix(engine string) string {
	engine = strings.TrimSpace(engine)
	if engine == "" {
		return ""
	}
	return " ENGINE=" + engine
}

func quoteIdent(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return quoteIdent(name[:i]) + "." + quoteIdent(name[i+1:])
	}
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
