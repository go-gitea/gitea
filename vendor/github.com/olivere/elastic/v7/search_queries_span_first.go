// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// SpanFirstQuery spans near the beginning of a field.
// The span first query maps to Lucene SpanFirstQuery
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.7/query-dsl-span-first-query.html
// for details.
type SpanFirstQuery struct {
	match     Query
	end       int
	boost     *float64
	queryName string
}

// NewSpanFirstQuery creates a new SpanFirstQuery.
func NewSpanFirstQuery(query Query, end int) *SpanFirstQuery {
	return &SpanFirstQuery{
		match: query,
		end:   end,
	}
}

// Match sets the query, e.g. a SpanTermQuery.
func (q *SpanFirstQuery) Match(query Query) *SpanFirstQuery {
	q.match = query
	return q
}

// End specifies the maximum end position of the match, which needs to be positive.
func (q *SpanFirstQuery) End(end int) *SpanFirstQuery {
	q.end = end
	return q
}

// Boost sets the boost for this query.
func (q *SpanFirstQuery) Boost(boost float64) *SpanFirstQuery {
	q.boost = &boost
	return q
}

// QueryName sets the query name for the filter that can be used when
// searching for matched_filters per hit.
func (q *SpanFirstQuery) QueryName(queryName string) *SpanFirstQuery {
	q.queryName = queryName
	return q
}

// Source returns the JSON body.
func (q *SpanFirstQuery) Source() (interface{}, error) {
	m := make(map[string]interface{})
	c := make(map[string]interface{})

	if v := q.match; v != nil {
		src, err := q.match.Source()
		if err != nil {
			return nil, err
		}
		c["match"] = src
	}
	c["end"] = q.end

	if v := q.boost; v != nil {
		c["boost"] = *v
	}
	if v := q.queryName; v != "" {
		c["query_name"] = v
	}
	m["span_first"] = c
	return m, nil
}
