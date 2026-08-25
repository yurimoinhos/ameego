// Package mssql exposes Microsoft SQL Server / T-SQL helpers used by Ameego codegen.
// These APIs are concrete — there is no cross-dialect feature interface.
package mssql

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// Vector is the SQL Server VECTOR(n) type (embedding).
// Values are exchanged as the textual form [1.0,2.0,3.0] understood by T-SQL VECTOR.
type Vector []float32

// Dimension returns the vector length.
func (v Vector) Dimension() int { return len(v) }

// Value implements driver.Valuer.
func (v Vector) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String(), nil
}

// Scan implements sql.Scanner.
func (v *Vector) Scan(src any) error {
	if src == nil {
		*v = nil
		return nil
	}
	var s string
	switch t := src.(type) {
	case string:
		s = t
	case []byte:
		s = string(t)
	default:
		return fmt.Errorf("mssql: cannot scan %T into Vector", src)
	}
	s = strings.TrimSpace(s)
	if s == "" || s == "NULL" {
		*v = nil
		return nil
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = s[1 : len(s)-1]
	}
	if strings.TrimSpace(s) == "" {
		*v = Vector{}
		return nil
	}
	parts := strings.Split(s, ",")
	out := make(Vector, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		f, err := strconv.ParseFloat(p, 32)
		if err != nil {
			return fmt.Errorf("mssql: parse vector element %q: %w", p, err)
		}
		out = append(out, float32(f))
	}
	*v = out
	return nil
}

// DistanceMetric is a VECTOR_DISTANCE metric name.
type DistanceMetric string

const (
	DistanceCosine     DistanceMetric = "cosine"
	DistanceEuclidean  DistanceMetric = "euclidean"
	DistanceDot        DistanceMetric = "dot"
)

// VectorDistanceExpr builds VECTOR_DISTANCE(metric, left, right) for T-SQL ORDER BY / SELECT.
func VectorDistanceExpr(metric DistanceMetric, leftSQL, rightSQL string) string {
	if metric == "" {
		metric = DistanceCosine
	}
	return fmt.Sprintf("VECTOR_DISTANCE('%s', %s, %s)", metric, leftSQL, rightSQL)
}

// VectorTypeSQL returns VECTOR(n) for DDL.
func VectorTypeSQL(dim int) string {
	if dim <= 0 {
		return "VECTOR"
	}
	return fmt.Sprintf("VECTOR(%d)", dim)
}

// VectorLiteral formats a VECTOR cast literal for embedding in T-SQL expressions.
func VectorLiteral(v Vector, dim int) (string, error) {
	raw, err := v.Value()
	if err != nil {
		return "", err
	}
	if raw == nil {
		return "NULL", nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("mssql: unexpected vector value type %T", raw)
	}
	s = strings.ReplaceAll(s, "'", "''")
	if dim <= 0 {
		dim = len(v)
	}
	return fmt.Sprintf("CAST(N'%s' AS VECTOR(%d))", s, dim), nil
}
