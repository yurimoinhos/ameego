package dialect

import (
	"fmt"
	"strings"
)

// RoutineKind classifies a database routine.
type RoutineKind string

const (
	RoutineFunction  RoutineKind = "function"
	RoutineProcedure RoutineKind = "procedure"
	RoutineTrigger   RoutineKind = "trigger"
)

// InvokeSQL builds the SQL that delegates execution to a database routine.
// Functions are invoked with SELECT; procedures with CALL/EXEC.
func InvokeSQL(d Dialect, kind RoutineKind, qualifiedName string, paramCount int) string {
	name := quoteQualified(d, qualifiedName)
	args := joinPlaceholders(d, paramCount)

	switch kind {
	case RoutineFunction:
		switch d.Name() {
		case "mssql":
			return fmt.Sprintf("SELECT %s(%s)", name, args)
		default:
			return fmt.Sprintf("SELECT %s(%s)", name, args)
		}
	case RoutineProcedure:
		switch d.Name() {
		case "postgres":
			return fmt.Sprintf("CALL %s(%s)", name, args)
		case "mssql":
			return fmt.Sprintf("EXEC %s %s", name, namedArgsMSSQL(paramCount))
		case "mysql":
			return fmt.Sprintf("CALL %s(%s)", name, args)
		default:
			return fmt.Sprintf("CALL %s(%s)", name, args)
		}
	default:
		return ""
	}
}

func joinPlaceholders(d Dialect, n int) string {
	if n == 0 {
		return ""
	}
	parts := make([]string, n)
	for i := 1; i <= n; i++ {
		parts[i-1] = d.Placeholder(i)
	}
	return strings.Join(parts, ", ")
}

func namedArgsMSSQL(n int) string {
	if n == 0 {
		return ""
	}
	parts := make([]string, n)
	for i := 1; i <= n; i++ {
		parts[i-1] = fmt.Sprintf("@p%d=@p%d", i, i)
	}
	return strings.Join(parts, ", ")
}

func quoteQualified(d Dialect, name string) string {
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		quoted := make([]string, len(parts))
		for i, p := range parts {
			quoted[i] = d.QuoteIdent(strings.TrimSpace(p))
		}
		return strings.Join(quoted, ".")
	}
	return d.QuoteIdent(name)
}
