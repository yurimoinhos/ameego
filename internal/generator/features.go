package generator

import (
	"fmt"
	"strings"

	"github.com/yurimoinhos/ameego/internal/naming"
	"github.com/yurimoinhos/ameego/internal/schema"
)

// DomainView is a generated PostgreSQL DOMAIN type.
type DomainView struct {
	TypeName     string
	Name         string
	BaseType     string
	BaseGoType   string
	NotNull      bool
	Default      string
	Check        string
	Collate      string
	CreateSQL    string
	DropSQL      string
	QualifiedPkg string // import alias when referenced from table packages
}

// applyDialectFeatures maps dialect-native schema constructs onto Go types
// and view metadata. Features are concrete per dialect — no shared interface.
func applyDialectFeatures(cfg Config, view *PackageView) error {
	switch cfg.Dialect.Name() {
	case "postgres":
		return applyPostgresDomains(cfg, view)
	case "mssql":
		return applyMSSQLVectors(cfg, view)
	case "mysql":
		return applyMySQLTableFeatures(cfg, view)
	default:
		return nil
	}
}

func applyPostgresDomains(cfg Config, view *PackageView) error {
	if len(cfg.Schema.Domains) == 0 {
		return nil
	}
	domains := make([]DomainView, 0, len(cfg.Schema.Domains))
	byName := map[string]DomainView{}
	for _, d := range cfg.Schema.Domains {
		baseGo := cfg.Dialect.MapType(d.BaseType, false)
		dv := DomainView{
			TypeName:   naming.StructName(d.Name),
			Name:       d.Name,
			BaseType:   d.BaseType,
			BaseGoType: baseGo,
			NotNull:    d.NotNull,
			Default:    d.Default,
			Check:      d.Check,
			Collate:    d.Collate,
			CreateSQL:  buildDomainCreateSQL(d),
			DropSQL:    fmt.Sprintf(`DROP DOMAIN IF EXISTS "%s"`, strings.ReplaceAll(stripSchema(d.Name), `"`, `""`)),
		}
		domains = append(domains, dv)
		byName[strings.ToLower(stripSchema(d.Name))] = dv
	}
	view.Domains = domains
	view.HasPostgresDomains = true

	domainPkg := "postgres"
	domainImport := ""
	if cfg.ModulePath != "" {
		domainImport = cfg.ModulePath + "/postgres"
	}

	for i := range view.Entities {
		ev := &view.Entities[i]
		for ci := range ev.Columns {
			col := &ev.Columns[ci]
			tableCol, ok := findSchemaColumn(cfg.Schema, ev.TableName, col.SQLName)
			if !ok || tableCol.Domain == "" {
				continue
			}
			dv, ok := byName[strings.ToLower(stripSchema(tableCol.Domain))]
			if !ok {
				continue
			}
			col.DomainType = dv.TypeName
			if domainImport == "" {
				// Without module path, keep the base mapped Go type.
				col.GoType = cfg.Dialect.MapType(dv.BaseType, tableCol.Nullable)
				continue
			}
			goType := domainPkg + "." + dv.TypeName
			if tableCol.Nullable && !dv.NotNull {
				goType = "*" + goType
			}
			col.GoType = goType
			ev.NeedsPostgresImport = true
			ev.PostgresImportPath = domainImport
		}
	}
	return nil
}

func applyMSSQLVectors(cfg Config, view *PackageView) error {
	for i := range view.Entities {
		ev := &view.Entities[i]
		var vectorCols []ColumnView
		for ci := range ev.Columns {
			col := &ev.Columns[ci]
			tableCol, ok := findSchemaColumn(cfg.Schema, ev.TableName, col.SQLName)
			if !ok || tableCol.VectorDim <= 0 {
				continue
			}
			goType := "mssql.Vector"
			if tableCol.Nullable {
				goType = "*mssql.Vector"
			}
			col.GoType = goType
			col.VectorDim = tableCol.VectorDim
			ev.NeedsMSSQLImport = true
			ev.HasVectorColumn = true
			vectorCols = append(vectorCols, *col)
		}
		ev.VectorColumns = vectorCols
	}
	view.HasMSSQLFeatures = true
	return nil
}

func applyMySQLTableFeatures(cfg Config, view *PackageView) error {
	tableByName := map[string]schema.Table{}
	for _, t := range cfg.Schema.Tables {
		tableByName[t.Name] = t
		tableByName[strings.ToLower(t.Name)] = t
	}
	for i := range view.Entities {
		ev := &view.Entities[i]
		t, ok := tableByName[ev.TableName]
		if !ok {
			t, ok = tableByName[strings.ToLower(ev.TableName)]
		}
		if !ok {
			continue
		}
		ev.Temporary = t.Temporary
		ev.Engine = t.Engine
		ev.TableOptions = t.Options
		if t.Temporary || t.Engine != "" {
			ev.NeedsMySQLImport = true
			ev.HasMySQLFeatures = true
			ev.CreateTemporarySQL = buildCreateTemporarySQL(t, cfg.Dialect.QuoteIdent)
			ev.DropTemporarySQL = fmt.Sprintf("DROP TEMPORARY TABLE IF EXISTS %s", cfg.Dialect.QuoteIdent(t.Name))
			ev.AlterEngineSQL = ""
			if t.Engine != "" {
				ev.AlterEngineSQL = fmt.Sprintf("ALTER TABLE %s ENGINE=%s", cfg.Dialect.QuoteIdent(t.Name), t.Engine)
			}
		}
	}
	view.HasMySQLFeatures = true
	return nil
}

func findSchemaColumn(s *schema.Schema, table, column string) (schema.Column, bool) {
	for _, t := range s.Tables {
		if !strings.EqualFold(t.Name, table) && t.Name != table {
			continue
		}
		for _, c := range t.Columns {
			if strings.EqualFold(c.Name, column) {
				return c, true
			}
		}
	}
	return schema.Column{}, false
}

func stripSchema(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

func buildDomainCreateSQL(d schema.Domain) string {
	var b strings.Builder
	name := stripSchema(d.Name)
	b.WriteString(`CREATE DOMAIN "`)
	b.WriteString(strings.ReplaceAll(name, `"`, `""`))
	b.WriteString(`" AS `)
	b.WriteString(d.BaseType)
	if d.Collate != "" {
		b.WriteString(` COLLATE "`)
		b.WriteString(strings.ReplaceAll(d.Collate, `"`, `""`))
		b.WriteString(`"`)
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

func buildCreateTemporarySQL(t schema.Table, quote func(string) string) string {
	cols := make([]string, 0, len(t.Columns))
	for _, c := range t.Columns {
		line := quote(c.Name) + " " + c.SQLType
		if !c.Nullable {
			line += " NOT NULL"
		}
		if c.IsPrimary && len(t.PrimaryKey) == 1 {
			line += " PRIMARY KEY"
		}
		if c.Default != "" {
			line += " DEFAULT " + c.Default
		}
		cols = append(cols, line)
	}
	sql := fmt.Sprintf("CREATE TEMPORARY TABLE %s (%s)", quote(t.Name), strings.Join(cols, ", "))
	if t.Engine != "" {
		sql += " ENGINE=" + t.Engine
	}
	return sql
}
