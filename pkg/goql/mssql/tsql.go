package mssql

import (
	"fmt"
	"strings"
)

// TopClause returns the T-SQL TOP (n) select modifier (without SELECT keyword).
func TopClause(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("TOP (%d) ", n)
}

// ApplyTop rewrites a compiled SELECT statement to include TOP (n) after SELECT.
func ApplyTop(sql string, n int) string {
	if n <= 0 {
		return sql
	}
	trimmed := strings.TrimSpace(sql)
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "SELECT") {
		return sql
	}
	rest := strings.TrimSpace(trimmed[6:])
	if strings.HasPrefix(strings.ToUpper(rest), "TOP") {
		return sql
	}
	if strings.HasPrefix(strings.ToUpper(rest), "DISTINCT") {
		rest = strings.TrimSpace(rest[8:])
		return "SELECT DISTINCT " + TopClause(n) + rest
	}
	return "SELECT " + TopClause(n) + rest
}

// TableHint builds a WITH (hint) clause, e.g. WITH (NOLOCK).
func TableHint(hints ...string) string {
	clean := make([]string, 0, len(hints))
	for _, h := range hints {
		h = strings.TrimSpace(h)
		if h != "" {
			clean = append(clean, h)
		}
	}
	if len(clean) == 0 {
		return ""
	}
	return " WITH (" + strings.Join(clean, ", ") + ")"
}

// OptionClause builds OPTION (...), e.g. OPTION (RECOMPILE).
func OptionClause(options ...string) string {
	clean := make([]string, 0, len(options))
	for _, o := range options {
		o = strings.TrimSpace(o)
		if o != "" {
			clean = append(clean, o)
		}
	}
	if len(clean) == 0 {
		return ""
	}
	return " OPTION (" + strings.Join(clean, ", ") + ")"
}

// OutputInserted builds OUTPUT INSERTED.col1, INSERTED.col2 for INSERT/UPDATE.
func OutputInserted(columns ...string) string {
	if len(columns) == 0 {
		return "OUTPUT INSERTED.*"
	}
	parts := make([]string, len(columns))
	for i, c := range columns {
		parts[i] = "INSERTED." + quoteIdent(c)
	}
	return "OUTPUT " + strings.Join(parts, ", ")
}

func quoteIdent(name string) string {
	return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
}
