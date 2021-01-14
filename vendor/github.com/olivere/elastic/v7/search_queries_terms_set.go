// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// TermsSetQuery returns any documents that match with at least
// one or more of the provided terms. The terms are not analyzed
// and thus must match exactly. The number of terms that must
// match varies per document and is either controlled by a
// minimum should match field or computed per document in a
// minimum should match script.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/query-dsl-terms-set-query.html
type TermsSetQuery struct {
	name                     string
	values                   []interface{}
	minimumShouldMatchField  string
	minimumShouldMatchScript *Script
	queryName                string
	boost                    *float64
}

// NewTermsSetQuery creates and initializes a new TermsSetQuery.
func NewTermsSetQuery(name string, values ...interface{}) *TermsSetQuery {
	q := &TermsSetQuery{
		name:   name,
		values: make([]interface{}, 0),
	}
	if len(values) > 0 {
		q.values = append(q.values, values...)
	}
	return q
}

// MinimumShouldMatchField specifies the field to match.
func (q *TermsSetQuery) MinimumShouldMatchField(minimumShouldMatchField string) *TermsSetQuery {
	q.minimumShouldMatchField = minimumShouldMatchField
	return q
}

// MinimumShouldMatchScript specifies the script to match.
func (q *TermsSetQuery) MinimumShouldMatchScript(minimumShouldMatchScript *Script) *TermsSetQuery {
	q.minimumShouldMatchScript = minimumShouldMatchScript
	return q
}

// Boost sets the boost for this query.
func (q *TermsSetQuery) Boost(boost float64) *TermsSetQuery {
	q.boost = &boost
	return q
}

// QueryName sets the query name for the filter that can be used
// when searching for matched_filters per hit
func (q *TermsSetQuery) QueryName(queryName string) *TermsSetQuery {
	q.queryName = queryName
	return q
}

// Source creates the query source for the term query.
func (q *TermsSetQuery) Source() (interface{}, error) {
	// {"terms_set":{"codes":{"terms":["abc","def"],"minimum_should_match_field":"required_matches"}}}
	source := make(map[string]interface{})
	inner := make(map[string]interface{})
	params := make(map[string]interface{})
	inner[q.name] = params
	source["terms_set"] = inner

	// terms
	params["terms"] = q.values

	// minimum_should_match_field
	if match := q.minimumShouldMatchField; match != "" {
		params["minimum_should_match_field"] = match
	}

	// minimum_should_match_script
	if match := q.minimumShouldMatchScript; match != nil {
		src, err := match.Source()
		if err != nil {
			return nil, err
		}
		params["minimum_should_match_script"] = src
	}

	// Common parameters for all queries
	if q.boost != nil {
		params["boost"] = *q.boost
	}
	if q.queryName != "" {
		params["_name"] = q.queryName
	}

	return source, nil
}
