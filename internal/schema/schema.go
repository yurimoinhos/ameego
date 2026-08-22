package schema

// Schema holds the full database schema extracted from migration files.
type Schema struct {
	Tables []Table
}

// Table represents a database table.
type Table struct {
	Name       string
	Columns    []Column
	PrimaryKey []string
	Indexes    []Index
}

// Column represents a table column.
type Column struct {
	Name       string
	SQLType    string
	GoType     string
	Nullable   bool
	Default    string
	IsPrimary  bool
	IsAutoIncr bool
	RefTable   string
	RefColumn  string
}

// Index represents a table index.
type Index struct {
	Name    string
	Columns []string
	Unique  bool
}

// PrimaryKeyColumn returns the primary key column if there is exactly one.
func (t Table) PrimaryKeyColumn() (Column, bool) {
	if len(t.PrimaryKey) != 1 {
		return Column{}, false
	}
	pkName := t.PrimaryKey[0]
	for _, col := range t.Columns {
		if col.Name == pkName {
			return col, true
		}
	}
	return Column{}, false
}
