package parser

import (
	"strings"
	"unicode"
)

type createTableStmt struct {
	Temporary bool
	Name      string
	Body      string
	Options   string
}

// extractCreateTables finds CREATE [TEMPORARY] TABLE statements with balanced parentheses.
func extractCreateTables(sql string) []createTableStmt {
	upper := strings.ToUpper(sql)
	var out []createTableStmt
	search := 0
	for {
		idx := indexCreateTable(upper, search)
		if idx < 0 {
			return out
		}
		rest := strings.TrimSpace(sql[idx:])
		stmt, consumed, ok := parseCreateTableStmt(rest)
		if !ok {
			search = idx + 6
			continue
		}
		out = append(out, stmt)
		search = idx + consumed
	}
}

func indexCreateTable(upper string, from int) int {
	if from >= len(upper) {
		return -1
	}
	pos := strings.Index(upper[from:], "CREATE")
	for pos >= 0 {
		abs := from + pos
		rest := strings.TrimSpace(upper[abs+6:])
		if strings.HasPrefix(rest, "TEMPORARY") {
			rest = strings.TrimSpace(rest[len("TEMPORARY"):])
		} else if strings.HasPrefix(rest, "TEMP") {
			rest = strings.TrimSpace(rest[4:])
		}
		if strings.HasPrefix(rest, "TABLE") {
			return abs
		}
		next := strings.Index(upper[abs+6:], "CREATE")
		if next < 0 {
			return -1
		}
		from = abs + 6 + next
		pos = 0
		return indexCreateTable(upper, from)
	}
	return -1
}

func parseCreateTableStmt(s string) (createTableStmt, int, bool) {
	orig := s
	s = strings.TrimSpace(s)
	if !hasPrefixFold(s, "CREATE") {
		return createTableStmt{}, 0, false
	}
	s = strings.TrimSpace(s[6:])

	temporary := false
	if hasPrefixFold(s, "TEMPORARY") {
		temporary = true
		s = strings.TrimSpace(s[len("TEMPORARY"):])
	} else if hasPrefixFold(s, "TEMP") {
		temporary = true
		s = strings.TrimSpace(s[4:])
	}
	if !hasPrefixFold(s, "TABLE") {
		return createTableStmt{}, 0, false
	}
	s = strings.TrimSpace(s[5:])
	if hasPrefixFold(s, "IF NOT EXISTS") {
		s = strings.TrimSpace(s[len("IF NOT EXISTS"):])
	}

	name, s, ok := readIdent(s)
	if !ok || name == "" {
		return createTableStmt{}, 0, false
	}
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "(") {
		return createTableStmt{}, 0, false
	}
	body, n, ok := readBalancedParens(s)
	if !ok {
		return createTableStmt{}, 0, false
	}
	s = strings.TrimSpace(s[n:])
	options, rem := takeTableOptions(s)
	consumed := len(orig) - len(rem)
	if consumed < 0 {
		consumed = len(orig)
	}
	return createTableStmt{
		Temporary: temporary,
		Name:      name,
		Body:      body,
		Options:   options,
	}, consumed, true
}

func takeTableOptions(s string) (options, rest string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", s
	}
	end := 0
	inSingle, inDouble, inTick := false, false, false
	depth := 0
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
		case depth == 0 && c == ';':
			end = i
			opts := strings.TrimSpace(s[:end])
			return opts, strings.TrimSpace(s[end+1:])
		}
	}
	opts := strings.TrimSpace(s)
	return opts, ""
}

func parseMySQLEngine(options string) string {
	if options == "" {
		return ""
	}
	upper := strings.ToUpper(options)
	idx := strings.Index(upper, "ENGINE")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(options[idx+len("ENGINE"):])
	if strings.HasPrefix(rest, "=") {
		rest = strings.TrimSpace(rest[1:])
	}
	ident, _, ok := readIdent(rest)
	if !ok {
		return ""
	}
	return ident
}

func parseVectorDim(sqlType string) int {
	t := strings.TrimSpace(sqlType)
	if !hasPrefixFold(t, "VECTOR") {
		return 0
	}
	rest := strings.TrimSpace(t[len("VECTOR"):])
	if !strings.HasPrefix(rest, "(") {
		return 0
	}
	inner, _, ok := readBalancedParens(rest)
	if !ok {
		return 0
	}
	inner = strings.TrimSpace(inner)
	n := 0
	for _, r := range inner {
		if !unicode.IsDigit(r) {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
