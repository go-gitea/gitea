// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// WrapperQuery accepts any other query as base64 encoded string.
//
// For details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/query-dsl-wrapper-query.html.
type WrapperQuery struct {
	source string
}

// NewWrapperQuery creates and initializes a new WrapperQuery.
func NewWrapperQuery(source string) *WrapperQuery {
	return &WrapperQuery{source: source}
}

// Source returns JSON for the query.
func (q *WrapperQuery) Source() (interface{}, error) {
	// {"wrapper":{"query":"..."}}
	source := make(map[string]interface{})
	tq := make(map[string]interface{})
	source["wrapper"] = tq
	tq["query"] = q.source
	return source, nil
}
