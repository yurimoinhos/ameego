package parser

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/yurimoinhos/ameego/internal/naming"
)

// QueryResultKind describes how a named query returns data.
type QueryResultKind string

const (
	QueryOne      QueryResultKind = "one"
	QueryMany     QueryResultKind = "many"
	QueryExec     QueryResultKind = "exec"
	QueryExecRows QueryResultKind = "execrows"
	QueryScalar   QueryResultKind = "scalar"
)

// NamedQuery is a parsed query from a .sql file.
type NamedQuery struct {
	Name       string
	ResultKind QueryResultKind
	SQL        string
	SourceFile string
	ParamNames []string
	ParamTypes []string
	Line       int
}

var (
	queryNameRe  = regexp.MustCompile(`(?i)^--\s*name:\s*(\w+)\s*:(one|many|exec|execrows|scalar)\s*$`)
	queryParamRe = regexp.MustCompile(`(?i)^--\s*params?:\s*(.+)\s*$`)
	placeholders = regexp.MustCompile(`(?i)(?:\$\d+|@p\d+|\?)`)
)

// ParseQueryFile reads a SQL file with named query annotations.
//
// Format:
//
//	-- name: FindByEmail :one
//	-- params: email, active
//	SELECT id, email FROM users WHERE email = $1 AND active = $2;
func ParseQueryFile(path string) ([]NamedQuery, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read query file %s: %w", path, err)
	}
	return ParseQuerySQL(string(raw), path)
}

// ParseQuerySQL parses named queries from SQL text.
func ParseQuerySQL(content, source string) ([]NamedQuery, error) {
	lines := strings.Split(content, "\n")
	var queries []NamedQuery
	var current *NamedQuery
	var sqlLines []string
	flush := func() error {
		if current == nil {
			return nil
		}
		sql := strings.TrimSpace(strings.Join(sqlLines, "\n"))
		if sql == "" {
			return fmt.Errorf("%s:%d: query %q has no SQL body", source, current.Line, current.Name)
		}
		current.SQL = sql
		n := countPlaceholders(sql)
		if len(current.ParamNames) == 0 {
			current.ParamNames = make([]string, n)
			for i := range current.ParamNames {
				current.ParamNames[i] = fmt.Sprintf("arg%d", i+1)
			}
		}
		if n != len(current.ParamNames) {
			return fmt.Errorf("%s:%d: query %q expects %d params, found %d placeholders",
				source, current.Line, current.Name, len(current.ParamNames), n)
		}
		queries = append(queries, *current)
		current = nil
		sqlLines = nil
		return nil
	}

	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if m := queryNameRe.FindStringSubmatch(trim); len(m) == 3 {
			if err := flush(); err != nil {
				return nil, err
			}
			current = &NamedQuery{
				Name:       m[1],
				ResultKind: QueryResultKind(strings.ToLower(m[2])),
				SourceFile: source,
				Line:       i + 1,
			}
			continue
		}
		if current == nil {
			if trim == "" || strings.HasPrefix(trim, "--") {
				continue
			}
			return nil, fmt.Errorf("%s:%d: SQL outside named query block", source, i+1)
		}
		if m := queryParamRe.FindStringSubmatch(trim); len(m) == 2 {
			current.ParamNames = splitParamNames(m[1])
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
	return queries, nil
}

func splitParamNames(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func countPlaceholders(sql string) int {
	return len(placeholders.FindAllString(sql, -1))
}

func inferParamNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("arg%d", i+1)
	}
	return names
}

// ResolveParamTypes maps param names to Go types using entity column types.
func ResolveParamTypes(paramNames []string, columnTypes map[string]string) []string {
	types := make([]string, len(paramNames))
	for i, name := range paramNames {
		colKey := strings.ToLower(name)
		if goType, ok := columnTypes[colKey]; ok {
			types[i] = goType
			continue
		}
		types[i] = "string"
	}
	return types
}

// MethodName converts a query name to an exported Go method name.
func MethodName(name string) string {
	return naming.FieldName(name)
}
