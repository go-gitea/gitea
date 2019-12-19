// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import "errors"

// ScriptScoreQuery uses a script to provide a custom score for returned documents.
//
// A ScriptScoreQuery query is useful if, for example, a scoring function is
// expensive and you only need to calculate the score of a filtered set of documents.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.4/query-dsl-script-score-query.html
type ScriptScoreQuery struct {
	query     Query
	script    *Script
	minScore  *float64
	boost     *float64
	queryName string
}

// NewScriptScoreQuery creates and initializes a new script_score query.
func NewScriptScoreQuery(query Query, script *Script) *ScriptScoreQuery {
	return &ScriptScoreQuery{
		query: query,
		script: script,
	}
}

// Query to be used in the ScriptScoreQuery.
func (q *ScriptScoreQuery) Query(query Query) *ScriptScoreQuery {
	q.query = query
	return q
}

// Script to calculate the score.
func (q *ScriptScoreQuery) Script(script *Script) *ScriptScoreQuery {
	q.script = script
	return q
}

// MinScore sets the minimum score.
func (q *ScriptScoreQuery) MinScore(minScore float64) *ScriptScoreQuery {
	q.minScore = &minScore
	return q
}

// Boost sets the boost for this query.
func (q *ScriptScoreQuery) Boost(boost float64) *ScriptScoreQuery {
	q.boost = &boost
	return q
}

// QueryName sets the query name for the filter.
func (q *ScriptScoreQuery) QueryName(queryName string) *ScriptScoreQuery {
	q.queryName = queryName
	return q
}

// Source returns JSON for the function score query.
func (q *ScriptScoreQuery) Source() (interface{}, error) {
	// {
	//   "script_score" : {
	//     "query" : {
	//       "match" : { "message": "elasticsearch" }
	//     },
	//     "script" : {
	//       "source" : "doc['likes'].value / 10"
	//     }
	//	 }
	// }

	source := make(map[string]interface{})
	query := make(map[string]interface{})
	source["script_score"] = query

	if q.query == nil {
		return nil, errors.New("ScriptScoreQuery: Query is missing")
	}
	if q.script == nil {
		return nil, errors.New("ScriptScoreQuery: Script is missing")
	}

	if src, err := q.query.Source(); err != nil {
		return nil, err
	} else {
		query["query"] = src
	}

	if src, err := q.script.Source(); err != nil {
		return nil, err
	} else {
		query["script"] = src
	}

	if v := q.minScore; v != nil {
		query["min_score"] = *v
	}

	if v := q.boost; v != nil {
		query["boost"] = *v
	}
	if q.queryName != "" {
		query["_name"] = q.queryName
	}

	return source, nil
}
