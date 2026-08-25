package mssql_test

import (
	"strings"
	"testing"

	"github.com/yurimoinhos/ameego/pkg/goql/mssql"
)

func TestVectorScanValue(t *testing.T) {
	var v mssql.Vector
	if err := v.Scan("[1.5, 2, 3]"); err != nil {
		t.Fatal(err)
	}
	if len(v) != 3 || v[0] != 1.5 {
		t.Fatalf("scan: %#v", v)
	}
	raw, err := v.Value()
	if err != nil {
		t.Fatal(err)
	}
	s, _ := raw.(string)
	if !strings.HasPrefix(s, "[") || !strings.Contains(s, "1.5") {
		t.Fatalf("value: %q", s)
	}
}

func TestApplyTop(t *testing.T) {
	sql := mssql.ApplyTop(`SELECT [id] FROM [users]`, 10)
	if !strings.Contains(sql, "SELECT TOP (10) ") {
		t.Fatalf("got %q", sql)
	}
}

func TestVectorLiteral(t *testing.T) {
	lit, err := mssql.VectorLiteral(mssql.Vector{1, 2}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(lit, "VECTOR(2)") || !strings.Contains(lit, "[1,2]") {
		t.Fatalf("literal: %q", lit)
	}
}
