// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// GeoCentroidAggregation is a metric aggregation that computes the weighted centroid
// from all coordinate values for a Geo-point datatype field.
// See: https://www.elastic.co/guide/en/elasticsearch/reference/5.6/search-aggregations-metrics-geocentroid-aggregation.html
type GeoCentroidAggregation struct {
	field           string
	script          *Script
	subAggregations map[string]Aggregation
	meta            map[string]interface{}
}

func NewGeoCentroidAggregation() *GeoCentroidAggregation {
	return &GeoCentroidAggregation{
		subAggregations: make(map[string]Aggregation),
	}
}

func (a *GeoCentroidAggregation) Field(field string) *GeoCentroidAggregation {
	a.field = field
	return a
}

func (a *GeoCentroidAggregation) Script(script *Script) *GeoCentroidAggregation {
	a.script = script
	return a
}

func (a *GeoCentroidAggregation) SubAggregation(name string, subAggregation Aggregation) *GeoCentroidAggregation {
	a.subAggregations[name] = subAggregation
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *GeoCentroidAggregation) Meta(metaData map[string]interface{}) *GeoCentroidAggregation {
	a.meta = metaData
	return a
}

func (a *GeoCentroidAggregation) Source() (interface{}, error) {
	// Example:
	// {
	//     "query" : {
	//         "match" : { "business_type" : "shop" }
	//     },
	//     "aggs" : {
	//			"centroid" : {
	//				"geo_centroid" : {
	//					"field" : "location"
	//				}
	//			}
	//		}
	// }
	//
	// This method returns only the { "geo_centroid" : { ... } } part.

	source := make(map[string]interface{})
	opts := make(map[string]interface{})
	source["geo_centroid"] = opts

	if a.field != "" {
		opts["field"] = a.field
	}
	if a.script != nil {
		src, err := a.script.Source()
		if err != nil {
			return nil, err
		}
		opts["script"] = src
	}

	// AggregationBuilder (SubAggregations)
	if len(a.subAggregations) > 0 {
		aggsMap := make(map[string]interface{})
		source["aggregations"] = aggsMap
		for name, aggregate := range a.subAggregations {
			src, err := aggregate.Source()
			if err != nil {
				return nil, err
			}
			aggsMap[name] = src
		}
	}

	// Add Meta data if available
	if len(a.meta) > 0 {
		source["meta"] = a.meta
	}

	return source, nil
}
