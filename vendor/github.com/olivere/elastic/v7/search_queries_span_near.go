// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// SpanNearQuery matches spans which are near one another. One can specify slop,
// the maximum number of intervening unmatched positions, as well as whether
// matches are required to be in-order. The span near query maps to Lucene SpanNearQuery.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.7/query-dsl-span-near-query.html
// for details.
type SpanNearQuery struct {
	clauses   []Query
	slop      *int
	inOrder   *bool
	boost     *float64
	queryName string
}

// NewSpanNearQuery creates a new SpanNearQuery.
func NewSpanNearQuery(clauses ...Query) *SpanNearQuery {
	return &SpanNearQuery{
		clauses: clauses,
	}
}

// Add clauses to use in the query.
func (q *SpanNearQuery) Add(clauses ...Query) *SpanNearQuery {
	q.clauses = append(q.clauses, clauses...)
	return q
}

// Clauses to use in the query.
func (q *SpanNearQuery) Clauses(clauses ...Query) *SpanNearQuery {
	q.clauses = clauses
	return q
}

// Slop controls the maximum number of intervening unmatched positions permitted.
func (q *SpanNearQuery) Slop(slop int) *SpanNearQuery {
	q.slop = &slop
	return q
}

// InOrder, when true, the spans from each clause must be in the same order as
// in Clauses and must be non-overlapping. Defaults to true.
func (q *SpanNearQuery) InOrder(inOrder bool) *SpanNearQuery {
	q.inOrder = &inOrder
	return q
}

// Boost sets the boost for this query.
func (q *SpanNearQuery) Boost(boost float64) *SpanNearQuery {
	q.boost = &boost
	return q
}

// QueryName sets the query name for the filter that can be used when
// searching for matched_filters per hit.
func (q *SpanNearQuery) QueryName(queryName string) *SpanNearQuery {
	q.queryName = queryName
	return q
}

// Source returns the JSON body.
func (q *SpanNearQuery) Source() (interface{}, error) {
	m := make(map[string]interface{})
	c := make(map[string]interface{})

	if len(q.clauses) > 0 {
		var clauses []interface{}
		for _, clause := range q.clauses {
			src, err := clause.Source()
			if err != nil {
				return nil, err
			}
			clauses = append(clauses, src)
		}
		c["clauses"] = clauses
	}

	if v := q.slop; v != nil {
		c["slop"] = *v
	}
	if v := q.inOrder; v != nil {
		c["in_order"] = *v
	}

	if v := q.boost; v != nil {
		c["boost"] = *v
	}
	if v := q.queryName; v != "" {
		c["query_name"] = v
	}
	m["span_near"] = c
	return m, nil
}
