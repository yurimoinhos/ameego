package introspect

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/schema"
)

// ---------------------------------------------------------------------------
// MySQL / MariaDB
// ---------------------------------------------------------------------------

type mysqlIntrospector struct{ d dialect.Dialect }

func (m *mysqlIntrospector) Introspect(ctx context.Context, db *sql.DB, opts Options) (*schema.Schema, error) {
	// MySQL "schema" == database. Empty resolves to the active DATABASE().
	dbName := strings.TrimSpace(opts.Schema)
	if dbName == "" {
		if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&dbName); err != nil {
			return nil, fmt.Errorf("introspect mysql: resolve database: %w", err)
		}
		if dbName == "" {
			return nil, fmt.Errorf("introspect mysql: no database selected; pass --schema <database>")
		}
	}

	tables := map[string]*schema.Table{}
	var order []string

	typeFilter := "'BASE TABLE'"
	if opts.IncludeViews {
		typeFilter = "'BASE TABLE','VIEW'"
	}
	rows, err := db.QueryContext(ctx, `SELECT table_name, table_type
		FROM information_schema.tables
		WHERE table_schema = ? AND table_type IN (`+typeFilter+`)
		ORDER BY table_name`, dbName)
	if err != nil {
		return nil, fmt.Errorf("introspect mysql tables: %w", err)
	}
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			rows.Close()
			return nil, err
		}
		kind := schema.KindTable
		if typ == "VIEW" {
			kind = schema.KindView
		}
		tables[name] = &schema.Table{Name: name, Kind: kind}
		order = append(order, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	colRows, err := db.QueryContext(ctx, `SELECT table_name, column_name, column_type, is_nullable,
			COALESCE(column_default, ''), column_key, extra
		FROM information_schema.columns
		WHERE table_schema = ?
		ORDER BY table_name, ordinal_position`, dbName)
	if err != nil {
		return nil, fmt.Errorf("introspect mysql columns: %w", err)
	}
	for colRows.Next() {
		var tbl, col, colType, nullable, def, key, extra string
		if err := colRows.Scan(&tbl, &col, &colType, &nullable, &def, &key, &extra); err != nil {
			colRows.Close()
			return nil, err
		}
		t, ok := tables[tbl]
		if !ok {
			continue
		}
		c := schema.Column{
			Name:       col,
			SQLType:    colType,
			Nullable:   nullable == "YES",
			Default:    def,
			IsPrimary:  key == "PRI",
			IsAutoIncr: strings.Contains(strings.ToLower(extra), "auto_increment"),
		}
		if c.IsPrimary {
			c.Nullable = false
		}
		t.Columns = append(t.Columns, c)
		if c.IsPrimary {
			t.PrimaryKey = append(t.PrimaryKey, col)
		}
	}
	colRows.Close()
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	fkRows, err := db.QueryContext(ctx, `SELECT table_name, column_name, referenced_table_name, referenced_column_name
		FROM information_schema.key_column_usage
		WHERE table_schema = ? AND referenced_table_name IS NOT NULL`, dbName)
	if err != nil {
		return nil, fmt.Errorf("introspect mysql foreign keys: %w", err)
	}
	for fkRows.Next() {
		var tbl, col, refTbl, refCol string
		if err := fkRows.Scan(&tbl, &col, &refTbl, &refCol); err != nil {
			fkRows.Close()
			return nil, err
		}
		if t, ok := tables[tbl]; ok {
			upsertColumnRef(t, col, refTbl, refCol)
		}
	}
	fkRows.Close()
	if err := fkRows.Err(); err != nil {
		return nil, err
	}

	s := &schema.Schema{}
	for _, name := range order {
		s.Tables = append(s.Tables, *tables[name])
	}
	finalize(s)
	return s, nil
}

// ---------------------------------------------------------------------------
// Microsoft SQL Server
// ---------------------------------------------------------------------------

type mssqlIntrospector struct{ d dialect.Dialect }

func (m *mssqlIntrospector) Introspect(ctx context.Context, db *sql.DB, opts Options) (*schema.Schema, error) {
	ns := defaultSchema("mssql", opts.Schema)

	tables := map[string]*schema.Table{}
	var order []string

	typeFilter := "'BASE TABLE'"
	if opts.IncludeViews {
		typeFilter = "'BASE TABLE','VIEW'"
	}
	rows, err := db.QueryContext(ctx, `SELECT table_name, table_type
		FROM information_schema.tables
		WHERE table_schema = @p1 AND table_type IN (`+typeFilter+`)
		ORDER BY table_name`, ns)
	if err != nil {
		return nil, fmt.Errorf("introspect mssql tables: %w", err)
	}
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			rows.Close()
			return nil, err
		}
		kind := schema.KindTable
		if typ == "VIEW" {
			kind = schema.KindView
		}
		tables[name] = &schema.Table{Name: name, Kind: kind}
		order = append(order, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	colRows, err := db.QueryContext(ctx, `SELECT c.table_name, c.column_name, c.data_type,
			c.character_maximum_length, c.is_nullable, COALESCE(c.column_default, ''),
			COLUMNPROPERTY(OBJECT_ID(c.table_schema + '.' + c.table_name), c.column_name, 'IsIdentity')
		FROM information_schema.columns c
		WHERE c.table_schema = @p1
		ORDER BY c.table_name, c.ordinal_position`, ns)
	if err != nil {
		return nil, fmt.Errorf("introspect mssql columns: %w", err)
	}
	for colRows.Next() {
		var tbl, col, dataType, nullable, def string
		var maxLen sql.NullInt64
		var isIdentity sql.NullInt64
		if err := colRows.Scan(&tbl, &col, &dataType, &maxLen, &nullable, &def, &isIdentity); err != nil {
			colRows.Close()
			return nil, err
		}
		t, ok := tables[tbl]
		if !ok {
			continue
		}
		sqlType := dataType
		if maxLen.Valid && maxLen.Int64 > 0 && mssqlTypeTakesLength(dataType) {
			sqlType = fmt.Sprintf("%s(%d)", dataType, maxLen.Int64)
		}
		t.Columns = append(t.Columns, schema.Column{
			Name:       col,
			SQLType:    sqlType,
			Nullable:   nullable == "YES",
			Default:    def,
			IsAutoIncr: isIdentity.Valid && isIdentity.Int64 == 1,
		})
	}
	colRows.Close()
	if err := colRows.Err(); err != nil {
		return nil, err
	}

	pkRows, err := db.QueryContext(ctx, `SELECT kcu.table_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY' AND tc.table_schema = @p1
		ORDER BY kcu.table_name, kcu.ordinal_position`, ns)
	if err != nil {
		return nil, fmt.Errorf("introspect mssql primary keys: %w", err)
	}
	for pkRows.Next() {
		var tbl, col string
		if err := pkRows.Scan(&tbl, &col); err != nil {
			pkRows.Close()
			return nil, err
		}
		if t, ok := tables[tbl]; ok {
			t.PrimaryKey = append(t.PrimaryKey, col)
			for i := range t.Columns {
				if t.Columns[i].Name == col {
					t.Columns[i].IsPrimary = true
					t.Columns[i].Nullable = false
				}
			}
		}
	}
	pkRows.Close()
	if err := pkRows.Err(); err != nil {
		return nil, err
	}

	fkRows, err := db.QueryContext(ctx, `SELECT kcu.table_name, kcu.column_name,
			ccu.table_name, ccu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = @p1`, ns)
	if err != nil {
		return nil, fmt.Errorf("introspect mssql foreign keys: %w", err)
	}
	for fkRows.Next() {
		var tbl, col, refTbl, refCol string
		if err := fkRows.Scan(&tbl, &col, &refTbl, &refCol); err != nil {
			fkRows.Close()
			return nil, err
		}
		if t, ok := tables[tbl]; ok {
			upsertColumnRef(t, col, refTbl, refCol)
		}
	}
	fkRows.Close()
	if err := fkRows.Err(); err != nil {
		return nil, err
	}

	s := &schema.Schema{}
	for _, name := range order {
		s.Tables = append(s.Tables, *tables[name])
	}
	finalize(s)
	return s, nil
}

func mssqlTypeTakesLength(dataType string) bool {
	switch strings.ToLower(dataType) {
	case "varchar", "nvarchar", "char", "nchar", "varbinary", "binary":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// SQLite
// ---------------------------------------------------------------------------

type sqliteIntrospector struct{ d dialect.Dialect }

func (m *sqliteIntrospector) Introspect(ctx context.Context, db *sql.DB, opts Options) (*schema.Schema, error) {
	typeFilter := "type = 'table'"
	if opts.IncludeViews {
		typeFilter = "type IN ('table','view')"
	}
	rows, err := db.QueryContext(ctx, `SELECT name, type FROM sqlite_master
		WHERE `+typeFilter+` AND name NOT LIKE 'sqlite_%'
		ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("introspect sqlite tables: %w", err)
	}
	type tbl struct {
		name string
		kind string
	}
	var list []tbl
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			rows.Close()
			return nil, err
		}
		kind := schema.KindTable
		if typ == "view" {
			kind = schema.KindView
		}
		list = append(list, tbl{name: name, kind: kind})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	s := &schema.Schema{}
	for _, item := range list {
		t := &schema.Table{Name: item.name, Kind: item.kind}
		if err := m.readTableInfo(ctx, db, t); err != nil {
			return nil, err
		}
		if err := m.readForeignKeys(ctx, db, t); err != nil {
			return nil, err
		}
		s.Tables = append(s.Tables, *t)
	}
	finalize(s)
	return s, nil
}

func (m *sqliteIntrospector) readTableInfo(ctx context.Context, db *sql.DB, t *schema.Table) error {
	// PRAGMA cannot be parameterized; identifier is a catalog name, quoted.
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%q)`, t.Name))
	if err != nil {
		return fmt.Errorf("introspect sqlite table_info(%s): %w", t.Name, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if ctype == "" {
			ctype = "text" // SQLite dynamic typing: default affinity
		}
		col := schema.Column{
			Name:      name,
			SQLType:   ctype,
			Nullable:  notnull == 0 && pk == 0,
			Default:   dflt.String,
			IsPrimary: pk > 0,
		}
		t.Columns = append(t.Columns, col)
		if pk > 0 {
			t.PrimaryKey = append(t.PrimaryKey, name)
		}
	}
	return rows.Err()
}

func (m *sqliteIntrospector) readForeignKeys(ctx context.Context, db *sql.DB, t *schema.Table) error {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA foreign_key_list(%q)`, t.Name))
	if err != nil {
		return fmt.Errorf("introspect sqlite foreign_key_list(%s): %w", t.Name, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, seq int
		var refTable, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return err
		}
		upsertColumnRef(t, from, refTable, to)
	}
	return rows.Err()
}
