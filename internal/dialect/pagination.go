package dialect

import "fmt"

// PaginationClause returns dialect-native LIMIT/OFFSET SQL.
// limit and offset must be >= 0; limit > 0 is required for a non-empty clause.
func PaginationClause(d Dialect, limit, offset int) string {
	if limit <= 0 {
		return ""
	}
	switch d.Name() {
	case "mssql":
		return fmt.Sprintf("OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit)
	default:
		if offset > 0 {
			return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
		}
		return fmt.Sprintf("LIMIT %d", limit)
	}
}
