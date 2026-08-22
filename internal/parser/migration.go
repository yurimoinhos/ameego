package parser

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yurimoinhos/ameego/internal/schema"
)

var (
	createTableRe = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([` + "`" + `"\[]?[\w.]+[` + "`" + `"\]]?)\s*\((.*?)\)\s*(?:;|$)`)
	alterAddColRe = regexp.MustCompile(`(?is)ALTER\s+TABLE\s+([` + "`" + `"\[]?[\w.]+[` + "`" + `"\]]?)\s+ADD\s+(?:COLUMN\s+)?(\w+)\s+(.+?)(?:;|$)`)
	pkInlineRe    = regexp.MustCompile(`(?is)PRIMARY\s+KEY\s*\(([^)]+)\)`)
	pkColRe       = regexp.MustCompile(`(?is)PRIMARY\s+KEY`)
	autoIncrRe    = regexp.MustCompile(`(?is)(?:SERIAL|BIGSERIAL|SMALLSERIAL|IDENTITY|AUTO_INCREMENT|AUTOINCREMENT)`)
	notNullRe     = regexp.MustCompile(`(?is)\bNOT\s+NULL\b`)
	defaultRe     = regexp.MustCompile(`(?is)\bDEFAULT\s+([^,\n]+)`)
	referencesRe  = regexp.MustCompile(`(?is)REFERENCES\s+([` + "`" + `"\[]?[\w.]+[` + "`" + `"\]]?)\s*\(\s*([` + "`" + `"\w]+[` + "`" + `"]?)\s*\)`)
)

// Options configures migration parsing.
type Options struct {
	MigrationsDir string
}

// ParseMigrations reads all .sql files in the migrations directory and builds a schema.
func ParseMigrations(opts Options) (*schema.Schema, error) {
	if opts.MigrationsDir == "" {
		return nil, fmt.Errorf("migrations directory is required")
	}

	tables := map[string]*schema.Table{}
	var order []string

	err := filepath.WalkDir(opts.MigrationsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".sql") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		applySQL(string(content), tables, &order)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk migrations: %w", err)
	}

	s := &schema.Schema{Tables: make([]schema.Table, 0, len(order))}
	for _, name := range order {
		s.Tables = append(s.Tables, *tables[name])
	}
	return s, nil
}

func applySQL(sql string, tables map[string]*schema.Table, order *[]string) {
	for _, m := range createTableRe.FindAllStringSubmatch(sql, -1) {
		rawName := unquoteIdent(m[1])
		body := m[2]
		table := parseTableBody(rawName, body)
		if _, exists := tables[rawName]; !exists {
			*order = append(*order, rawName)
		}
		tables[rawName] = table
	}

	for _, m := range alterAddColRe.FindAllStringSubmatch(sql, -1) {
		rawName := unquoteIdent(m[1])
		colName := m[2]
		line := strings.TrimSpace(m[3])
		colType := extractSQLType(line, strings.Fields(line))
		col := schema.Column{
			Name:     colName,
			SQLType:  colType,
			Nullable: !notNullRe.MatchString(line),
		}
		if rm := referencesRe.FindStringSubmatch(line); len(rm) == 3 {
			col.RefTable = unquoteIdent(rm[1])
			col.RefColumn = unquoteIdent(rm[2])
		}
		table, ok := tables[rawName]
		if !ok {
			table = &schema.Table{Name: rawName}
			tables[rawName] = table
			*order = append(*order, rawName)
		}
		table.Columns = append(table.Columns, col)
	}
}

func parseTableBody(name, body string) *schema.Table {
	table := &schema.Table{Name: name}
	lines := splitColumnDefs(body)

	var inlinePK []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)

		if strings.HasPrefix(upper, "PRIMARY KEY") {
			if m := pkInlineRe.FindStringSubmatch(line); len(m) == 2 {
				inlinePK = splitCSV(m[1])
			}
			continue
		}
		if strings.HasPrefix(upper, "CONSTRAINT ") || strings.HasPrefix(upper, "UNIQUE ") ||
			strings.HasPrefix(upper, "FOREIGN KEY") || strings.HasPrefix(upper, "CHECK ") ||
			strings.HasPrefix(upper, "INDEX ") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		colName := unquoteIdent(parts[0])
		colType := extractSQLType(line, parts)
		col := schema.Column{
			Name:       colName,
			SQLType:    colType,
			Nullable:   !notNullRe.MatchString(line),
			IsPrimary:  pkColRe.MatchString(line),
			IsAutoIncr: autoIncrRe.MatchString(line),
		}
		if dm := defaultRe.FindStringSubmatch(line); len(dm) == 2 {
			col.Default = strings.TrimSpace(dm[1])
		}
		if rm := referencesRe.FindStringSubmatch(line); len(rm) == 3 {
			col.RefTable = unquoteIdent(rm[1])
			col.RefColumn = unquoteIdent(rm[2])
		}
		table.Columns = append(table.Columns, col)
	}

	if len(inlinePK) > 0 {
		table.PrimaryKey = inlinePK
		for i := range table.Columns {
			for _, pk := range inlinePK {
				if table.Columns[i].Name == pk {
					table.Columns[i].IsPrimary = true
					table.Columns[i].Nullable = false
				}
			}
		}
	} else {
		for i := range table.Columns {
			if table.Columns[i].IsPrimary {
				table.PrimaryKey = append(table.PrimaryKey, table.Columns[i].Name)
				table.Columns[i].Nullable = false
			}
		}
	}

	return table
}

func splitColumnDefs(body string) []string {
	var lines []string
	depth := 0
	var b strings.Builder
	for _, r := range body {
		switch r {
		case '(':
			depth++
			b.WriteRune(r)
		case ')':
			depth--
			b.WriteRune(r)
		case ',':
			if depth == 0 {
				lines = append(lines, b.String())
				b.Reset()
			} else {
				b.WriteRune(r)
			}
		default:
			b.WriteRune(r)
		}
	}
	if rem := strings.TrimSpace(b.String()); rem != "" {
		lines = append(lines, rem)
	}
	return lines
}

func extractSQLType(line string, parts []string) string {
	var typeParts []string
	for i := 1; i < len(parts); i++ {
		p := parts[i]
		up := strings.ToUpper(p)
		if up == "NOT" || up == "NULL" || up == "DEFAULT" || up == "PRIMARY" ||
			up == "UNIQUE" || up == "REFERENCES" || up == "CHECK" || up == "CONSTRAINT" ||
			up == "GENERATED" || up == "IDENTITY" || up == "AUTO_INCREMENT" {
			break
		}
		typeParts = append(typeParts, p)
	}
	t := strings.Join(typeParts, " ")
	if strings.Contains(line, "(") && !strings.Contains(t, "(") {
		if idx := strings.Index(line, "("); idx >= 0 {
			if end := strings.Index(line[idx:], ")"); end >= 0 {
				t += line[idx : idx+end+1]
			}
		}
	}
	return strings.TrimSpace(t)
}

func unquoteIdent(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		switch s[0] {
		case '"', '`', '[', ']':
			return strings.Trim(s, `"`+"`"+`[]`)
		}
	}
	if dot := strings.LastIndex(s, "."); dot >= 0 {
		return unquoteIdent(s[dot+1:])
	}
	return s
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, unquoteIdent(p))
		}
	}
	return out
}
