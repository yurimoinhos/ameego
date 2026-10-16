package goql

import (
	"fmt"
	"strings"

	"github.com/yurimoinhos/ameego/internal/dialect"
)

type pagingMode int

const (
	pagingNone pagingMode = iota
	pagingOffsetLimit
	pagingPage
)

type fromMode int

const (
	fromUnset fromMode = iota
	fromTable
	fromSubquery
)

// Query is a deferred SQL query — compilation happens at SelectQuery/GetQuery time.
type Query struct {
	d        dialect.Dialect
	selects  []selectItem
	from     Table
	fromSub  Subquery
	fromSet  fromMode
	joins    []joinItem
	wheres   []whereItem
	orders   []orderItem
	paging   pagingMode
	limit    int
	offset   int
	page     int
	pageSize int
	args     []any

	// MSSQL / T-SQL only — ignored (and rejected) on other dialects.
	mssqlTop    int
	mssqlHint   string
	mssqlOption string
	orderExprs  []orderExprItem
}

type orderExprItem struct {
	expr string
	desc bool
}

type selectKind int

const (
	selectColumn selectKind = iota
	selectSubquery
)

type selectItem struct {
	kind       selectKind
	col        Col
	nestPrefix string
	sub        *Query
	subAlias   string
}

type joinItem struct {
	kind       JoinType
	table      Table
	left       Col
	right      Col
	nestKey    string
	registered bool
}

type whereKind int

const (
	whereCompare whereKind = iota
	whereIn
	whereNotIn
	whereExists
	whereNotExists
	whereInValues
	whereNotInValues
)

type whereItem struct {
	kind  whereKind
	col   Col
	cmp   Comparator
	value any
	sub   *Query
}

type orderItem struct {
	col  Col
	desc bool
}

// NewQuery creates a query bound to a DB dialect.
func NewQuery(db *DB) *Query {
	return &Query{d: db.dialect}
}

// NewQueryFromDialect creates a query for tests and tooling.
func NewQueryFromDialect(d dialect.Dialect) *Query {
	return &Query{d: d}
}

// CompileForTest exposes SQL compilation for tests.
func (q *Query) CompileForTest() (string, []any) { return q.compile() }

// Select adds top-level or nested columns. Joined-table columns nest automatically.
func (q *Query) Select(cols ...Col) *Query {
	if len(cols) == 0 {
		panic("ameego: Select requires at least one column")
	}
	for _, c := range cols {
		q.selects = append(q.selects, selectItem{
			kind:       selectColumn,
			col:        c,
			nestPrefix: q.nestPrefixFor(c),
		})
	}
	return q
}

// SelectSub adds a scalar subquery column, e.g. SELECT (SELECT COUNT(*) ...) AS alias.
func (q *Query) SelectSub(sub *Query, alias string) *Query {
	if sub == nil {
		panic("ameego: SelectSub requires a non-nil subquery")
	}
	if alias == "" {
		panic("ameego: SelectSub requires a non-empty alias")
	}
	q.selects = append(q.selects, selectItem{
		kind:     selectSubquery,
		sub:      sub,
		subAlias: alias,
	})
	return q
}

// From sets the root table. Cannot be combined with FromSub.
func (q *Query) From(t Table) *Query {
	if q.fromSet == fromSubquery {
		panic("ameego: From cannot be used after FromSub")
	}
	if q.fromSet == fromTable {
		panic("ameego: From was already set")
	}
	q.fromSet = fromTable
	q.from = t
	return q
}

// FromSub uses a subquery as the FROM source. Cannot be combined with From.
func (q *Query) FromSub(sub Subquery) *Query {
	if q.fromSet == fromTable {
		panic("ameego: FromSub cannot be used after From")
	}
	if q.fromSet == fromSubquery {
		panic("ameego: FromSub was already set")
	}
	q.fromSet = fromSubquery
	q.fromSub = sub
	return q
}

// Join adds a typed join. The joined table (not already in FROM/JOIN) is nested under its alias.
// Use ColOf(TableAs(name, alias), col) for self-joins or custom aliases.
// FULL JOIN is rejected on MySQL/MariaDB (the engine has no FULL OUTER JOIN).
func (q *Query) Join(kind JoinType, a Col, b Col) *Query {
	if kind == JoinFull && q.d != nil && q.d.Name() == "mysql" {
		panic("ameego: FULL JOIN is not supported by MySQL/MariaDB")
	}
	joined := q.joinedSide(a, b)
	q.joins = append(q.joins, joinItem{
		kind:    kind,
		table:   joined,
		left:    a,
		right:   b,
		nestKey: joined.Alias(),
	})
	return q
}

func (q *Query) joinedSide(a, b Col) Table {
	aKnown := q.knowsAlias(a.alias)
	bKnown := q.knowsAlias(b.alias)
	switch {
	case aKnown && !bKnown:
		return b.Table()
	case bKnown && !aKnown:
		return a.Table()
	case aKnown:
		return b.Table()
	default:
		return a.Table()
	}
}

func (q *Query) knowsAlias(alias string) bool {
	if alias == "" {
		return false
	}
	if q.fromSet == fromTable && q.from.Alias() == alias {
		return true
	}
	if q.fromSet == fromSubquery && q.fromSub.alias == alias {
		return true
	}
	for _, j := range q.joins {
		if j.table.Alias() == alias {
			return true
		}
	}
	return false
}

// Where adds a typed predicate.
func (q *Query) Where(col Col, cmp Comparator, value any) *Query {
	if cmp == IsNull || cmp == IsNotNull {
		value = nil
	}
	q.wheres = append(q.wheres, whereItem{kind: whereCompare, col: col, cmp: cmp, value: value})
	return q
}

// WhereIn adds col IN (subquery).
func (q *Query) WhereIn(col Col, sub *Query) *Query {
	if sub == nil {
		panic("ameego: WhereIn requires a non-nil subquery")
	}
	q.wheres = append(q.wheres, whereItem{kind: whereIn, col: col, sub: sub})
	return q
}

// WhereNotIn adds col NOT IN (subquery).
func (q *Query) WhereNotIn(col Col, sub *Query) *Query {
	if sub == nil {
		panic("ameego: WhereNotIn requires a non-nil subquery")
	}
	q.wheres = append(q.wheres, whereItem{kind: whereNotIn, col: col, sub: sub})
	return q
}

// WhereInValues adds col IN (v1, v2, ...) for a literal value list.
// An empty list compiles to a constant-false predicate (col IN (NULL) semantics),
// so a zero-length filter never silently matches every row.
func (q *Query) WhereInValues(col Col, values []any) *Query {
	q.wheres = append(q.wheres, whereItem{kind: whereInValues, col: col, value: values})
	return q
}

// WhereNotInValues adds col NOT IN (v1, v2, ...) for a literal value list.
func (q *Query) WhereNotInValues(col Col, values []any) *Query {
	q.wheres = append(q.wheres, whereItem{kind: whereNotInValues, col: col, value: values})
	return q
}

// WhereExists adds EXISTS (subquery).
func (q *Query) WhereExists(sub *Query) *Query {
	if sub == nil {
		panic("ameego: WhereExists requires a non-nil subquery")
	}
	q.wheres = append(q.wheres, whereItem{kind: whereExists, sub: sub})
	return q
}

// WhereNotExists adds NOT EXISTS (subquery).
func (q *Query) WhereNotExists(sub *Query) *Query {
	if sub == nil {
		panic("ameego: WhereNotExists requires a non-nil subquery")
	}
	q.wheres = append(q.wheres, whereItem{kind: whereNotExists, sub: sub})
	return q
}

// OrderBy sorts by column.
func (q *Query) OrderBy(col Col, desc bool) *Query {
	q.orders = append(q.orders, orderItem{col: col, desc: desc})
	return q
}

// OrderByExpr sorts by a raw SQL expression (e.g. MSSQL VECTOR_DISTANCE(...)).
// Prefer dialect packages / generated helpers over hand-written expressions.
func (q *Query) OrderByExpr(expr string, desc bool) *Query {
	if strings.TrimSpace(expr) == "" {
		panic("ameego: OrderByExpr requires a non-empty expression")
	}
	q.orderExprs = append(q.orderExprs, orderExprItem{expr: expr, desc: desc})
	return q
}

// MSSQLTop applies T-SQL SELECT TOP (n). Panics unless the dialect is mssql.
func (q *Query) MSSQLTop(n int) *Query {
	q.requireMSSQL("MSSQLTop")
	if n <= 0 {
		panic("ameego: MSSQLTop requires n > 0")
	}
	q.mssqlTop = n
	return q
}

// MSSQLTableHint applies a WITH (hint) on the FROM table (e.g. NOLOCK).
func (q *Query) MSSQLTableHint(hint string) *Query {
	q.requireMSSQL("MSSQLTableHint")
	hint = strings.TrimSpace(hint)
	if hint == "" {
		panic("ameego: MSSQLTableHint requires a non-empty hint")
	}
	q.mssqlHint = hint
	return q
}

// MSSQLOption appends OPTION (...), e.g. RECOMPILE.
func (q *Query) MSSQLOption(option string) *Query {
	q.requireMSSQL("MSSQLOption")
	option = strings.TrimSpace(option)
	if option == "" {
		panic("ameego: MSSQLOption requires a non-empty option")
	}
	q.mssqlOption = option
	return q
}

func (q *Query) requireMSSQL(method string) {
	if q.d == nil || q.d.Name() != "mssql" {
		panic("ameego: " + method + " is only available for the mssql dialect")
	}
}

// Paginate applies page-based pagination (1-indexed page). Mutually exclusive with Offset and Limit.
func (q *Query) Paginate(page, size int) *Query {
	if q.paging == pagingOffsetLimit {
		panic("ameego: Paginate cannot be used with Offset or Limit")
	}
	if page < 1 {
		panic("ameego: Paginate page must be >= 1")
	}
	if size < 1 {
		panic("ameego: Paginate size must be >= 1")
	}
	q.paging = pagingPage
	q.page = page
	q.pageSize = size
	return q
}

// Limit restricts row count when n > 0. Mutually exclusive with Paginate.
func (q *Query) Limit(n int) *Query {
	if q.paging == pagingPage {
		panic("ameego: Limit cannot be used with Paginate")
	}
	if n > 0 {
		q.paging = pagingOffsetLimit
	}
	q.limit = n
	return q
}

// Offset skips rows when n > 0. Mutually exclusive with Paginate.
func (q *Query) Offset(n int) *Query {
	if q.paging == pagingPage {
		panic("ameego: Offset cannot be used with Paginate")
	}
	if n > 0 {
		q.paging = pagingOffsetLimit
	}
	q.offset = n
	return q
}

func (q *Query) nestPrefixFor(c Col) string {
	if q.from.name != "" && c.table == q.from.name {
		return ""
	}
	for _, j := range q.joins {
		if c.table == j.table.name {
			return j.nestKey + "."
		}
	}
	return c.alias + "."
}

func (q *Query) compile() (string, []any) {
	sql, args, _, err := q.compileAt(1)
	if err != nil {
		panic(err.Error())
	}
	return sql, args
}

func (q *Query) compileAt(argStart int) (string, []any, int, error) {
	if q.fromSet == fromUnset {
		return "", nil, argStart, fmt.Errorf("ameego: From(table) or FromSub(subquery) is required")
	}
	if len(q.selects) == 0 {
		return "", nil, argStart, fmt.Errorf("ameego: Select(...) or SelectSub(...) is required")
	}

	args := append([]any(nil), q.args...)
	argN := argStart + len(q.args) - 1

	var selectParts []string
	for _, s := range q.selects {
		switch s.kind {
		case selectSubquery:
			subSQL, subArgs, next, err := s.sub.compileAt(argN + 1)
			if err != nil {
				return "", nil, argStart, err
			}
			argN = next - 1
			args = append(args, subArgs...)
			selectParts = append(selectParts, fmt.Sprintf("(%s) AS %s", subSQL, q.d.QuoteIdent(s.subAlias)))
		default:
			expr := s.col.qualified(q.d)
			if s.nestPrefix != "" {
				alias := s.nestPrefix + s.col.name
				selectParts = append(selectParts, fmt.Sprintf("%s AS %s", expr, q.d.QuoteIdent(alias)))
			} else {
				selectParts = append(selectParts, expr)
			}
		}
	}

	selectHead := "SELECT "
	if q.mssqlTop > 0 {
		selectHead += fmt.Sprintf("TOP (%d) ", q.mssqlTop)
	}
	parts := []string{selectHead + strings.Join(selectParts, ", ")}

	switch q.fromSet {
	case fromSubquery:
		subSQL, subArgs, next, err := q.fromSub.q.compileAt(argN + 1)
		if err != nil {
			return "", nil, argStart, err
		}
		argN = next - 1
		args = append(args, subArgs...)
		parts = append(parts, fmt.Sprintf("FROM (%s) AS %s", subSQL, q.d.QuoteIdent(q.fromSub.alias)))
	default:
		from := "FROM " + q.d.QuoteIdent(q.from.name) + " AS " + q.d.QuoteIdent(q.from.Alias())
		if q.mssqlHint != "" {
			from += " WITH (" + q.mssqlHint + ")"
		}
		parts = append(parts, from)
	}

	joined := map[string]struct{}{}
	for _, j := range q.joins {
		key := j.table.Alias()
		if _, ok := joined[key]; ok {
			continue
		}
		joined[key] = struct{}{}
		parts = append(parts, fmt.Sprintf("%s %s AS %s ON %s = %s",
			j.kind.sql(),
			q.d.QuoteIdent(j.table.name),
			q.d.QuoteIdent(j.table.Alias()),
			j.left.qualified(q.d),
			j.right.qualified(q.d),
		))
	}

	if len(q.wheres) > 0 {
		clauses := make([]string, 0, len(q.wheres))
		for _, w := range q.wheres {
			switch w.kind {
			case whereIn, whereNotIn:
				subSQL, subArgs, next, err := w.sub.compileAt(argN + 1)
				if err != nil {
					return "", nil, argStart, err
				}
				argN = next - 1
				args = append(args, subArgs...)
				op := "IN"
				if w.kind == whereNotIn {
					op = "NOT IN"
				}
				clauses = append(clauses, fmt.Sprintf("%s %s (%s)", w.col.qualified(q.d), op, subSQL))
			case whereInValues, whereNotInValues:
				op := "IN"
				if w.kind == whereNotInValues {
					op = "NOT IN"
				}
				vals, _ := w.value.([]any)
				if len(vals) == 0 {
					// Empty list: never match (IN) / always match (NOT IN).
					if w.kind == whereInValues {
						clauses = append(clauses, "1 = 0")
					} else {
						clauses = append(clauses, "1 = 1")
					}
					continue
				}
				holders := make([]string, len(vals))
				for i, v := range vals {
					argN++
					holders[i] = q.d.Placeholder(argN)
					args = append(args, v)
				}
				clauses = append(clauses, fmt.Sprintf("%s %s (%s)", w.col.qualified(q.d), op, strings.Join(holders, ", ")))
			case whereExists, whereNotExists:
				subSQL, subArgs, next, err := w.sub.compileAt(argN + 1)
				if err != nil {
					return "", nil, argStart, err
				}
				argN = next - 1
				args = append(args, subArgs...)
				op := "EXISTS"
				if w.kind == whereNotExists {
					op = "NOT EXISTS"
				}
				clauses = append(clauses, fmt.Sprintf("%s (%s)", op, subSQL))
			default:
				if w.cmp == IsNull || w.cmp == IsNotNull {
					clauses = append(clauses, w.col.qualified(q.d)+" "+w.cmp.sql())
					continue
				}
				argN++
				clauses = append(clauses, fmt.Sprintf("%s %s %s", w.col.qualified(q.d), w.cmp.sql(), q.d.Placeholder(argN)))
				args = append(args, w.value)
			}
		}
		parts = append(parts, "WHERE "+strings.Join(clauses, " AND "))
	}

	if len(q.orders) > 0 || len(q.orderExprs) > 0 {
		orderParts := make([]string, 0, len(q.orders)+len(q.orderExprs))
		for _, o := range q.orders {
			dir := "ASC"
			if o.desc {
				dir = "DESC"
			}
			orderParts = append(orderParts, o.col.qualified(q.d)+" "+dir)
		}
		for _, o := range q.orderExprs {
			dir := "ASC"
			if o.desc {
				dir = "DESC"
			}
			orderParts = append(orderParts, o.expr+" "+dir)
		}
		parts = append(parts, "ORDER BY "+strings.Join(orderParts, ", "))
	}

	limit, offset := q.resolvedPaging()
	if limit > 0 {
		if q.d.Name() == "mssql" && len(q.orders) == 0 && len(q.orderExprs) == 0 {
			return "", nil, argStart, fmt.Errorf("ameego: MSSQL pagination requires OrderBy")
		}
		if clause := dialect.PaginationClause(q.d, limit, offset); clause != "" {
			parts = append(parts, clause)
		}
	}

	if q.mssqlOption != "" {
		parts = append(parts, "OPTION ("+q.mssqlOption+")")
	}

	return strings.Join(parts, " "), args, argN + 1, nil
}

func (q *Query) resolvedPaging() (limit, offset int) {
	switch q.paging {
	case pagingPage:
		return q.pageSize, (q.page - 1) * q.pageSize
	default:
		return q.limit, q.offset
	}
}
