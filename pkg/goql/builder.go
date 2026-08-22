package goql

import (
	"fmt"
	"strings"

	"github.com/yurimoinhos/ameego/internal/dialect"
)

// Builder composes dialect-native SQL without reflection.
type Builder struct {
	d dialect.Dialect
}

// NewBuilder creates a SQL builder for the DB dialect.
func (db *DB) NewBuilder() *Builder {
	return &Builder{d: db.dialect}
}

// NewBuilderFromDialect creates a builder for tests and codegen helpers.
func NewBuilderFromDialect(d interface{ Name() string }) *Builder {
	parsed, _ := dialect.ParseDialect(d.Name())
	return &Builder{d: parsed}
}

// Select starts a SELECT statement.
func (b *Builder) Select(columns ...string) *SelectBuilder {
	quoted := make([]string, len(columns))
	for i, c := range columns {
		quoted[i] = b.d.QuoteIdent(c)
	}
	return &SelectBuilder{
		b:       b,
		columns: quoted,
	}
}

// SelectBuilder builds SELECT queries.
type SelectBuilder struct {
	b       *Builder
	columns []string
	table   string
	where   []whereClause
	orderBy string
	limit   int
	offset  int
	args    []any
}

type whereClause struct {
	sql string
}

// From sets the source table.
func (s *SelectBuilder) From(table string) *SelectBuilder {
	s.table = s.b.d.QuoteIdent(table)
	return s
}

// WhereEq adds column = placeholder.
func (s *SelectBuilder) WhereEq(column string, value any) *SelectBuilder {
	n := len(s.args) + 1
	s.where = append(s.where, whereClause{
		sql: fmt.Sprintf("%s = %s", s.b.d.QuoteIdent(column), s.b.d.Placeholder(n)),
	})
	s.args = append(s.args, value)
	return s
}

// WhereRaw appends a raw WHERE fragment with its args.
func (s *SelectBuilder) WhereRaw(fragment string, args ...any) *SelectBuilder {
	s.where = append(s.where, whereClause{sql: fragment})
	s.args = append(s.args, args...)
	return s
}

// OrderBy adds ORDER BY column.
func (s *SelectBuilder) OrderBy(column string, desc bool) *SelectBuilder {
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	s.orderBy = fmt.Sprintf("%s %s", s.b.d.QuoteIdent(column), dir)
	return s
}

// Limit sets LIMIT n when n > 0.
func (s *SelectBuilder) Limit(n int) *SelectBuilder {
	s.limit = n
	return s
}

// Offset sets OFFSET n when n > 0.
func (s *SelectBuilder) Offset(n int) *SelectBuilder {
	s.offset = n
	return s
}

// SQL returns the native SQL and bound arguments.
func (s *SelectBuilder) SQL() (string, []any) {
	var parts []string
	parts = append(parts, "SELECT "+strings.Join(s.columns, ", "))
	parts = append(parts, "FROM "+s.table)
	if len(s.where) > 0 {
		clauses := make([]string, len(s.where))
		for i, w := range s.where {
			clauses[i] = w.sql
		}
		parts = append(parts, "WHERE "+strings.Join(clauses, " AND "))
	}
	if s.orderBy != "" {
		parts = append(parts, "ORDER BY "+s.orderBy)
	}
	if s.limit > 0 {
		parts = append(parts, fmt.Sprintf("LIMIT %d", s.limit))
	}
	if s.offset > 0 {
		parts = append(parts, fmt.Sprintf("OFFSET %d", s.offset))
	}
	return strings.Join(parts, " "), append([]any(nil), s.args...)
}

// InsertBuilder builds INSERT statements.
type InsertBuilder struct {
	b       *Builder
	table   string
	columns []string
	args    []any
}

// Insert starts an INSERT statement.
func (b *Builder) Insert(table string, columns []string) *InsertBuilder {
	quoted := make([]string, len(columns))
	for i, c := range columns {
		quoted[i] = b.d.QuoteIdent(c)
	}
	return &InsertBuilder{
		b:       b,
		table:   b.d.QuoteIdent(table),
		columns: quoted,
	}
}

// Values sets INSERT values in column order.
func (i *InsertBuilder) Values(values ...any) *InsertBuilder {
	i.args = append(i.args, values...)
	return i
}

// SQL returns INSERT SQL and args.
func (i *InsertBuilder) SQL() (string, []any) {
	holders := make([]string, len(i.columns))
	for n := range i.columns {
		holders[n] = i.b.d.Placeholder(n + 1)
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		i.table,
		strings.Join(i.columns, ", "),
		strings.Join(holders, ", "),
	)
	return query, append([]any(nil), i.args...)
}

// UpdateBuilder builds UPDATE statements.
type UpdateBuilder struct {
	b      *Builder
	table  string
	sets   []string
	where  string
	args   []any
}

// Update starts an UPDATE statement.
func (b *Builder) Update(table string) *UpdateBuilder {
	return &UpdateBuilder{
		b:     b,
		table: b.d.QuoteIdent(table),
	}
}

// SetEq adds column = placeholder.
func (u *UpdateBuilder) SetEq(column string, value any) *UpdateBuilder {
	n := len(u.args) + 1
	u.sets = append(u.sets, fmt.Sprintf("%s = %s", u.b.d.QuoteIdent(column), u.b.d.Placeholder(n)))
	u.args = append(u.args, value)
	return u
}

// WhereEq adds a WHERE column = placeholder after SET clauses.
func (u *UpdateBuilder) WhereEq(column string, value any) *UpdateBuilder {
	n := len(u.args) + 1
	u.where = fmt.Sprintf("%s = %s", u.b.d.QuoteIdent(column), u.b.d.Placeholder(n))
	u.args = append(u.args, value)
	return u
}

// SQL returns UPDATE SQL and args.
func (u *UpdateBuilder) SQL() (string, []any) {
	query := fmt.Sprintf("UPDATE %s SET %s", u.table, strings.Join(u.sets, ", "))
	if u.where != "" {
		query += " WHERE " + u.where
	}
	return query, append([]any(nil), u.args...)
}

// DeleteBuilder builds DELETE statements.
type DeleteBuilder struct {
	b     *Builder
	table string
	where string
	args  []any
}

// Delete starts a DELETE statement.
func (b *Builder) Delete(table string) *DeleteBuilder {
	return &DeleteBuilder{
		b:     b,
		table: b.d.QuoteIdent(table),
	}
}

// WhereEq adds WHERE column = placeholder.
func (d *DeleteBuilder) WhereEq(column string, value any) *DeleteBuilder {
	n := len(d.args) + 1
	d.where = fmt.Sprintf("%s = %s", d.b.d.QuoteIdent(column), d.b.d.Placeholder(n))
	d.args = append(d.args, value)
	return d
}

// SQL returns DELETE SQL and args.
func (d *DeleteBuilder) SQL() (string, []any) {
	query := "DELETE FROM " + d.table
	if d.where != "" {
		query += " WHERE " + d.where
	}
	return query, append([]any(nil), d.args...)
}

// NormalizePlaceholders rewrites positional placeholders to the target dialect.
func NormalizePlaceholders(sql, dialectName string) (string, error) {
	d, err := dialect.ParseDialect(dialectName)
	if err != nil {
		return "", err
	}
	switch d.Name() {
	case "postgres":
		return normalizePostgres(sql), nil
	case "mssql":
		return normalizeMSSQL(sql), nil
	default:
		return normalizeQuestion(sql), nil
	}
}

func normalizePostgres(sql string) string {
	// already $n or rewrite ? / @pn
	if strings.Contains(sql, "$") {
		return sql
	}
	n := 0
	var b strings.Builder
	for i := 0; i < len(sql); i++ {
		if sql[i] == '?' {
			n++
			b.WriteString(fmt.Sprintf("$%d", n))
			continue
		}
		b.WriteByte(sql[i])
	}
	return b.String()
}

func normalizeMSSQL(sql string) string {
	if strings.Contains(strings.ToLower(sql), "@p") {
		return sql
	}
	n := 0
	var b strings.Builder
	for i := 0; i < len(sql); i++ {
		if sql[i] == '?' || (sql[i] == '$' && i+1 < len(sql) && sql[i+1] >= '0' && sql[i+1] <= '9') {
			if sql[i] == '$' {
				for i+1 < len(sql) && sql[i+1] >= '0' && sql[i+1] <= '9' {
					i++
				}
			}
			n++
			b.WriteString(fmt.Sprintf("@p%d", n))
			continue
		}
		b.WriteByte(sql[i])
	}
	return b.String()
}

func normalizeQuestion(sql string) string {
	n := 0
	var b strings.Builder
	for i := 0; i < len(sql); i++ {
		if sql[i] == '$' && i+1 < len(sql) && sql[i+1] >= '0' && sql[i+1] <= '9' {
			for i+1 < len(sql) && sql[i+1] >= '0' && sql[i+1] <= '9' {
				i++
			}
			b.WriteByte('?')
			n++
			continue
		}
		if strings.HasPrefix(strings.ToLower(sql[i:]), "@p") {
			j := i + 2
			for j < len(sql) && sql[j] >= '0' && sql[j] <= '9' {
				j++
			}
			i = j - 1
			b.WriteByte('?')
			n++
			continue
		}
		b.WriteByte(sql[i])
	}
	return b.String()
}
