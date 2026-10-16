package introspect

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/schema"
)

type postgresIntrospector struct{ d dialect.Dialect }

func (p *postgresIntrospector) Introspect(ctx context.Context, db *sql.DB, opts Options) (*schema.Schema, error) {
	ns := defaultSchema("postgres", opts.Schema)

	s := &schema.Schema{}
	tables, order, err := p.readTables(ctx, db, ns, opts.IncludeViews)
	if err != nil {
		return nil, err
	}
	if err := p.readColumns(ctx, db, ns, tables); err != nil {
		return nil, err
	}
	if err := p.readPrimaryKeys(ctx, db, ns, tables); err != nil {
		return nil, err
	}
	if err := p.readForeignKeys(ctx, db, ns, tables); err != nil {
		return nil, err
	}
	domains, err := p.readDomains(ctx, db, ns)
	if err != nil {
		return nil, err
	}
	s.Domains = domains
	for _, name := range order {
		s.Tables = append(s.Tables, *tables[name])
	}
	finalize(s)
	return s, nil
}

func (p *postgresIntrospector) readTables(ctx context.Context, db *sql.DB, ns string, includeViews bool) (map[string]*schema.Table, []string, error) {
	q := `SELECT table_name, table_type
	        FROM information_schema.tables
	       WHERE table_schema = $1
	         AND table_type IN ('BASE TABLE'` + viewsClausePG(includeViews) + `)
	    ORDER BY table_name`
	rows, err := db.QueryContext(ctx, q, ns)
	if err != nil {
		return nil, nil, fmt.Errorf("introspect postgres tables: %w", err)
	}
	defer rows.Close()

	tables := map[string]*schema.Table{}
	var order []string
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return nil, nil, err
		}
		kind := schema.KindTable
		if typ == "VIEW" {
			kind = schema.KindView
		}
		tables[name] = &schema.Table{Name: name, Kind: kind}
		order = append(order, name)
	}
	return tables, order, rows.Err()
}

func viewsClausePG(includeViews bool) string {
	if includeViews {
		return `, 'VIEW'`
	}
	return ``
}

func (p *postgresIntrospector) readColumns(ctx context.Context, db *sql.DB, ns string, tables map[string]*schema.Table) error {
	q := `SELECT table_name, column_name, data_type, udt_name, is_nullable,
	             COALESCE(column_default, ''), domain_name
	        FROM information_schema.columns
	       WHERE table_schema = $1
	    ORDER BY table_name, ordinal_position`
	rows, err := db.QueryContext(ctx, q, ns)
	if err != nil {
		return fmt.Errorf("introspect postgres columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, colName, dataType, udtName, isNullable, colDefault string
		var domainName sql.NullString
		if err := rows.Scan(&tableName, &colName, &dataType, &udtName, &isNullable, &colDefault, &domainName); err != nil {
			return err
		}
		t, ok := tables[tableName]
		if !ok {
			continue
		}
		sqlType := pgResolveType(dataType, udtName, domainName)
		t.Columns = append(t.Columns, schema.Column{
			Name:       colName,
			SQLType:    sqlType,
			Nullable:   isNullable == "YES",
			Default:    colDefault,
			IsAutoIncr: isPgSerial(colDefault, udtName),
			Domain:     domainName.String,
		})
	}
	return rows.Err()
}

// pgResolveType prefers a user-defined domain name, otherwise the SQL type.
func pgResolveType(dataType, udtName string, domain sql.NullString) string {
	if domain.Valid && domain.String != "" {
		return domain.String
	}
	if dataType == "USER-DEFINED" || dataType == "ARRAY" {
		return udtName
	}
	return dataType
}

func isPgSerial(def, udt string) bool {
	return def != "" && (len(def) >= 7 && def[:7] == "nextval")
}

func (p *postgresIntrospector) readPrimaryKeys(ctx context.Context, db *sql.DB, ns string, tables map[string]*schema.Table) error {
	q := `SELECT kcu.table_name, kcu.column_name
	        FROM information_schema.table_constraints tc
	        JOIN information_schema.key_column_usage kcu
	          ON tc.constraint_name = kcu.constraint_name
	         AND tc.table_schema = kcu.table_schema
	       WHERE tc.constraint_type = 'PRIMARY KEY'
	         AND tc.table_schema = $1
	    ORDER BY kcu.table_name, kcu.ordinal_position`
	rows, err := db.QueryContext(ctx, q, ns)
	if err != nil {
		return fmt.Errorf("introspect postgres primary keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, colName string
		if err := rows.Scan(&tableName, &colName); err != nil {
			return err
		}
		t, ok := tables[tableName]
		if !ok {
			continue
		}
		t.PrimaryKey = append(t.PrimaryKey, colName)
		for i := range t.Columns {
			if t.Columns[i].Name == colName {
				t.Columns[i].IsPrimary = true
				t.Columns[i].Nullable = false
			}
		}
	}
	return rows.Err()
}

func (p *postgresIntrospector) readForeignKeys(ctx context.Context, db *sql.DB, ns string, tables map[string]*schema.Table) error {
	q := `SELECT kcu.table_name, kcu.column_name,
	             ccu.table_name AS ref_table, ccu.column_name AS ref_column
	        FROM information_schema.table_constraints tc
	        JOIN information_schema.key_column_usage kcu
	          ON tc.constraint_name = kcu.constraint_name
	         AND tc.table_schema = kcu.table_schema
	        JOIN information_schema.constraint_column_usage ccu
	          ON ccu.constraint_name = tc.constraint_name
	         AND ccu.table_schema = tc.table_schema
	       WHERE tc.constraint_type = 'FOREIGN KEY'
	         AND tc.table_schema = $1`
	rows, err := db.QueryContext(ctx, q, ns)
	if err != nil {
		return fmt.Errorf("introspect postgres foreign keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, colName, refTable, refColumn string
		if err := rows.Scan(&tableName, &colName, &refTable, &refColumn); err != nil {
			return err
		}
		if t, ok := tables[tableName]; ok {
			upsertColumnRef(t, colName, refTable, refColumn)
		}
	}
	return rows.Err()
}

func (p *postgresIntrospector) readDomains(ctx context.Context, db *sql.DB, ns string) ([]schema.Domain, error) {
	q := `SELECT domain_name, data_type, COALESCE(domain_default, ''),
	             CASE WHEN is_nullable = 'NO' THEN true ELSE false END
	        FROM information_schema.domains
	       WHERE domain_schema = $1
	    ORDER BY domain_name`
	rows, err := db.QueryContext(ctx, q, ns)
	if err != nil {
		return nil, fmt.Errorf("introspect postgres domains: %w", err)
	}
	defer rows.Close()

	var out []schema.Domain
	for rows.Next() {
		var name, baseType, def string
		var notNull bool
		if err := rows.Scan(&name, &baseType, &def, &notNull); err != nil {
			return nil, err
		}
		out = append(out, schema.Domain{
			Name:     name,
			BaseType: baseType,
			Default:  def,
			NotNull:  notNull,
		})
	}
	return out, rows.Err()
}
