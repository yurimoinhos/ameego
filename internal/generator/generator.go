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

//go:embed templates/*.tmpl templates/root/*.tmpl templates/table/*.tmpl templates/routine/*.tmpl
var templateFS embed.FS

// QueryBinding associates parsed queries with an entity.
type QueryBinding struct {
	Entity  string
	Queries []parser.NamedQuery
}

// RoutineBinding associates parsed routines with a source file.
type RoutineBinding struct {
	SourceFile string
	Routines   []parser.ParsedRoutine
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
	WithRoutines     bool
	QueryBindings    []QueryBinding
	RoutineBindings  []RoutineBinding
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

// RoutineView is the template view for a database routine.
type RoutineView struct {
	Name            string
	MethodName      string
	Kind            string
	ResultKind      string
	QualifiedName   string
	DefinitionSQL   string
	InvokeSQL       string
	SQLConstName    string
	InvokeConstName string
	SourceFile      string
	ReturnGoType    string
	Params          []ParamView
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

// TableView is the template view for a single table package.
type TableView struct {
	Package         string
	Entity          EntityView
	NestedEntities  []NestedEntityView
	CustomQueries   []CustomQueryView
	NeedsSQLImport  bool
	NeedsTimeImport bool
}

// RoutinePackageView is the template view for a single routine package.
type RoutinePackageView struct {
	Package string
	Routine RoutineView
}

// RootView is the root doc package view.
type RootView struct {
	Package           string
	ConfigPath        string
	TablesImportPath  string
	FunctionsPath     string
	ProceduresPath    string
	TriggersPath      string
}

// PackageView is the aggregate view used while building output.
type PackageView struct {
	Package         string
	ModulePath      string
	DialectName     string
	ConfigPath      string
	Entities        []EntityView
	NestedEntities  []NestedEntityView
	CustomQueries   []CustomQueryView
	Routines        []RoutineView
	NeedsSQLImport  bool
	NeedsTimeImport bool
}

// Generate writes per-table and per-routine Go packages under OutputDir.
func Generate(cfg Config) error {
	if cfg.Package == "" {
		cfg.Package = "store"
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
	if !cfg.WithEntities && !cfg.WithRepositories && !cfg.WithQueries && !cfg.WithRoutines && len(cfg.QueryBindings) == 0 && len(cfg.RoutineBindings) == 0 {
		return fmt.Errorf("nothing to generate: enable entities, repositories, query_helpers, routines, or add queries/routines in ameego.yaml")
	}

	dialect.ApplyGoTypes(cfg.Dialect, cfg.Schema)

	view, err := buildPackageView(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("mkdir output: %w", err)
	}

	tablesBase := "tables"
	functionsBase := "functions"
	proceduresBase := "procedures"
	triggersBase := "triggers"
	if cfg.ModulePath != "" {
		tablesBase = cfg.ModulePath + "/tables"
		functionsBase = cfg.ModulePath + "/functions"
		proceduresBase = cfg.ModulePath + "/procedures"
		triggersBase = cfg.ModulePath + "/triggers"
	}

	root := RootView{
		Package:          cfg.Package,
		ConfigPath:       cfg.ConfigPath,
		TablesImportPath: tablesBase,
		FunctionsPath:    functionsBase,
		ProceduresPath:   proceduresBase,
		TriggersPath:     triggersBase,
	}
	if err := writeTemplate("root/doc.go.tmpl", filepath.Join(cfg.OutputDir, "doc_gen.go"), root); err != nil {
		return err
	}

	queriesByEntity := map[string][]CustomQueryView{}
	for _, q := range view.CustomQueries {
		queriesByEntity[q.StructName] = append(queriesByEntity[q.StructName], q)
	}
	nestedByParent := map[string][]NestedEntityView{}
	for _, n := range view.NestedEntities {
		nestedByParent[n.ParentStruct] = append(nestedByParent[n.ParentStruct], n)
	}

	for _, entity := range view.Entities {
		pkg := naming.PackageName(entity.TableName)
		dir := filepath.Join(cfg.OutputDir, "tables", pkg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir table package %s: %w", pkg, err)
		}

		needsSQL, needsTime := entityImports(entity)
		tableView := TableView{
			Package:         pkg,
			Entity:          entity,
			NestedEntities:  nestedByParent[entity.StructName],
			CustomQueries:   queriesByEntity[entity.StructName],
			NeedsSQLImport:  needsSQL,
			NeedsTimeImport: needsTime,
		}

		if cfg.WithEntities {
			if err := writeTemplate("table/entity.go.tmpl", filepath.Join(dir, "entity_gen.go"), tableView); err != nil {
				return err
			}
			if err := writeTemplate("table/refs.go.tmpl", filepath.Join(dir, "refs_gen.go"), tableView); err != nil {
				return err
			}
			if len(tableView.NestedEntities) > 0 {
				if err := writeTemplate("table/nested.go.tmpl", filepath.Join(dir, "nested_gen.go"), tableView); err != nil {
					return err
				}
			}
		}
		if cfg.WithRepositories {
			if err := writeTemplate("table/repository.go.tmpl", filepath.Join(dir, "repository_gen.go"), tableView); err != nil {
				return err
			}
		}
		if cfg.WithQueries && entity.HasSinglePK {
			if err := writeTemplate("table/queries.go.tmpl", filepath.Join(dir, "queries_gen.go"), tableView); err != nil {
				return err
			}
		}
		if len(tableView.CustomQueries) > 0 {
			if err := writeTemplate("table/custom_queries_sql.go.tmpl", filepath.Join(dir, "custom_queries_sql_gen.go"), tableView); err != nil {
				return err
			}
			if err := writeTemplate("table/custom_queries.go.tmpl", filepath.Join(dir, "custom_queries_gen.go"), tableView); err != nil {
				return err
			}
		}
	}

	for _, routine := range view.Routines {
		if !cfg.WithRoutines && routine.Kind != string(dialect.RoutineTrigger) {
			continue
		}
		subdir, tmpl := routineOutput(routine.Kind)
		pkg := naming.PackageName(routine.Name)
		dir := filepath.Join(cfg.OutputDir, subdir, pkg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir routine package %s: %w", pkg, err)
		}
		routineView := RoutinePackageView{Package: pkg, Routine: routine}
		if routine.Kind == string(dialect.RoutineTrigger) {
			if err := writeTemplate("routine/trigger.go.tmpl", filepath.Join(dir, "trigger_gen.go"), routineView); err != nil {
				return err
			}
			continue
		}
		if err := writeTemplate("routine/routine_sql.go.tmpl", filepath.Join(dir, "routine_sql_gen.go"), routineView); err != nil {
			return err
		}
		if err := writeTemplate(tmpl, filepath.Join(dir, "routine_gen.go"), routineView); err != nil {
			return err
		}
	}

	return nil
}

func routineOutput(kind string) (subdir, tmpl string) {
	switch kind {
	case string(dialect.RoutineProcedure):
		return "procedures", "routine/procedure.go.tmpl"
	case string(dialect.RoutineTrigger):
		return "triggers", "routine/trigger.go.tmpl"
	default:
		return "functions", "routine/function.go.tmpl"
	}
}

func entityImports(entity EntityView) (needsSQL, needsTime bool) {
	for _, col := range entity.Columns {
		if strings.HasPrefix(col.GoType, "sql.Null") {
			needsSQL = true
		}
		if col.GoType == "time.Time" || col.GoType == "sql.NullTime" {
			needsTime = true
		}
	}
	return needsSQL, needsTime
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

	routines, err := buildRoutines(cfg)
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
		Routines:        routines,
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

func buildRoutines(cfg Config) ([]RoutineView, error) {
	var out []RoutineView
	seen := map[string]struct{}{}

	for _, binding := range cfg.RoutineBindings {
		for _, r := range binding.Routines {
			if !cfg.WithRoutines && r.Kind != dialect.RoutineTrigger {
				continue
			}
			key := string(r.Kind) + "." + r.Name
			if _, dup := seen[key]; dup {
				return nil, fmt.Errorf("duplicate routine %q", r.Name)
			}
			seen[key] = struct{}{}

			invokeSQL, err := goql.NormalizePlaceholders(r.InvokeSQL, cfg.Dialect.Name())
			if err != nil && r.Kind != dialect.RoutineTrigger {
				return nil, fmt.Errorf("routine %q: %w", r.Name, err)
			}

			params := make([]ParamView, len(r.ParamNames))
			for i, name := range r.ParamNames {
				goType := "string"
				if i < len(r.ParamTypes) {
					goType = r.ParamTypes[i]
				}
				params[i] = ParamView{
					Name:      name,
					VarName:   naming.VarName(name),
					FieldName: naming.FieldName(name),
					GoType:    goType,
				}
			}

			out = append(out, RoutineView{
				Name:            r.Name,
				MethodName:      r.MethodName,
				Kind:            string(r.Kind),
				ResultKind:      string(r.ResultKind),
				QualifiedName:   r.QualifiedName,
				DefinitionSQL:   r.DefinitionSQL,
				InvokeSQL:       invokeSQL,
				SQLConstName:    naming.SQLConstName("Routine", r.Name) + "DefSQL",
				InvokeConstName: naming.SQLConstName("Routine", r.Name) + "InvokeSQL",
				SourceFile:      filepath.Base(r.SourceFile),
				ReturnGoType:    r.ReturnGoType,
				Params:          params,
			})
		}
	}
	return out, nil
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
