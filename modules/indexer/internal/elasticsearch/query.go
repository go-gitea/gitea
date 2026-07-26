// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

// MultiMatch types used by the call sites. See
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-multi-match-query.html#multi-match-types
const (
	MultiMatchTypeBestFields   = "best_fields"
	MultiMatchTypePhrasePrefix = "phrase_prefix"
)

// ToAnySlice converts []T to []any for variadic query args like TermsQuery.
func ToAnySlice[T any](s []T) []any {
	out := make([]any, len(s))
	for idx, v := range s {
		out[idx] = v
	}
	return out
}

// Query is an Elasticsearch query DSL node. It marshals to the JSON
// object expected by the ES query API.
type Query interface {
	querySource() map[string]any
}

type rawQuery map[string]any

func (q rawQuery) querySource() map[string]any { return q }

// TermQuery matches documents whose `field` exactly equals `value`.
func TermQuery(field string, value any) Query {
	return rawQuery{"term": map[string]any{field: value}}
}

// TermsQuery matches documents whose `field` equals any of `values`.
func TermsQuery(field string, values ...any) Query {
	return rawQuery{"terms": map[string]any{field: values}}
}

// MatchQuery is a full-text match on a single field.
func MatchQuery(field string, value any) Query {
	return rawQuery{"match": map[string]any{field: value}}
}

// MatchPhraseQuery matches the exact phrase on `field`.
func MatchPhraseQuery(field, value string) Query {
	return rawQuery{"match_phrase": map[string]any{field: value}}
}

// MultiMatchQuery is the fluent builder for a multi_match query.
type MultiMatchQuery struct {
	query    any
	fields   []string
	typ      string
	operator string
}

// NewMultiMatchQuery creates a multi_match query over the given fields.
func NewMultiMatchQuery(query any, fields ...string) *MultiMatchQuery {
	return &MultiMatchQuery{query: query, fields: fields}
}

func (m *MultiMatchQuery) Type(t string) *MultiMatchQuery      { m.typ = t; return m }
func (m *MultiMatchQuery) Operator(op string) *MultiMatchQuery { m.operator = op; return m }

func (m *MultiMatchQuery) querySource() map[string]any {
	body := map[string]any{"query": m.query}
	if len(m.fields) > 0 {
		body["fields"] = m.fields
	}
	if m.typ != "" {
		body["type"] = m.typ
	}
	if m.operator != "" {
		body["operator"] = m.operator
	}
	return map[string]any{"multi_match": body}
}

// RangeQuery is the fluent builder for a range query.
type RangeQuery struct {
	field string
	body  map[string]any
}

func NewRangeQuery(field string) *RangeQuery {
	return &RangeQuery{field: field, body: map[string]any{}}
}

func (r *RangeQuery) Gte(v any) *RangeQuery { r.body["gte"] = v; return r }
func (r *RangeQuery) Lte(v any) *RangeQuery { r.body["lte"] = v; return r }

func (r *RangeQuery) querySource() map[string]any {
	return map[string]any{"range": map[string]any{r.field: r.body}}
}

// BoolQuery is the fluent builder for a bool query.
type BoolQuery struct {
	must    []Query
	should  []Query
	mustNot []Query
}

func NewBoolQuery() *BoolQuery { return &BoolQuery{} }

func (b *BoolQuery) Must(q ...Query) *BoolQuery    { b.must = append(b.must, q...); return b }
func (b *BoolQuery) Should(q ...Query) *BoolQuery  { b.should = append(b.should, q...); return b }
func (b *BoolQuery) MustNot(q ...Query) *BoolQuery { b.mustNot = append(b.mustNot, q...); return b }

func (b *BoolQuery) querySource() map[string]any {
	body := map[string]any{}
	if len(b.must) > 0 {
		body["must"] = querySlice(b.must)
	}
	if len(b.should) > 0 {
		body["should"] = querySlice(b.should)
	}
	if len(b.mustNot) > 0 {
		body["must_not"] = querySlice(b.mustNot)
	}
	return map[string]any{"bool": body}
}

func querySlice(queries []Query) []map[string]any {
	out := make([]map[string]any, len(queries))
	for idx, q := range queries {
		out[idx] = q.querySource()
	}
	return out
}
