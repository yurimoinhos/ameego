package goql

import "github.com/yurimoinhos/ameego/internal/dialect"

// JoinType is a SQL join kind.
type JoinType int

const (
	JoinInner JoinType = iota
	JoinLeft
	JoinRight
	JoinFull
)

func (j JoinType) sql() string {
	switch j {
	case JoinLeft:
		return "LEFT JOIN"
	case JoinRight:
		return "RIGHT JOIN"
	case JoinFull:
		return "FULL JOIN"
	default:
		return "INNER JOIN"
	}
}

// Comparator is a WHERE comparison operator.
type Comparator int

const (
	Equals Comparator = iota
	NotEquals
	GreaterThan
	GreaterOrEqual
	LessThan
	LessOrEqual
	Like
	IsNull
	IsNotNull
)

func (c Comparator) sql() string {
	switch c {
	case NotEquals:
		return "<>"
	case GreaterThan:
		return ">"
	case GreaterOrEqual:
		return ">="
	case LessThan:
		return "<"
	case LessOrEqual:
		return "<="
	case Like:
		return "LIKE"
	case IsNull:
		return "IS NULL"
	case IsNotNull:
		return "IS NOT NULL"
	default:
		return "="
	}
}

// Table identifies a database table in the DSL.
type Table struct {
	name  string
	alias string
}

// TableOf creates a table reference.
func TableOf(name string) Table {
	return Table{name: name, alias: defaultAlias(name)}
}

// TableAs creates a table reference with an explicit SQL alias (e.g. subquery FROM aliases).
func TableAs(name, alias string) Table {
	return Table{name: name, alias: alias}
}

// Name returns the physical table name.
func (t Table) Name() string { return t.name }

// Alias returns the SQL alias used in JOIN/SELECT.
func (t Table) Alias() string {
	if t.alias != "" {
		return t.alias
	}
	return t.name
}

// Col is a compile-time typed column reference (e.g. Users.Ref.Id).
type Col struct {
	table string
	name  string
	alias string
}

// ColOf creates a column reference bound to a table.
func ColOf(table Table, column string) Col {
	return Col{table: table.name, name: column, alias: table.Alias()}
}

// Table returns the column's table reference.
func (c Col) Table() Table {
	return Table{name: c.table, alias: c.alias}
}

// Name returns the column name.
func (c Col) Name() string { return c.name }

func (c Col) qualified(d dialect.Dialect) string {
	return d.QuoteIdent(c.alias) + "." + d.QuoteIdent(c.name)
}

func defaultAlias(table string) string {
	if i := lastDot(table); i >= 0 {
		table = table[i+1:]
	}
	return table
}

func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}
