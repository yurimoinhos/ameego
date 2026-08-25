// Package postgres exposes PostgreSQL-native helpers used by Ameego codegen.
// These APIs are concrete — there is no cross-dialect feature interface.
package postgres

import (
	"fmt"
	"strings"
)

// Domain describes a PostgreSQL DOMAIN type for documentation and SQL rebuild.
type Domain struct {
	Name     string
	BaseType string
	NotNull  bool
	Default  string
	Check    string
	Collate  string
}

// CreateDomainSQL builds a CREATE DOMAIN statement.
func CreateDomainSQL(d Domain) string {
	var b strings.Builder
	b.WriteString("CREATE DOMAIN ")
	b.WriteString(quoteIdent(d.Name))
	b.WriteString(" AS ")
	b.WriteString(d.BaseType)
	if d.Collate != "" {
		b.WriteString(" COLLATE ")
		b.WriteString(quoteIdent(d.Collate))
	}
	if d.Default != "" {
		b.WriteString(" DEFAULT ")
		b.WriteString(d.Default)
	}
	if d.NotNull {
		b.WriteString(" NOT NULL")
	}
	if d.Check != "" {
		b.WriteString(" CHECK (")
		b.WriteString(d.Check)
		b.WriteString(")")
	}
	return b.String()
}

// DropDomainSQL builds DROP DOMAIN IF EXISTS name.
func DropDomainSQL(name string) string {
	return fmt.Sprintf("DROP DOMAIN IF EXISTS %s", quoteIdent(name))
}

func quoteIdent(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return quoteIdent(name[:i]) + "." + quoteIdent(name[i+1:])
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
