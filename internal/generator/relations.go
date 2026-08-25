package generator

import (
	"fmt"
	"strings"

	"github.com/yurimoinhos/ameego/internal/naming"
	"github.com/yurimoinhos/ameego/internal/schema"
)

func mergeDeclaredRelations(s *schema.Schema, extra []RelationSpec) error {
	if s == nil {
		return nil
	}
	seen := map[string]struct{}{}
	for _, rel := range s.Relations {
		seen[relationKey(rel)] = struct{}{}
	}
	for i, spec := range extra {
		fromTable, ok := resolveRelationTable(s, spec.From)
		if !ok {
			return fmt.Errorf("relations[%d]: from %q not found in schema", i, spec.From)
		}
		toTable, ok := resolveRelationTable(s, spec.To)
		if !ok {
			return fmt.Errorf("relations[%d]: to %q not found in schema", i, spec.To)
		}
		fromCol := spec.FromColumn
		toCol := spec.ToColumn
		if _, ok := fromTable.ColumnNamed(fromCol); !ok {
			if col, ok := columnNamedFold(fromTable, fromCol); ok {
				fromCol = col.Name
			} else {
				return fmt.Errorf("relations[%d]: column %q not found on %s", i, spec.FromColumn, fromTable.Name)
			}
		}
		if _, ok := toTable.ColumnNamed(toCol); !ok {
			if col, ok := columnNamedFold(toTable, toCol); ok {
				toCol = col.Name
			} else {
				return fmt.Errorf("relations[%d]: column %q not found on %s", i, spec.ToColumn, toTable.Name)
			}
		}
		joinKind := strings.ToLower(strings.TrimSpace(spec.Join))
		if joinKind == "" {
			joinKind = "left"
			if col, ok := fromTable.ColumnNamed(fromCol); ok && !col.Nullable {
				joinKind = "inner"
			}
		}
		rel := schema.Relation{
			FromTable:   fromTable.Name,
			FromColumn:  fromCol,
			ToTable:     toTable.Name,
			ToColumn:    toCol,
			JoinKind:    joinKind,
			NestedField: strings.TrimSpace(spec.Field),
		}
		key := relationKey(rel)
		if _, dup := seen[key]; dup {
			for i := range s.Relations {
				if relationKey(s.Relations[i]) == key {
					if rel.JoinKind != "" {
						s.Relations[i].JoinKind = rel.JoinKind
					}
					if rel.NestedField != "" {
						s.Relations[i].NestedField = rel.NestedField
					}
					break
				}
			}
			continue
		}
		seen[key] = struct{}{}
		s.Relations = append(s.Relations, rel)
	}
	return nil
}

func relationKey(r schema.Relation) string {
	return strings.ToLower(r.FromTable + "\x00" + r.FromColumn + "\x00" + r.ToTable + "\x00" + r.ToColumn)
}

func resolveRelationTable(s *schema.Schema, name string) (schema.Table, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	if want == "" {
		return schema.Table{}, false
	}
	for _, t := range s.Tables {
		if strings.EqualFold(t.Name, want) {
			return t, true
		}
		if strings.EqualFold(naming.StructName(t.Name), want) {
			return t, true
		}
		if strings.EqualFold(naming.PluralStructName(t.Name), want) {
			return t, true
		}
	}
	return schema.Table{}, false
}

func columnNamedFold(t schema.Table, name string) (schema.Column, bool) {
	for _, col := range t.Columns {
		if strings.EqualFold(col.Name, name) {
			return col, true
		}
	}
	return schema.Column{}, false
}

func joinKindToGo(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "inner":
		return "goql.JoinInner"
	case "right":
		return "goql.JoinRight"
	case "full":
		return "goql.JoinFull"
	default:
		return "goql.JoinLeft"
	}
}
