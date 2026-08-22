package goql

// Subquery wraps a nested SELECT for use in FROM, SELECT, or WHERE clauses.
type Subquery struct {
	q     *Query
	alias string
}

// AsSubquery marks a query as a subquery with the given SQL alias.
func AsSubquery(q *Query, alias string) Subquery {
	if q == nil {
		panic("ameego: AsSubquery requires a non-nil query")
	}
	if alias == "" {
		panic("ameego: AsSubquery requires a non-empty alias")
	}
	return Subquery{q: q, alias: alias}
}
