package dialect

import "fmt"

// MigrationTableDDL returns CREATE TABLE for the migration version tracker.
func MigrationTableDDL(d Dialect, schema, table string) string {
	qTable := d.QuoteIdent(table)
	fqn := qualifiedTable(d, schema, table)

	switch d.Name() {
	case "mssql":
		return fmt.Sprintf(`IF NOT EXISTS (SELECT 1 FROM sys.tables t JOIN sys.schemas s ON t.schema_id = s.schema_id WHERE s.name = N'%s' AND t.name = N'%s')
BEGIN
    EXEC(N'CREATE TABLE %s (
        version VARCHAR(255) NOT NULL PRIMARY KEY,
        applied_at DATETIME2 NOT NULL DEFAULT SYSUTCDATETIME(),
        checksum VARCHAR(64) NOT NULL
    )')
END`, schema, table, fqn)
	case "mysql":
		return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    version VARCHAR(255) NOT NULL PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(64) NOT NULL
) ENGINE=InnoDB`, fqn)
	case "sqlite":
		return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    version TEXT NOT NULL PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now')),
    checksum TEXT NOT NULL
)`, qTable)
	default: // postgres
		return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    version VARCHAR(255) NOT NULL PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    checksum VARCHAR(64) NOT NULL
)`, fqn)
	}
}

// EnsureSchemaDDL returns SQL to create the migration schema when supported.
func EnsureSchemaDDL(d Dialect, schema string) string {
	if schema == "" {
		return ""
	}
	switch d.Name() {
	case "postgres":
		return fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", d.QuoteIdent(schema))
	case "mssql":
		return fmt.Sprintf(`IF NOT EXISTS (SELECT 1 FROM sys.schemas WHERE name = N'%s') EXEC(N'CREATE SCHEMA %s')`, schema, d.QuoteIdent(schema))
	default:
		return ""
	}
}

// MigrationTableName returns the fully qualified migration table name.
func MigrationTableName(d Dialect, schema, table string) string {
	return qualifiedTable(d, schema, table)
}

func qualifiedTable(d Dialect, schema, table string) string {
	qTable := d.QuoteIdent(table)
	if schema == "" || d.Name() == "sqlite" {
		return qTable
	}
	return d.QuoteIdent(schema) + "." + qTable
}
