package main

import (
	"fmt"
	"strings"
)

type Query struct {
	selectCols []string
	from       string
	where      []string
	orderBy    string
	limit      int
}

type QueryBuilder struct {
	q Query
}

func Select(cols ...string) *QueryBuilder {
	return &QueryBuilder{q: Query{selectCols: cols}}
}

func (b *QueryBuilder) From(table string) *QueryBuilder {
	b.q.from = table
	return b
}

func (b *QueryBuilder) Where(cond string) *QueryBuilder {
	b.q.where = append(b.q.where, cond)
	return b
}

func (b *QueryBuilder) OrderBy(col string) *QueryBuilder {
	b.q.orderBy = col
	return b
}

func (b *QueryBuilder) Limit(n int) *QueryBuilder {
	b.q.limit = n
	return b
}

func (b *QueryBuilder) Build() string {
	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(strings.Join(b.q.selectCols, ", "))
	sb.WriteString(" FROM ")
	sb.WriteString(b.q.from)
	if len(b.q.where) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(b.q.where, " AND "))
	}
	if b.q.orderBy != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(b.q.orderBy)
	}
	if b.q.limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", b.q.limit))
	}
	return sb.String()
}

func main() {
	query := Select("id", "name", "email").
		From("users").
		Where("active = true").
		Where("created_at > '2024-01-01'").
		OrderBy("name").
		Limit(10).
		Build()

	fmt.Println(query)
}
