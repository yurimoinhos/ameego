package parser

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/yurimoinhos/ameego/internal/dialect"
	"github.com/yurimoinhos/ameego/internal/naming"
)

// RoutineResultKind describes how a routine returns data when invoked.
type RoutineResultKind string

const (
	RoutineScalar   RoutineResultKind = "scalar"
	RoutineOne      RoutineResultKind = "one"
	RoutineMany     RoutineResultKind = "many"
	RoutineExec     RoutineResultKind = "exec"
	RoutineExecRows RoutineResultKind = "execrows"
)

// ParsedRoutine is a database routine extracted from a SQL file.
type ParsedRoutine struct {
	Name           string
	MethodName     string
	Kind           dialect.RoutineKind
	ResultKind     RoutineResultKind
	QualifiedName  string
	DefinitionSQL  string
	InvokeSQL      string
	SourceFile     string
	Line           int
	ParamNames     []string
	ParamTypes     []string
	ReturnGoType   string
}

var (
	routineNameRe   = regexp.MustCompile(`(?i)^--\s*name:\s*(\w+)\s*(?::(function|procedure|trigger|scalar|one|many|exec|execrows))?\s*$`)
	routineParamRe  = regexp.MustCompile(`(?i)^--\s*params?:\s*(.+)\s*$`)
	routineReturnRe = regexp.MustCompile(`(?i)^--\s*returns?:\s*(\w+)\s*$`)
	routineInvokeRe = regexp.MustCompile(`(?i)^--\s*invoke:\s*(.+)\s*$`)
	routineDBNameRe   = regexp.MustCompile(`(?i)^--\s*db(?:name)?:\s*([\w.]+)\s*$`)

	createRoutineRe = regexp.MustCompile(`(?is)CREATE\s+(?:OR\s+REPLACE\s+)?(?:FUNCTION|PROCEDURE|TRIGGER)\s+([` + "`" + `"\[]?[\w.]+[` + "`" + `"\]]?)`)
)

// ParseRoutineFile reads routines from a SQL file.
//
// Format:
//
//	-- name: CountActiveUsers :function
//	-- dbname: count_active_users
//	-- returns: int64
//	-- params: active
//	CREATE OR REPLACE FUNCTION count_active_users(p_active BOOLEAN) ...
func ParseRoutineFile(path string, d dialect.Dialect) ([]ParsedRoutine, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read routine file %s: %w", path, err)
	}
	return ParseRoutineSQL(string(raw), path, d)
}

// ParseRoutineSQL parses routines from SQL text.
func ParseRoutineSQL(content, source string, d dialect.Dialect) ([]ParsedRoutine, error) {
	lines := strings.Split(content, "\n")
	var routines []ParsedRoutine
	var current *ParsedRoutine
	var sqlLines []string

	flush := func() error {
		if current == nil {
			return nil
		}
		sql := strings.TrimSpace(strings.Join(sqlLines, "\n"))
		if sql == "" {
			return fmt.Errorf("%s:%d: routine %q has no SQL body", source, current.Line, current.Name)
		}
		current.DefinitionSQL = sql
		if current.QualifiedName == "" {
			if m := createRoutineRe.FindStringSubmatch(sql); len(m) == 2 {
				current.QualifiedName = unquoteIdent(m[1])
			}
		}
		if current.QualifiedName == "" {
			return fmt.Errorf("%s:%d: routine %q: missing dbname or CREATE name", source, current.Line, current.Name)
		}
		if current.Kind == dialect.RoutineTrigger {
			routines = append(routines, *current)
			current = nil
			sqlLines = nil
			return nil
		}
		if current.InvokeSQL == "" {
			current.InvokeSQL = dialect.InvokeSQL(d, current.Kind, current.QualifiedName, len(current.ParamNames))
		}
		if current.InvokeSQL == "" {
			return fmt.Errorf("%s:%d: routine %q: cannot build invoke SQL for kind %q", source, current.Line, current.Name, current.Kind)
		}
		if current.ReturnGoType == "" {
			current.ReturnGoType = defaultReturnType(current.ResultKind)
		}
		current.MethodName = naming.FieldName(current.Name)
		routines = append(routines, *current)
		current = nil
		sqlLines = nil
		return nil
	}

	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if m := routineNameRe.FindStringSubmatch(trim); len(m) >= 2 {
			if err := flush(); err != nil {
				return nil, err
			}
			kind, result := parseRoutineKind(m[2])
			current = &ParsedRoutine{
				Name:       m[1],
				Kind:       kind,
				ResultKind: result,
				SourceFile: source,
				Line:       i + 1,
			}
			continue
		}
		if current == nil {
			if trim == "" || strings.HasPrefix(trim, "--") {
				continue
			}
			return nil, fmt.Errorf("%s:%d: SQL outside routine block", source, i+1)
		}
		if m := routineParamRe.FindStringSubmatch(trim); len(m) == 2 {
			current.ParamNames = splitParamNames(m[1])
			current.ParamTypes = parseParamTypes(m[1])
			continue
		}
		if m := routineReturnRe.FindStringSubmatch(trim); len(m) == 2 {
			current.ReturnGoType = mapReturnType(m[1])
			continue
		}
		if m := routineInvokeRe.FindStringSubmatch(trim); len(m) == 2 {
			current.InvokeSQL = strings.TrimSpace(m[1])
			continue
		}
		if m := routineDBNameRe.FindStringSubmatch(trim); len(m) == 2 {
			current.QualifiedName = m[1]
			continue
		}
		if strings.HasPrefix(trim, "--") {
			continue
		}
		sqlLines = append(sqlLines, line)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return routines, nil
}

func parseRoutineKind(raw string) (dialect.RoutineKind, RoutineResultKind) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "procedure":
		return dialect.RoutineProcedure, RoutineExec
	case "trigger":
		return dialect.RoutineTrigger, RoutineExec
	case "scalar":
		return dialect.RoutineFunction, RoutineScalar
	case "one":
		return dialect.RoutineFunction, RoutineOne
	case "many":
		return dialect.RoutineFunction, RoutineMany
	case "execrows":
		return dialect.RoutineProcedure, RoutineExecRows
	case "exec":
		return dialect.RoutineProcedure, RoutineExec
	default:
		return dialect.RoutineFunction, RoutineScalar
	}
}

func parseParamTypes(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		fields := strings.Fields(p)
		if len(fields) >= 2 {
			out = append(out, mapReturnType(fields[len(fields)-1]))
		} else {
			out = append(out, "string")
		}
	}
	return out
}

func mapReturnType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "bool", "boolean":
		return "bool"
	case "int", "int32":
		return "int32"
	case "int64", "bigint":
		return "int64"
	case "float32":
		return "float32"
	case "float64":
		return "float64"
	case "string", "text", "varchar":
		return "string"
	case "bytes", "[]byte":
		return "[]byte"
	default:
		return "string"
	}
}

func defaultReturnType(kind RoutineResultKind) string {
	switch kind {
	case RoutineScalar:
		return "int64"
	case RoutineExecRows:
		return "int64"
	default:
		return "string"
	}
}

// ParseRoutineDir loads all routine files from a directory.
func ParseRoutineDir(dir string, d dialect.Dialect) ([]ParsedRoutine, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var all []ParsedRoutine
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".sql") {
			continue
		}
		path := dir + string(os.PathSeparator) + e.Name()
		routines, err := ParseRoutineFile(path, d)
		if err != nil {
			return nil, err
		}
		all = append(all, routines...)
	}
	return all, nil
}
