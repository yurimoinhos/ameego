package dialect

import "strings"

func normalizeSQLType(raw string) string {
	t := strings.ToLower(strings.TrimSpace(raw))
	if idx := strings.IndexByte(t, '('); idx >= 0 {
		t = strings.TrimSpace(t[:idx])
	}
	return t
}

func wrapNullable(goType string, nullable bool) string {
	if !nullable {
		return goType
	}
	switch goType {
	case "string", "[]byte":
		return goType // zero value is fine
	case "int8", "int16", "int32", "int64", "uint8", "float32", "float64", "bool":
		return "sql.Null" + capitalize(nullTypeSuffix(goType))
	case "time.Time":
		return "sql.NullTime"
	default:
		return goType
	}
}

func nullTypeSuffix(goType string) string {
	switch goType {
	case "int8":
		return "Int16" // sql.NullInt16 covers small ints
	case "uint8":
		return "Int16"
	default:
		return strings.ToUpper(goType[:1]) + goType[1:]
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
