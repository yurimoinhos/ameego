// Package introspect reverse-engineers a live database into an ameego schema.
//
// Each dialect implements Introspector by reading the engine's catalog
// (information_schema / pg_catalog / sys.*) and returns the same
// *schema.Schema the migration parser produces. That schema then feeds the
// exact same codegen pipeline, so generated structs are identical whether the
// source is a migrations directory or an existing database.
package introspect

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/schema"
)

// Options configures a reverse-engineering run.
type Options struct {
	// Schema is the catalog/namespace to read (postgres/mssql: schema name,
	// mysql: database name). Empty uses the dialect default.
	Schema string
	// IncludeViews also reverse-engineers views as read-only entities.
	IncludeViews bool
}

// Introspector reads a live database's structure into a *schema.Schema.
type Introspector interface {
	// Introspect queries the connection's catalog and builds a schema.
	Introspect(ctx context.Context, db *sql.DB, opts Options) (*schema.Schema, error)
}

// For returns the Introspector for a dialect, or an error if reverse
// engineering is not supported for that engine yet.
func For(d dialect.Dialect) (Introspector, error) {
	switch d.Name() {
	case "postgres":
		return &postgresIntrospector{d: d}, nil
	case "mysql":
		return &mysqlIntrospector{d: d}, nil
	case "mssql":
		return &mssqlIntrospector{d: d}, nil
	case "sqlite":
		return &sqliteIntrospector{d: d}, nil
	default:
		return nil, fmt.Errorf("introspect: dialect %q is not supported", d.Name())
	}
}

// defaultSchema returns the catalog default namespace per dialect.
func defaultSchema(dialectName, configured string) string {
	if strings.TrimSpace(configured) != "" {
		return configured
	}
	switch dialectName {
	case "postgres":
		return "public"
	case "mssql":
		return "dbo"
	default:
		return ""
	}
}

// finalize sorts/annotates the schema so it matches migration-parser output:
// FK-derived relations and domain column annotation.
func finalize(s *schema.Schema) {
	s.Relations = relationsFromFKs(s)
	annotateDomainColumns(s)
}

func relationsFromFKs(s *schema.Schema) []schema.Relation {
	var out []schema.Relation
	for _, table := range s.Tables {
		for _, col := range table.Columns {
			if col.RefTable == "" || col.RefColumn == "" {
				continue
			}
			joinKind := "inner"
			if col.Nullable {
				joinKind = "left"
			}
			out = append(out, schema.Relation{
				FromTable:  table.Name,
				FromColumn: col.Name,
				ToTable:    col.RefTable,
				ToColumn:   col.RefColumn,
				JoinKind:   joinKind,
			})
		}
	}
	return out
}

func annotateDomainColumns(s *schema.Schema) {
	if len(s.Domains) == 0 {
		return
	}
	for ti := range s.Tables {
		for ci := range s.Tables[ti].Columns {
			col := &s.Tables[ti].Columns[ci]
			if d, ok := s.DomainNamed(col.SQLType); ok {
				col.Domain = d.Name
			}
		}
	}
}

// upsertColumnRef sets a FK target on a named column.
func upsertColumnRef(t *schema.Table, column, refTable, refColumn string) {
	for i := range t.Columns {
		if strings.EqualFold(t.Columns[i].Name, column) {
			t.Columns[i].RefTable = refTable
			t.Columns[i].RefColumn = refColumn
			return
		}
	}
}
