package goql

import (
	"fmt"
	"strings"

	"github.com/yurimoinhos/ameego/internal/dialect"
)

// Builder composes dialect-native INSERT/UPDATE/DELETE SQL without reflection.
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
