package naming

import (
	"strings"
	"unicode"
)

// StructName converts a table name to an exported Go struct name.
func StructName(table string) string {
	return toExported(singularize(stripSchema(table)))
}

// SQLConstName builds a const identifier for a named query SQL string.
func SQLConstName(structName, queryName string) string {
	return toExported(structName) + toExported(queryName) + "SQL"
}

// VarName converts a param name to a lowercase Go parameter name.
func VarName(name string) string {
	parts := splitWords(name)
	for i, p := range parts {
		if i == 0 {
			parts[i] = strings.ToLower(p)
		} else {
			parts[i] = capitalize(p)
		}
	}
	return strings.Join(parts, "")
}

// RepoName returns the repository type name for a table.
func RepoName(table string) string {
	return StructName(table) + "Repository"
}

// FieldName converts a column name to an exported Go field name.
func FieldName(column string) string {
	return toExported(column)
}

// ColumnTag returns the db tag value (snake_case column name).
func ColumnTag(column string) string {
	return strings.ToLower(column)
}

func stripSchema(name string) string {
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func toExported(s string) string {
	parts := splitWords(s)
	for i, p := range parts {
		parts[i] = capitalize(p)
	}
	return strings.Join(parts, "")
}

func splitWords(s string) []string {
	s = strings.ReplaceAll(s, "-", "_")
	raw := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == ' '
	})
	if len(raw) == 1 && !strings.Contains(s, "_") {
		return splitCamelCase(raw[0])
	}
	return raw
}

func splitCamelCase(s string) []string {
	var parts []string
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			if b.Len() > 0 {
				parts = append(parts, b.String())
				b.Reset()
			}
		}
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		parts = append(parts, b.String())
	}
	if len(parts) == 0 {
		return []string{s}
	}
	return parts
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(strings.ToLower(s))
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func singularize(s string) string {
	lower := strings.ToLower(s)
	switch {
	case strings.HasSuffix(lower, "ies"):
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(lower, "ses"), strings.HasSuffix(lower, "xes"), strings.HasSuffix(lower, "ches"), strings.HasSuffix(lower, "shes"):
		return s[:len(s)-2]
	case strings.HasSuffix(lower, "s") && !strings.HasSuffix(lower, "ss"):
		return s[:len(s)-1]
	default:
		return s
	}
}
