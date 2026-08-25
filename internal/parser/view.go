package parser

import (
	"strings"
	"unicode"

	"github.com/yurimoinhos/ameego/internal/schema"
)

func parseViews(sql string, tables map[string]*schema.Table, order *[]string) {
	upper := strings.ToUpper(sql)
	search := 0
	for {
		idx := indexCreateView(upper, search)
		if idx < 0 {
			return
		}
		rest := strings.TrimSpace(sql[idx:])
		rest = stripCreateViewPrefix(rest)
		name, rest, ok := readIdent(rest)
		if !ok || name == "" {
			search = idx + 6
			continue
		}

		rest = strings.TrimSpace(rest)
		var colNames []string
		if strings.HasPrefix(rest, "(") {
			body, n, ok := readBalancedParens(rest)
			if !ok {
				search = idx + 6
				continue
			}
			colNames = splitCSV(body)
			rest = strings.TrimSpace(rest[n:])
		}
		rest = skipViewOptions(rest)
		if !hasPrefixFold(rest, "AS") {
			search = idx + 6
			continue
		}
		rest = strings.TrimSpace(rest[2:])
		selectSQL := takeUntilUnquotedSemicolon(rest)

		if existing, exists := tables[name]; exists && !existing.IsView() {
			search = idx + 6
			continue
		}

		view := &schema.Table{Name: name, Kind: schema.KindView}
		if len(colNames) > 0 {
			for _, cn := range colNames {
				col := schema.Column{Name: cn, SQLType: "text", Nullable: true}
				if src := findColumnAnywhere(tables, cn); src != nil {
					col.SQLType = src.SQLType
					col.Nullable = src.Nullable
				}
				view.Columns = append(view.Columns, col)
			}
		} else {
			view.Columns = parseSelectColumns(selectSQL, tables)
		}
		if len(view.Columns) == 0 {
			search = idx + 6
			continue
		}
		if _, exists := tables[name]; !exists {
			*order = append(*order, name)
		}
		tables[name] = view
		search = idx + 6
	}
}

func indexCreateView(upper string, from int) int {
	if from >= len(upper) {
		return -1
	}
	pos := strings.Index(upper[from:], "CREATE")
	for pos >= 0 {
		abs := from + pos
		rest := strings.TrimSpace(upper[abs+6:])
		rest = strings.TrimPrefix(rest, "OR REPLACE")
		rest = strings.TrimSpace(rest)
		rest = strings.TrimPrefix(rest, "TEMPORARY")
		rest = strings.TrimSpace(rest)
		rest = strings.TrimPrefix(rest, "TEMP")
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, "VIEW") {
			return abs
		}
		next := strings.Index(upper[abs+6:], "CREATE")
		if next < 0 {
			return -1
		}
		from = abs + 6 + next
		pos = 0
		upper = upper // keep
		// re-loop with from
		return indexCreateView(upper, from)
	}
	return -1
}

func stripCreateViewPrefix(s string) string {
	s = strings.TrimSpace(s)
	if !hasPrefixFold(s, "CREATE") {
		return s
	}
	s = strings.TrimSpace(s[6:])
	if hasPrefixFold(s, "OR REPLACE") {
		s = strings.TrimSpace(s[len("OR REPLACE"):])
	}
	if hasPrefixFold(s, "TEMPORARY") {
		s = strings.TrimSpace(s[len("TEMPORARY"):])
	} else if hasPrefixFold(s, "TEMP") {
		s = strings.TrimSpace(s[4:])
	}
	if hasPrefixFold(s, "VIEW") {
		s = strings.TrimSpace(s[4:])
	}
	if hasPrefixFold(s, "IF NOT EXISTS") {
		s = strings.TrimSpace(s[len("IF NOT EXISTS"):])
	}
	return s
}

func skipViewOptions(s string) string {
	s = strings.TrimSpace(s)
	if !hasPrefixFold(s, "WITH") {
		return s
	}
	s = strings.TrimSpace(s[4:])
	if strings.HasPrefix(s, "(") {
		_, n, ok := readBalancedParens(s)
		if !ok {
			return s
		}
		return strings.TrimSpace(s[n:])
	}
	// WITH SCHEMABINDING [AS] or WITH ENCRYPTION, SCHEMABINDING
	for {
		s = strings.TrimSpace(s)
		if s == "" || hasPrefixFold(s, "AS") {
			return s
		}
		if s[0] == ',' {
			s = strings.TrimSpace(s[1:])
			continue
		}
		_, rest, ok := readIdent(s)
		if !ok {
			return s
		}
		s = rest
	}
}

func parseSelectColumns(selectSQL string, tables map[string]*schema.Table) []schema.Column {
	s := strings.TrimSpace(selectSQL)
	if hasPrefixFold(s, "WITH") {
		return nil
	}
	if !hasPrefixFold(s, "SELECT") {
		return nil
	}
	s = strings.TrimSpace(s[6:])
	if hasPrefixFold(s, "DISTINCT") {
		s = strings.TrimSpace(s[8:])
		if hasPrefixFold(s, "ON") {
			s = strings.TrimSpace(s[2:])
			if strings.HasPrefix(s, "(") {
				_, n, ok := readBalancedParens(s)
				if !ok {
					return nil
				}
				s = strings.TrimSpace(s[n:])
			}
		}
	}
	if hasPrefixFold(s, "TOP") {
		return nil
	}

	list, fromRest, ok := splitSelectFrom(s)
	if !ok {
		return nil
	}
	aliases := parseFromAliases(fromRest)
	items := splitCSV(list)
	var cols []schema.Column
	seen := map[string]struct{}{}
	starExpand := false
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if item == "*" {
			starExpand = true
			continue
		}
		if strings.HasSuffix(item, ".*") {
			qual := strings.TrimSpace(item[:len(item)-2])
			qual = unquoteIdent(qual)
			if tableName := aliases[strings.ToLower(qual)]; tableName != "" {
				if t, ok := tables[tableName]; ok {
					for _, c := range t.Columns {
						if _, dup := seen[c.Name]; dup {
							continue
						}
						seen[c.Name] = struct{}{}
						cc := c
						cc.IsPrimary = false
						cc.RefTable = ""
						cc.RefColumn = ""
						cols = append(cols, cc)
					}
				}
			}
			continue
		}
		col := columnFromSelectItem(item, tables, aliases)
		if col.Name == "" {
			continue
		}
		if _, dup := seen[col.Name]; dup {
			continue
		}
		seen[col.Name] = struct{}{}
		cols = append(cols, col)
	}
	if starExpand {
		base := firstFromTable(fromRest, aliases)
		if t, ok := tables[base]; ok {
			for _, c := range t.Columns {
				if _, dup := seen[c.Name]; dup {
					continue
				}
				seen[c.Name] = struct{}{}
				cc := c
				cc.IsPrimary = false
				cc.RefTable = ""
				cc.RefColumn = ""
				cols = append(cols, cc)
			}
		}
	}
	return cols
}

func splitSelectFrom(s string) (selectList, fromRest string, ok bool) {
	depth := 0
	inSingle, inDouble, inTick := false, false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inSingle:
			if c == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					i++
				} else {
					inSingle = false
				}
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			}
		case inTick:
			if c == '`' {
				inTick = false
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == '`':
			inTick = true
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		case depth == 0 && isWordAt(s, i, "FROM"):
			return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+4:]), true
		}
	}
	return "", "", false
}

func isWordAt(s string, i int, word string) bool {
	if i+len(word) > len(s) {
		return false
	}
	if !strings.EqualFold(s[i:i+len(word)], word) {
		return false
	}
	if i > 0 {
		prev := rune(s[i-1])
		if unicode.IsLetter(prev) || unicode.IsDigit(prev) || prev == '_' {
			return false
		}
	}
	if i+len(word) < len(s) {
		next := rune(s[i+len(word)])
		if unicode.IsLetter(next) || unicode.IsDigit(next) || next == '_' {
			return false
		}
	}
	return true
}

func parseFromAliases(fromRest string) map[string]string {
	aliases := map[string]string{}
	parts := splitTopLevelJoin(fromRest)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = trimJoinKeyword(p)
		onIdx := -1
		upper := strings.ToUpper(p)
		if i := strings.Index(upper, " ON "); i >= 0 {
			onIdx = i
		}
		if onIdx >= 0 {
			p = strings.TrimSpace(p[:onIdx])
		}
		table, alias := parseTableAlias(p)
		if table == "" {
			continue
		}
		aliases[strings.ToLower(table)] = table
		if alias != "" {
			aliases[strings.ToLower(alias)] = table
		}
	}
	return aliases
}

func splitTopLevelJoin(s string) []string {
	upper := strings.ToUpper(s)
	var parts []string
	depth := 0
	start := 0
	i := 0
	for i < len(s) {
		c := s[i]
		switch c {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 {
			for _, kw := range []string{" LEFT OUTER JOIN ", " RIGHT OUTER JOIN ", " FULL OUTER JOIN ", " LEFT JOIN ", " RIGHT JOIN ", " FULL JOIN ", " INNER JOIN ", " CROSS JOIN ", " JOIN "} {
				if i+len(kw) <= len(upper) && upper[i:i+len(kw)] == kw {
					parts = append(parts, s[start:i])
					start = i + 1 // keep JOIN word for trimJoinKeyword via remainder
					// actually include from JOIN
					start = i
					i += len(kw) - 1
					break
				}
			}
		}
		i++
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	if len(parts) == 0 {
		return []string{s}
	}
	return parts
}

func trimJoinKeyword(p string) string {
	upper := strings.ToUpper(strings.TrimSpace(p))
	p = strings.TrimSpace(p)
	for _, kw := range []string{"LEFT OUTER JOIN", "RIGHT OUTER JOIN", "FULL OUTER JOIN", "LEFT JOIN", "RIGHT JOIN", "FULL JOIN", "INNER JOIN", "CROSS JOIN", "JOIN"} {
		if strings.HasPrefix(upper, kw) {
			return strings.TrimSpace(p[len(kw):])
		}
	}
	if hasPrefixFold(p, "FROM") {
		return strings.TrimSpace(p[4:])
	}
	return p
}

func parseTableAlias(p string) (table, alias string) {
	p = strings.TrimSpace(p)
	p = cutAtSQLClause(p)
	fields := strings.Fields(p)
	if len(fields) == 0 {
		return "", ""
	}
	table = unquoteIdent(fields[0])
	switch len(fields) {
	case 1:
		return table, ""
	case 2:
		if isSQLKeyword(fields[1]) {
			return table, ""
		}
		return table, unquoteIdent(fields[1])
	default:
		if strings.EqualFold(fields[1], "AS") {
			return table, unquoteIdent(fields[2])
		}
		if isSQLKeyword(fields[1]) {
			return table, ""
		}
		return table, unquoteIdent(fields[1])
	}
}

func cutAtSQLClause(s string) string {
	upper := strings.ToUpper(s)
	cut := len(s)
	for _, kw := range []string{" WHERE ", " GROUP ", " HAVING ", " ORDER ", " LIMIT ", " OFFSET ", " UNION ", " EXCEPT ", " INTERSECT ", " WINDOW "} {
		if i := strings.Index(upper, kw); i >= 0 && i < cut {
			cut = i
		}
	}
	return strings.TrimSpace(s[:cut])
}

func firstFromTable(fromRest string, aliases map[string]string) string {
	table, _ := parseTableAlias(fromRest)
	if table != "" {
		return table
	}
	for _, v := range aliases {
		return v
	}
	return ""
}

func columnFromSelectItem(item string, tables map[string]*schema.Table, aliases map[string]string) schema.Column {
	alias, expr := splitSelectAlias(item)
	name := alias
	if name == "" {
		name = lastIdent(expr)
	}
	if name == "" || strings.ContainsAny(name, "()+*/") {
		return schema.Column{}
	}
	col := schema.Column{Name: name, SQLType: "text", Nullable: true}
	qual, colName := splitQualIdent(expr)
	if colName == "" {
		colName = name
	}
	var src *schema.Column
	if qual != "" {
		if tableName := aliases[strings.ToLower(unquoteIdent(qual))]; tableName != "" {
			src = findColumn(tables, tableName, colName)
		}
	} else {
		src = findColumnAnywhere(tables, colName)
	}
	if src != nil {
		col.SQLType = src.SQLType
		col.Nullable = src.Nullable
	}
	return col
}

func splitSelectAlias(item string) (alias, expr string) {
	upper := strings.ToUpper(item)
	if i := strings.LastIndex(upper, " AS "); i >= 0 {
		return unquoteIdent(strings.TrimSpace(item[i+4:])), strings.TrimSpace(item[:i])
	}
	fields := strings.Fields(item)
	if len(fields) >= 2 {
		last := fields[len(fields)-1]
		prev := fields[len(fields)-2]
		if !isSQLKeyword(prev) && !strings.ContainsAny(last, "().") {
			expr = strings.TrimSpace(strings.Join(fields[:len(fields)-1], " "))
			return unquoteIdent(last), expr
		}
	}
	return "", strings.TrimSpace(item)
}

func splitQualIdent(expr string) (qual, name string) {
	expr = strings.TrimSpace(expr)
	if strings.ContainsAny(expr, "()+*/") && !strings.Contains(expr, ".") {
		return "", ""
	}
	parts := strings.Split(expr, ".")
	if len(parts) == 1 {
		return "", unquoteIdent(parts[0])
	}
	return unquoteIdent(parts[len(parts)-2]), unquoteIdent(parts[len(parts)-1])
}

func lastIdent(expr string) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ""
	}
	parts := strings.FieldsFunc(expr, func(r rune) bool {
		return r == '.' || unicode.IsSpace(r)
	})
	if len(parts) == 0 {
		return ""
	}
	return unquoteIdent(parts[len(parts)-1])
}

func findColumn(tables map[string]*schema.Table, table, column string) *schema.Column {
	t, ok := tables[table]
	if !ok {
		for name, tt := range tables {
			if strings.EqualFold(name, table) {
				t = tt
				ok = true
				break
			}
		}
	}
	if !ok {
		return nil
	}
	for i := range t.Columns {
		if strings.EqualFold(t.Columns[i].Name, column) {
			c := t.Columns[i]
			return &c
		}
	}
	return nil
}

func findColumnAnywhere(tables map[string]*schema.Table, column string) *schema.Column {
	var found *schema.Column
	for _, t := range tables {
		for i := range t.Columns {
			if strings.EqualFold(t.Columns[i].Name, column) {
				if found != nil {
					return nil // ambiguous
				}
				c := t.Columns[i]
				found = &c
			}
		}
	}
	return found
}

func readIdent(s string) (ident, rest string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", s, false
	}
	switch s[0] {
	case '"', '`':
		q := s[0]
		end := strings.IndexByte(s[1:], q)
		if end < 0 {
			return "", s, false
		}
		raw := s[1 : 1+end]
		return unquoteIdent(raw), strings.TrimSpace(s[2+end:]), true
	case '[':
		end := strings.IndexByte(s, ']')
		if end < 0 {
			return "", s, false
		}
		return unquoteIdent(s[:end+1]), strings.TrimSpace(s[end+1:]), true
	}
	i := 0
	for i < len(s) {
		c := s[i]
		if unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c)) || c == '_' || c == '.' {
			i++
			continue
		}
		break
	}
	if i == 0 {
		return "", s, false
	}
	return unquoteIdent(s[:i]), strings.TrimSpace(s[i:]), true
}

func readBalancedParens(s string) (inner string, n int, ok bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "(") {
		return "", 0, false
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[1:i], i + 1, true
			}
		}
	}
	return "", 0, false
}

func takeUntilUnquotedSemicolon(s string) string {
	inSingle, inDouble, inTick := false, false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inSingle:
			if c == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					i++
				} else {
					inSingle = false
				}
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			}
		case inTick:
			if c == '`' {
				inTick = false
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == '`':
			inTick = true
		case c == ';':
			return strings.TrimSpace(s[:i])
		}
	}
	return strings.TrimSpace(s)
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

func isSQLKeyword(s string) bool {
	switch strings.ToUpper(s) {
	case "AS", "AND", "OR", "ON", "IN", "IS", "NOT", "NULL", "LIKE", "FROM", "JOIN", "WHERE", "GROUP", "HAVING", "ORDER", "LIMIT", "OFFSET":
		return true
	default:
		return false
	}
}
