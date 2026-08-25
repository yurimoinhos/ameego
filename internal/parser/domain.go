package parser

import (
	"strings"

	"github.com/yurimoinhos/ameego/internal/schema"
)

// extractDomains parses PostgreSQL CREATE DOMAIN statements.
func extractDomains(sql string) []schema.Domain {
	upper := strings.ToUpper(sql)
	var out []schema.Domain
	search := 0
	for {
		idx := indexCreateDomain(upper, search)
		if idx < 0 {
			return out
		}
		rest := strings.TrimSpace(sql[idx:])
		domain, consumed, ok := parseCreateDomain(rest)
		if !ok {
			search = idx + 6
			continue
		}
		out = append(out, domain)
		search = idx + consumed
	}
}

func indexCreateDomain(upper string, from int) int {
	if from >= len(upper) {
		return -1
	}
	pos := strings.Index(upper[from:], "CREATE")
	for pos >= 0 {
		abs := from + pos
		rest := strings.TrimSpace(upper[abs+6:])
		if strings.HasPrefix(rest, "DOMAIN") {
			return abs
		}
		next := strings.Index(upper[abs+6:], "CREATE")
		if next < 0 {
			return -1
		}
		return indexCreateDomain(upper, abs+6+next)
	}
	return -1
}

func parseCreateDomain(s string) (schema.Domain, int, bool) {
	orig := s
	s = strings.TrimSpace(s)
	if !hasPrefixFold(s, "CREATE") {
		return schema.Domain{}, 0, false
	}
	s = strings.TrimSpace(s[6:])
	if !hasPrefixFold(s, "DOMAIN") {
		return schema.Domain{}, 0, false
	}
	s = strings.TrimSpace(s[6:])
	if hasPrefixFold(s, "IF NOT EXISTS") {
		s = strings.TrimSpace(s[len("IF NOT EXISTS"):])
	}

	name, s, ok := readIdent(s)
	if !ok || name == "" {
		return schema.Domain{}, 0, false
	}
	s = strings.TrimSpace(s)
	if !hasPrefixFold(s, "AS") {
		return schema.Domain{}, 0, false
	}
	s = strings.TrimSpace(s[2:])

	baseType, s := readDomainBaseType(s)
	if baseType == "" {
		return schema.Domain{}, 0, false
	}

	domain := schema.Domain{
		Name:     name,
		BaseType: baseType,
	}

	for {
		s = strings.TrimSpace(s)
		if s == "" || s[0] == ';' {
			break
		}
		switch {
		case hasPrefixFold(s, "COLLATE"):
			s = strings.TrimSpace(s[7:])
			collate, rest, ok := readIdent(s)
			if !ok {
				return schema.Domain{}, 0, false
			}
			domain.Collate = collate
			s = rest
		case hasPrefixFold(s, "DEFAULT"):
			s = strings.TrimSpace(s[7:])
			def, rest := readDomainDefault(s)
			domain.Default = def
			s = rest
		case hasPrefixFold(s, "NOT NULL"):
			domain.NotNull = true
			s = strings.TrimSpace(s[8:])
		case hasPrefixFold(s, "NULL"):
			domain.NotNull = false
			s = strings.TrimSpace(s[4:])
		case hasPrefixFold(s, "CONSTRAINT"):
			s = strings.TrimSpace(s[10:])
			_, s, ok = readIdent(s)
			if !ok {
				return schema.Domain{}, 0, false
			}
		case hasPrefixFold(s, "CHECK"):
			s = strings.TrimSpace(s[5:])
			if !strings.HasPrefix(s, "(") {
				return schema.Domain{}, 0, false
			}
			inner, n, ok := readBalancedParens(s)
			if !ok {
				return schema.Domain{}, 0, false
			}
			domain.Check = strings.TrimSpace(inner)
			s = strings.TrimSpace(s[n:])
		default:
			// stop at unrecognized token (likely next statement without semicolon)
			if isDomainTerminator(s) {
				break
			}
			// consume one token to avoid infinite loop
			_, rest, ok := readIdent(s)
			if !ok {
				break
			}
			s = rest
			continue
		}
	}

	rest := s
	if strings.HasPrefix(strings.TrimSpace(rest), ";") {
		rest = strings.TrimSpace(rest)[1:]
	}
	consumed := len(orig) - len(rest)
	if consumed < 0 {
		consumed = len(orig)
	}
	return domain, consumed, true
}

func readDomainBaseType(s string) (baseType, rest string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", s
	}
	var b strings.Builder
	for {
		s = strings.TrimSpace(s)
		if s == "" || s[0] == ';' || isDomainConstraintKeyword(s) {
			break
		}
		if strings.HasPrefix(s, "(") {
			inner, n, ok := readBalancedParens(s)
			if !ok {
				break
			}
			b.WriteByte('(')
			b.WriteString(inner)
			b.WriteByte(')')
			s = strings.TrimSpace(s[n:])
			continue
		}
		ident, rem, ok := readIdent(s)
		if !ok {
			break
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(ident)
		s = rem

		up := strings.ToUpper(b.String())
		switch up {
		case "CHARACTER", "DOUBLE", "BIT", "TIMESTAMP", "TIME", "INTERVAL",
			"TIMESTAMP WITH", "TIMESTAMP WITHOUT", "TIME WITH", "TIME WITHOUT",
			"TIMESTAMP WITH TIME", "TIMESTAMP WITHOUT TIME", "TIME WITH TIME", "TIME WITHOUT TIME":
			continue
		}
		peek := strings.TrimSpace(s)
		if strings.HasPrefix(peek, "(") {
			continue
		}
		break
	}
	if b.Len() == 0 {
		return "", s
	}
	return b.String(), s
}

func readDomainDefault(s string) (def, rest string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", s
	}
	if strings.HasPrefix(s, "'") || strings.HasPrefix(s, "\"") {
		q := s[0]
		i := 1
		for i < len(s) {
			if s[i] == q {
				if i+1 < len(s) && s[i+1] == q {
					i += 2
					continue
				}
				return s[:i+1], strings.TrimSpace(s[i+1:])
			}
			i++
		}
		return s, ""
	}
	if strings.HasPrefix(s, "(") {
		inner, n, ok := readBalancedParens(s)
		if ok {
			return "(" + inner + ")", strings.TrimSpace(s[n:])
		}
	}
	// bare literal / function call until next keyword
	end := 0
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i:])
			}
		}
		if depth == 0 && i > 0 {
			rest := s[i:]
			if isDomainConstraintKeyword(rest) {
				return strings.TrimSpace(s[:i]), strings.TrimSpace(rest)
			}
		}
		end = i + 1
	}
	return strings.TrimSpace(s[:end]), ""
}

func isDomainConstraintKeyword(s string) bool {
	s = strings.TrimSpace(s)
	for _, kw := range []string{"COLLATE", "DEFAULT", "NOT NULL", "NULL", "CONSTRAINT", "CHECK"} {
		if hasPrefixFold(s, kw) {
			if len(s) == len(kw) {
				return true
			}
			r := rune(s[len(kw)])
			if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '(' || r == ';' {
				return true
			}
		}
	}
	return false
}

func isDomainTerminator(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || s[0] == ';' || hasPrefixFold(s, "CREATE") || hasPrefixFold(s, "ALTER")
}
