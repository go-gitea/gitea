// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// MatchNoneQuery returns no documents. It is the inverse of
// MatchAllQuery.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/5.6/query-dsl-match-all-query.html
type MatchNoneQuery struct {
	queryName string
}

// NewMatchNoneQuery creates and initializes a new match none query.
func NewMatchNoneQuery() *MatchNoneQuery {
	return &MatchNoneQuery{}
}

// QueryName sets the query name.
func (q *MatchNoneQuery) QueryName(name string) *MatchNoneQuery {
	q.queryName = name
	return q
}

// Source returns JSON for the match none query.
func (q MatchNoneQuery) Source() (interface{}, error) {
	// {
	//   "match_none" : { ... }
	// }
	source := make(map[string]interface{})
	params := make(map[string]interface{})
	source["match_none"] = params
	if q.queryName != "" {
		params["_name"] = q.queryName
	}
	return source, nil
}
