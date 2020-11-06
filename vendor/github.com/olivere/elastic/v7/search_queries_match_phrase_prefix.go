// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// MatchPhrasePrefixQuery is the same as match_phrase, except that it allows for
// prefix matches on the last term in the text.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/query-dsl-match-query-phrase-prefix.html
type MatchPhrasePrefixQuery struct {
	name          string
	value         interface{}
	analyzer      string
	slop          *int
	maxExpansions *int
	boost         *float64
	queryName     string
}

// NewMatchPhrasePrefixQuery creates and initializes a new MatchPhrasePrefixQuery.
func NewMatchPhrasePrefixQuery(name string, value interface{}) *MatchPhrasePrefixQuery {
	return &MatchPhrasePrefixQuery{name: name, value: value}
}

// Analyzer explicitly sets the analyzer to use. It defaults to use explicit
// mapping config for the field, or, if not set, the default search analyzer.
func (q *MatchPhrasePrefixQuery) Analyzer(analyzer string) *MatchPhrasePrefixQuery {
	q.analyzer = analyzer
	return q
}

// Slop sets the phrase slop if evaluated to a phrase query type.
func (q *MatchPhrasePrefixQuery) Slop(slop int) *MatchPhrasePrefixQuery {
	q.slop = &slop
	return q
}

// MaxExpansions sets the number of term expansions to use.
func (q *MatchPhrasePrefixQuery) MaxExpansions(n int) *MatchPhrasePrefixQuery {
	q.maxExpansions = &n
	return q
}

// Boost sets the boost to apply to this query.
func (q *MatchPhrasePrefixQuery) Boost(boost float64) *MatchPhrasePrefixQuery {
	q.boost = &boost
	return q
}

// QueryName sets the query name for the filter that can be used when
// searching for matched filters per hit.
func (q *MatchPhrasePrefixQuery) QueryName(queryName string) *MatchPhrasePrefixQuery {
	q.queryName = queryName
	return q
}

// Source returns JSON for the function score query.
func (q *MatchPhrasePrefixQuery) Source() (interface{}, error) {
	// {"match_phrase_prefix":{"name":{"query":"value","max_expansions":10}}}
	source := make(map[string]interface{})

	match := make(map[string]interface{})
	source["match_phrase_prefix"] = match

	query := make(map[string]interface{})
	match[q.name] = query

	query["query"] = q.value

	if q.analyzer != "" {
		query["analyzer"] = q.analyzer
	}
	if q.slop != nil {
		query["slop"] = *q.slop
	}
	if q.maxExpansions != nil {
		query["max_expansions"] = *q.maxExpansions
	}
	if q.boost != nil {
		query["boost"] = *q.boost
	}
	if q.queryName != "" {
		query["_name"] = q.queryName
	}

	return source, nil
}
