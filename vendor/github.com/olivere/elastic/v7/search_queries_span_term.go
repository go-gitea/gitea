// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// SpanTermQuery matches spans containing a term. The span term query maps to Lucene SpanTermQuery.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.7/query-dsl-span-term-query.html
// for details.
type SpanTermQuery struct {
	field     string
	value     interface{}
	boost     *float64
	queryName string
}

// NewSpanTermQuery creates a new SpanTermQuery. When passing values, the first one
// is used to initialize the value.
func NewSpanTermQuery(field string, value ...interface{}) *SpanTermQuery {
	q := &SpanTermQuery{
		field: field,
	}
	if len(value) > 0 {
		q.value = value[0]
	}
	return q
}

// Field name to match the term against.
func (q *SpanTermQuery) Field(field string) *SpanTermQuery {
	q.field = field
	return q
}

// Value of the term.
func (q *SpanTermQuery) Value(value interface{}) *SpanTermQuery {
	q.value = value
	return q
}

// Boost sets the boost for this query.
func (q *SpanTermQuery) Boost(boost float64) *SpanTermQuery {
	q.boost = &boost
	return q
}

// QueryName sets the query name for the filter that can be used when
// searching for matched_filters per hit.
func (q *SpanTermQuery) QueryName(queryName string) *SpanTermQuery {
	q.queryName = queryName
	return q
}

// Source returns the JSON body.
func (q *SpanTermQuery) Source() (interface{}, error) {
	m := make(map[string]interface{})
	c := make(map[string]interface{})
	i := make(map[string]interface{})
	i["value"] = q.value
	if v := q.boost; v != nil {
		i["boost"] = *v
	}
	if v := q.queryName; v != "" {
		i["query_name"] = v
	}
	c[q.field] = i
	m["span_term"] = c
	return m, nil
}
