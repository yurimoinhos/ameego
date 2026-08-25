package schema

import "strings"

const (
	KindTable = "table"
	KindView  = "view"
)

// Schema holds the full database schema extracted from migration files.
type Schema struct {
	Tables    []Table
	Relations []Relation
	Domains   []Domain // PostgreSQL CREATE DOMAIN (ignored by other dialects at codegen)
}

// Table represents a database table or view.
type Table struct {
	Name       string
	Kind       string // KindTable or KindView
	Columns    []Column
	PrimaryKey []string
	Indexes    []Index
	Temporary  bool   // MySQL/MariaDB CREATE TEMPORARY TABLE
	Engine     string // MySQL/MariaDB ENGINE=InnoDB (etc.)
	Options    string // raw trailing table options from CREATE TABLE
}

// Domain is a PostgreSQL DOMAIN type.
type Domain struct {
	Name     string
	BaseType string
	NotNull  bool
	Default  string
	Check    string
	Collate  string
}

// Relation is a joinable link between two tables or views (FK or declared).
type Relation struct {
	FromTable   string
	FromColumn  string
	ToTable     string
	ToColumn    string
	JoinKind    string // inner, left, right, full
	NestedField string
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
	Domain     string // PostgreSQL domain type name when SQLType is a DOMAIN
	VectorDim  int    // MSSQL VECTOR(n) dimension; 0 if not a vector column
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

// IsView reports whether this relation is a database view.
func (t Table) IsView() bool {
	return t.Kind == KindView
}

// ColumnNamed returns a column by name.
func (t Table) ColumnNamed(name string) (Column, bool) {
	for _, col := range t.Columns {
		if col.Name == name {
			return col, true
		}
	}
	return Column{}, false
}

// DomainNamed returns a domain by name (case-insensitive; schema-qualified ok).
func (s Schema) DomainNamed(name string) (Domain, bool) {
	want := stripSchemaIdent(name)
	for _, d := range s.Domains {
		if strings.EqualFold(stripSchemaIdent(d.Name), want) {
			return d, true
		}
	}
	return Domain{}, false
}

func stripSchemaIdent(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}
