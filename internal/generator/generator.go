package generator

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/yurimoinhos/ameego/internal/codegen"
	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/naming"
	"github.com/yurimoinhos/ameego/internal/parser"
	"github.com/yurimoinhos/ameego/internal/schema"
	"github.com/yurimoinhos/ameego/pkg/goql"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// QueryBinding associates parsed queries with an entity.
type QueryBinding struct {
	Entity  string
	Queries []parser.NamedQuery
}

// Config controls code generation output.
type Config struct {
	Package          string
	OutputDir        string
	Dialect          dialect.Dialect
	Schema           *schema.Schema
	ModulePath       string
	ConfigPath       string
	WithQueries      bool
	WithEntities     bool
	WithRepositories bool
	QueryBindings    []QueryBinding
}

// EntityView is the template view for a single table entity.
type EntityView struct {
	StructName    string
	PluralName    string
	RefTypeName   string
	ColsTypeName  string
	TableName     string
	RepoName      string
	Columns       []ColumnView
	PrimaryKey    []string
	HasSinglePK   bool
	PKField       string
	PKGoType      string
	PKColumn      string
	DialectName   string
	SelectAllSQL  string
	SelectByIDSQL string
	InsertSQL     string
	UpdateSQL     string
	DeleteByIDSQL string
}

// NestedEntityView is a generated nested result struct for FK joins.
type NestedEntityView struct {
	StructName     string
	ParentStruct   string
	NestedStruct   string
	NestedField    string
	NestedPrefix   string
	ParentColumns  []ColumnView
}

// ColumnView is the template view for a column.
type ColumnView struct {
	FieldName string
	GoType    string
	DBTag     string
	SQLName   string
}

// CustomQueryView is the template view for a custom SQL query method.
type CustomQueryView struct {
	Name          string
	MethodName    string
	ResultKind    string
	SQL           string
	SQLConstName  string
	RepoName      string
	StructName    string
	SourceFile    string
	Params        []ParamView
}

// ParamView is a typed query parameter.
type ParamView struct {
	Name      string
	VarName   string
	FieldName string
	GoType    string
}

// PackageView is the root template view.
type PackageView struct {
	Package         string
	ModulePath      string
	DialectName     string
	ConfigPath      string
	Entities        []EntityView
	NestedEntities  []NestedEntityView
	CustomQueries   []CustomQueryView
	NeedsSQLImport  bool
	NeedsTimeImport bool
}

// Generate writes all generated Go files to OutputDir.
func Generate(cfg Config) error {
	if cfg.Package == "" {
		cfg.Package = "generated"
	}
	if cfg.OutputDir == "" {
		return fmt.Errorf("output directory is required")
	}
	if cfg.Schema == nil {
		return fmt.Errorf("schema is required")
	}
	if cfg.Dialect == nil {
		return fmt.Errorf("dialect is required")
	}
	if !cfg.WithEntities && !cfg.WithRepositories && !cfg.WithQueries && len(cfg.QueryBindings) == 0 {
		return fmt.Errorf("nothing to generate: enable entities, repositories, query_helpers, or add queries in goql.yaml")
	}

	dialect.ApplyGoTypes(cfg.Dialect, cfg.Schema)

	view, err := buildPackageView(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("mkdir output: %w", err)
	}

	if cfg.WithEntities {
		if err := writeTemplate("entities.go.tmpl", filepath.Join(cfg.OutputDir, "entities_gen.go"), view); err != nil {
			return err
		}
		if err := writeTemplate("refs.go.tmpl", filepath.Join(cfg.OutputDir, "refs_gen.go"), view); err != nil {
			return err
		}
		if len(view.NestedEntities) > 0 {
			if err := writeTemplate("nested.go.tmpl", filepath.Join(cfg.OutputDir, "nested_gen.go"), view); err != nil {
				return err
			}
		}
	}
	if cfg.WithRepositories {
		if err := writeTemplate("repositories.go.tmpl", filepath.Join(cfg.OutputDir, "repositories_gen.go"), view); err != nil {
			return err
		}
	}
	if cfg.WithQueries {
		if err := writeTemplate("queries.go.tmpl", filepath.Join(cfg.OutputDir, "queries_gen.go"), view); err != nil {
			return err
		}
	}
	if len(view.CustomQueries) > 0 {
		if err := writeTemplate("custom_queries_sql.go.tmpl", filepath.Join(cfg.OutputDir, "custom_queries_sql_gen.go"), view); err != nil {
			return err
		}
		if err := writeTemplate("custom_queries.go.tmpl", filepath.Join(cfg.OutputDir, "custom_queries_gen.go"), view); err != nil {
			return err
		}
	}
	return writeTemplate("doc.go.tmpl", filepath.Join(cfg.OutputDir, "doc_gen.go"), view)
}

func buildPackageView(cfg Config) (PackageView, error) {
	sqlBuilder := codegen.NewBuilder(cfg.Dialect)
	entities := make([]EntityView, 0, len(cfg.Schema.Tables))
	entityByStruct := map[string]*EntityView{}
	needsSQL := false
	needsTime := false

	for _, table := range cfg.Schema.Tables {
		colNames := make([]string, 0, len(table.Columns))
		ev := EntityView{
			StructName:  naming.StructName(table.Name),
			PluralName:  naming.PluralStructName(table.Name),
			RefTypeName: naming.VarName(naming.StructName(table.Name)) + "Ref",
			ColsTypeName: naming.StructName(table.Name) + "Cols",
			TableName:   table.Name,
			RepoName:    naming.RepoName(table.Name),
			PrimaryKey:  table.PrimaryKey,
			DialectName: cfg.Dialect.Name(),
		}
		if len(table.PrimaryKey) == 1 {
			ev.HasSinglePK = true
			ev.PKColumn = table.PrimaryKey[0]
			ev.PKField = naming.FieldName(table.PrimaryKey[0])
			if pk, ok := table.PrimaryKeyColumn(); ok {
				ev.PKGoType = pk.GoType
			}
		}
		for _, col := range table.Columns {
			colNames = append(colNames, col.Name)
			ev.Columns = append(ev.Columns, ColumnView{
				FieldName: naming.FieldName(col.Name),
				GoType:    col.GoType,
				DBTag:     naming.ColumnTag(col.Name),
				SQLName:   col.Name,
			})
			if strings.HasPrefix(col.GoType, "sql.Null") {
				needsSQL = true
			}
			if col.GoType == "time.Time" || col.GoType == "sql.NullTime" {
				needsTime = true
			}
		}
		ev.SelectAllSQL = sqlBuilder.SelectAll(table.Name, colNames)
		if ev.HasSinglePK {
			ev.SelectByIDSQL = sqlBuilder.SelectByPK(table.Name, colNames, ev.PKColumn)
			ev.InsertSQL = sqlBuilder.Insert(table.Name, colNames)
			ev.UpdateSQL = sqlBuilder.UpdateByPK(table.Name, colNames, ev.PKColumn)
			ev.DeleteByIDSQL = sqlBuilder.DeleteByPK(table.Name, ev.PKColumn)
		}
		entities = append(entities, ev)
		entityCopy := ev
		entityByStruct[ev.StructName] = &entityCopy
	}

	customQueries, err := buildCustomQueries(cfg, entityByStruct)
	if err != nil {
		return PackageView{}, err
	}

	nested := buildNestedEntities(cfg.Schema, entityByStruct)

	return PackageView{
		Package:         cfg.Package,
		ModulePath:      cfg.ModulePath,
		DialectName:     cfg.Dialect.Name(),
		ConfigPath:      cfg.ConfigPath,
		Entities:        entities,
		NestedEntities:  nested,
		CustomQueries:   customQueries,
		NeedsSQLImport:  needsSQL,
		NeedsTimeImport: needsTime,
	}, nil
}

func buildNestedEntities(s *schema.Schema, entities map[string]*EntityView) []NestedEntityView {
	tableByName := map[string]schema.Table{}
	for _, t := range s.Tables {
		tableByName[t.Name] = t
	}

	var out []NestedEntityView
	for _, table := range s.Tables {
		parent, ok := entities[naming.StructName(table.Name)]
		if !ok {
			continue
		}
		for _, col := range table.Columns {
			if col.RefTable == "" {
				continue
			}
			refTable, ok := tableByName[col.RefTable]
			if !ok {
				continue
			}
			nestedStruct := naming.StructName(refTable.Name)
			nestedEntity, ok := entities[nestedStruct]
			if !ok {
				continue
			}
			out = append(out, NestedEntityView{
				StructName:    parent.StructName + "With" + nestedStruct,
				ParentStruct:  parent.StructName,
				NestedStruct:  nestedStruct,
				NestedField:   nestedStruct,
				NestedPrefix:  defaultAlias(refTable.Name),
				ParentColumns: parent.Columns,
			})
			_ = nestedEntity
		}
	}
	return out
}

func defaultAlias(table string) string {
	if i := strings.LastIndex(table, "."); i >= 0 {
		table = table[i+1:]
	}
	return table
}

func buildCustomQueries(cfg Config, entities map[string]*EntityView) ([]CustomQueryView, error) {
	var out []CustomQueryView
	seen := map[string]struct{}{}

	for _, binding := range cfg.QueryBindings {
		entity, ok := entities[binding.Entity]
		if !ok {
			return nil, fmt.Errorf("query entity %q not found in schema (available tables: %s)",
				binding.Entity, entityNames(entities))
		}
		colTypes := map[string]string{}
		for _, col := range entity.Columns {
			colTypes[strings.ToLower(col.SQLName)] = col.GoType
		}
		for _, q := range binding.Queries {
			key := entity.StructName + "." + q.Name
			if _, dup := seen[key]; dup {
				return nil, fmt.Errorf("duplicate query %q for entity %q", q.Name, binding.Entity)
			}
			seen[key] = struct{}{}

			sql, err := goql.NormalizePlaceholders(q.SQL, cfg.Dialect.Name())
			if err != nil {
				return nil, fmt.Errorf("query %q: %w", q.Name, err)
			}
			paramTypes := parser.ResolveParamTypes(q.ParamNames, colTypes)
			params := make([]ParamView, len(q.ParamNames))
			for i, name := range q.ParamNames {
				params[i] = ParamView{
					Name:      name,
					VarName:   naming.VarName(name),
					FieldName: naming.FieldName(name),
					GoType:    paramTypes[i],
				}
				if strings.HasPrefix(paramTypes[i], "sql.Null") {
					// keep
				}
			}
			constName := naming.SQLConstName(entity.StructName, q.Name)
			out = append(out, CustomQueryView{
				Name:         q.Name,
				MethodName:   parser.MethodName(q.Name),
				ResultKind:   string(q.ResultKind),
				SQL:          sql,
				SQLConstName: constName,
				RepoName:     entity.RepoName,
				StructName:   entity.StructName,
				SourceFile:   filepath.Base(q.SourceFile),
				Params:       params,
			})
		}
	}
	return out, nil
}

func entityNames(entities map[string]*EntityView) string {
	names := make([]string, 0, len(entities))
	for n := range entities {
		names = append(names, n)
	}
	return strings.Join(names, ", ")
}

func writeTemplate(name, outPath string, data any) error {
	raw, err := templateFS.ReadFile(filepath.Join("templates", name))
	if err != nil {
		return fmt.Errorf("read template %s: %w", name, err)
	}

	funcs := template.FuncMap{
		"join":        strings.Join,
		"joinStrings": func(parts []string, sep string) string { return strings.Join(parts, sep) },
		"quote":       func(s string) string { return "`" + strings.ReplaceAll(s, "`", "` + \"`\" + `") + "`" },
		"lower":       strings.ToLower,
		"hasPrefix":   strings.HasPrefix,
		"len":         func(v any) int {
			switch t := v.(type) {
			case []ColumnView:
				return len(t)
			default:
				return 0
			}
		},
	}

	tmpl, err := template.New(name).Funcs(funcs).Parse(string(raw))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template %s: %w", name, err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return os.WriteFile(outPath, buf.Bytes(), 0o644)
	}
	return os.WriteFile(outPath, formatted, 0o644)
}
