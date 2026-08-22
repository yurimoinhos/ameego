package goql

import (
	"fmt"
	"strings"

	"github.com/yurimoinhos/ameego/internal/dialect"
)

// Query is a deferred SQL query — compile happens at SelectQuery/GetQuery time.
type Query struct {
	d       dialect.Dialect
	selects []selectItem
	from    Table
	joins   []joinItem
	wheres  []whereItem
	orders  []orderItem
	limit   int
	offset  int
	args    []any
}

type selectItem struct {
	col        Col
	nestPrefix string
}

type joinItem struct {
	kind      JoinType
	table     Table
	left      Col
	right     Col
	nestKey   string
	registered bool
}

type whereItem struct {
	col   Col
	cmp   Comparator
	value any
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
	for _, c := range cols {
		q.selects = append(q.selects, selectItem{col: c, nestPrefix: q.nestPrefixFor(c)})
	}
	return q
}

// From sets the root table.
func (q *Query) From(t Table) *Query {
	q.from = t
	return q
}

// Join adds a typed join. The joined table (not in From) is nested under its alias.
func (q *Query) Join(kind JoinType, a Col, b Col) *Query {
	joined := a.Table()
	if a.table == q.from.name {
		joined = b.Table()
	}
	q.joins = append(q.joins, joinItem{
		kind:    kind,
		table:   joined,
		left:    a,
		right:   b,
		nestKey: joined.Alias(),
	})
	return q
}

// Where adds a typed predicate.
func (q *Query) Where(col Col, cmp Comparator, value any) *Query {
	q.wheres = append(q.wheres, whereItem{col: col, cmp: cmp, value: value})
	return q
}

// OrderBy sorts by column.
func (q *Query) OrderBy(col Col, desc bool) *Query {
	q.orders = append(q.orders, orderItem{col: col, desc: desc})
	return q
}

// Limit restricts row count when n > 0.
func (q *Query) Limit(n int) *Query {
	q.limit = n
	return q
}

// Offset skips rows when n > 0.
func (q *Query) Offset(n int) *Query {
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
	if q.from.name == "" {
		panic("goql: From(table) is required")
	}
	if len(q.selects) == 0 {
		panic("goql: Select(...) is required")
	}

	args := append([]any(nil), q.args...)
	argN := len(args)

	var selectParts []string
	for _, s := range q.selects {
		expr := s.col.qualified(q.d)
		if s.nestPrefix != "" {
			alias := s.nestPrefix + s.col.name
			selectParts = append(selectParts, fmt.Sprintf("%s AS %s", expr, q.d.QuoteIdent(alias)))
		} else {
			selectParts = append(selectParts, expr)
		}
	}

	parts := []string{
		"SELECT " + strings.Join(selectParts, ", "),
		"FROM " + q.d.QuoteIdent(q.from.name) + " AS " + q.d.QuoteIdent(q.from.Alias()),
	}

	joined := map[string]struct{}{}
	for _, j := range q.joins {
		key := j.table.name
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
			if w.cmp == IsNull || w.cmp == IsNotNull {
				clauses = append(clauses, w.col.qualified(q.d)+" "+w.cmp.sql())
				continue
			}
			argN++
			clauses = append(clauses, fmt.Sprintf("%s %s %s", w.col.qualified(q.d), w.cmp.sql(), q.d.Placeholder(argN)))
			args = append(args, w.value)
		}
		parts = append(parts, "WHERE "+strings.Join(clauses, " AND "))
	}

	if len(q.orders) > 0 {
		orderParts := make([]string, len(q.orders))
		for i, o := range q.orders {
			dir := "ASC"
			if o.desc {
				dir = "DESC"
			}
			orderParts[i] = o.col.qualified(q.d) + " " + dir
		}
		parts = append(parts, "ORDER BY "+strings.Join(orderParts, ", "))
	}

	if q.limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", q.limit))
	}
	if q.offset > 0 {
		parts = append(parts, fmt.Sprintf("OFFSET %d", q.offset))
	}

	return strings.Join(parts, " "), args
}
