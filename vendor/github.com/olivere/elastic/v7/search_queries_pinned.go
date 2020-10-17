// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// PinnedQuery is a query that promotes selected documents to rank higher than those matching a given query.
//
// For more details, see:
// https://www.elastic.co/guide/en/elasticsearch/reference/7.8/query-dsl-pinned-query.html
type PinnedQuery struct {
	ids     []string
	organic Query
}

// NewPinnedQuery creates and initializes a new pinned query.
func NewPinnedQuery() *PinnedQuery {
	return &PinnedQuery{}
}

// Ids sets an array of document IDs listed in the order they are to appear in results.
func (q *PinnedQuery) Ids(ids ...string) *PinnedQuery {
	q.ids = ids
	return q
}

// Organic sets a choice of query used to rank documents which will be ranked below the "pinned" document ids.
func (q *PinnedQuery) Organic(query Query) *PinnedQuery {
	q.organic = query
	return q
}

// Source returns the JSON serializable content for this query.
func (q *PinnedQuery) Source() (interface{}, error) {
	// {
	// 	  "pinned": {
	// 	  	"ids": [ "1", "4", "100" ],
	// 	  	"organic": {
	// 		  "match": {
	// 		    "description": "iphone"
	// 		  }
	// 		}
	//    }
	// }

	query := make(map[string]interface{})
	params := make(map[string]interface{})
	query["pinned"] = params
	if len(q.ids) > 0 {
		params["ids"] = q.ids
	}
	if q.organic != nil {
		src, err := q.organic.Source()
		if err != nil {
			return nil, err
		}
		params["organic"] = src
	}

	return query, nil
}
