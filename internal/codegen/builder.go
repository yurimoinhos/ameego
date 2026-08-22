package codegen

import (
	"fmt"
	"strings"

	"github.com/yurimoinhos/ameego/internal/dialect"
)

// Builder generates native SQL strings at codegen time.
type Builder struct {
	d dialect.Dialect
}

// NewBuilder creates a codegen SQL builder for a dialect.
func NewBuilder(d dialect.Dialect) *Builder {
	return &Builder{d: d}
}

// SelectAll builds SELECT col1, col2 FROM table.
func (b *Builder) SelectAll(table string, columns []string) string {
	quoted := quoteList(b.d, columns)
	return fmt.Sprintf("SELECT %s FROM %s", quoted, b.d.QuoteIdent(table))
}

// SelectByPK builds SELECT ... WHERE pk = placeholder.
func (b *Builder) SelectByPK(table string, columns []string, pkColumn string) string {
	return fmt.Sprintf("%s WHERE %s = %s",
		b.SelectAll(table, columns),
		b.d.QuoteIdent(pkColumn),
		b.d.Placeholder(1),
	)
}

// Insert builds INSERT INTO table (cols) VALUES (ph).
func (b *Builder) Insert(table string, columns []string) string {
	quoted := quoteList(b.d, columns)
	holders := make([]string, len(columns))
	for i := range columns {
		holders[i] = b.d.Placeholder(i + 1)
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		b.d.QuoteIdent(table),
		quoted,
		strings.Join(holders, ", "),
	)
}

// UpdateByPK builds UPDATE table SET col = ph ... WHERE pk = ph.
func (b *Builder) UpdateByPK(table string, columns []string, pkColumn string) string {
	sets := make([]string, 0, len(columns))
	n := 1
	for _, col := range columns {
		if col == pkColumn {
			continue
		}
		sets = append(sets, fmt.Sprintf("%s = %s", b.d.QuoteIdent(col), b.d.Placeholder(n)))
		n++
	}
	return fmt.Sprintf("UPDATE %s SET %s WHERE %s = %s",
		b.d.QuoteIdent(table),
		strings.Join(sets, ", "),
		b.d.QuoteIdent(pkColumn),
		b.d.Placeholder(n),
	)
}

// DeleteByPK builds DELETE FROM table WHERE pk = ph.
func (b *Builder) DeleteByPK(table string, pkColumn string) string {
	return fmt.Sprintf("DELETE FROM %s WHERE %s = %s",
		b.d.QuoteIdent(table),
		b.d.QuoteIdent(pkColumn),
		b.d.Placeholder(1),
	)
}

func quoteList(d dialect.Dialect, columns []string) string {
	quoted := make([]string, len(columns))
	for i, c := range columns {
		quoted[i] = d.QuoteIdent(c)
	}
	return strings.Join(quoted, ", ")
}
