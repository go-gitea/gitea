// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"fmt"
)

// DistanceFeatureQuery uses a script to provide a custom score for returned documents.
//
// A DistanceFeatureQuery query is useful if, for example, a scoring function is
// expensive and you only need to calculate the score of a filtered set of documents.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.4/query-dsl-distance-feature-query.html
type DistanceFeatureQuery struct {
	field     string
	pivot     string
	origin    interface{}
	boost     *float64
	queryName string
}

// NewDistanceFeatureQuery creates and initializes a new script_score query.
func NewDistanceFeatureQuery(field string, origin interface{}, pivot string) *DistanceFeatureQuery {
	return &DistanceFeatureQuery{
		field:  field,
		origin: origin,
		pivot:  pivot,
	}
}

// Field to be used in the DistanceFeatureQuery.
func (q *DistanceFeatureQuery) Field(name string) *DistanceFeatureQuery {
	q.field = name
	return q
}

// Origin is the date or point of origin used to calculate distances.
//
// If the field is a date or date_nanos field, the origin value must be a
// date. Date math such as "now-1h" is supported.
//
// If the field is a geo_point field, the origin must be a GeoPoint.
func (q *DistanceFeatureQuery) Origin(origin interface{}) *DistanceFeatureQuery {
	q.origin = origin
	return q
}

// Pivot is distance from the origin at which relevance scores
// receive half of the boost value.
//
// If field is a date or date_nanos field, the pivot value must be a time
// unit, such as "1h" or "10d".
//
// If field is a geo_point field, the pivot value must be a distance unit,
// such as "1km" or "12m". You can pass a string, or a GeoPoint.
func (q *DistanceFeatureQuery) Pivot(pivot string) *DistanceFeatureQuery {
	q.pivot = pivot
	return q
}

// Boost sets the boost for this query.
func (q *DistanceFeatureQuery) Boost(boost float64) *DistanceFeatureQuery {
	q.boost = &boost
	return q
}

// QueryName sets the query name for the filter.
func (q *DistanceFeatureQuery) QueryName(queryName string) *DistanceFeatureQuery {
	q.queryName = queryName
	return q
}

// Source returns JSON for the function score query.
func (q *DistanceFeatureQuery) Source() (interface{}, error) {
	// {
	//   "distance_feature" : {
	//     "field" : "production_date",
	//     "pivot" : "7d",
	//     "origin" : "now"
	//	 }
	// }
	// {
	//   "distance_feature" : {
	//     "field" : "location",
	//     "pivot" : "1000m",
	//     "origin" : [-71.3, 41.15]
	//	 }
	// }

	source := make(map[string]interface{})
	query := make(map[string]interface{})
	source["distance_feature"] = query

	query["field"] = q.field
	query["pivot"] = q.pivot
	switch v := q.origin.(type) {
	default:
		return nil, fmt.Errorf("DistanceFeatureQuery: unable to serialize Origin from type %T", v)
	case string:
		query["origin"] = v
	case *GeoPoint:
		query["origin"] = v.Source()
	case GeoPoint:
		query["origin"] = v.Source()
	}

	if v := q.boost; v != nil {
		query["boost"] = *v
	}
	if q.queryName != "" {
		query["_name"] = q.queryName
	}

	return source, nil
}
