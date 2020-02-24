// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// AutoDateHistogramAggregation is a multi-bucket aggregation similar to the
// histogram except it can only be applied on date values, and the buckets num can bin pointed.
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.3/search-aggregations-bucket-autodatehistogram-aggregation.html
type AutoDateHistogramAggregation struct {
	field           string
	script          *Script
	missing         interface{}
	subAggregations map[string]Aggregation
	meta            map[string]interface{}

	buckets         int
	minDocCount     *int64
	timeZone        string
	format          string
	minimumInterval string
}

// NewAutoDateHistogramAggregation creates a new AutoDateHistogramAggregation.
func NewAutoDateHistogramAggregation() *AutoDateHistogramAggregation {
	return &AutoDateHistogramAggregation{
		subAggregations: make(map[string]Aggregation),
	}
}

// Field on which the aggregation is processed.
func (a *AutoDateHistogramAggregation) Field(field string) *AutoDateHistogramAggregation {
	a.field = field
	return a
}

// Script on which th
func (a *AutoDateHistogramAggregation) Script(script *Script) *AutoDateHistogramAggregation {
	a.script = script
	return a
}

// Missing configures the value to use when documents miss a value.
func (a *AutoDateHistogramAggregation) Missing(missing interface{}) *AutoDateHistogramAggregation {
	a.missing = missing
	return a
}

// SubAggregation sub aggregation
func (a *AutoDateHistogramAggregation) SubAggregation(name string, subAggregation Aggregation) *AutoDateHistogramAggregation {
	a.subAggregations[name] = subAggregation
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *AutoDateHistogramAggregation) Meta(metaData map[string]interface{}) *AutoDateHistogramAggregation {
	a.meta = metaData
	return a
}

// Buckets buckets num by which the aggregation gets processed.
func (a *AutoDateHistogramAggregation) Buckets(buckets int) *AutoDateHistogramAggregation {
	a.buckets = buckets
	return a
}

// MinDocCount sets the minimum document count per bucket.
// Buckets with less documents than this min value will not be returned.
func (a *AutoDateHistogramAggregation) MinDocCount(minDocCount int64) *AutoDateHistogramAggregation {
	a.minDocCount = &minDocCount
	return a
}

// TimeZone sets the timezone in which to translate dates before computing buckets.
func (a *AutoDateHistogramAggregation) TimeZone(timeZone string) *AutoDateHistogramAggregation {
	a.timeZone = timeZone
	return a
}

// Format sets the format to use for dates.
func (a *AutoDateHistogramAggregation) Format(format string) *AutoDateHistogramAggregation {
	a.format = format
	return a
}

// MinimumInterval accepted units for minimum_interval are: year/month/day/hour/minute/second
func (a *AutoDateHistogramAggregation) MinimumInterval(interval string) *AutoDateHistogramAggregation {
	a.minimumInterval = interval
	return a
}

// Source source for AutoDateHistogramAggregation
func (a *AutoDateHistogramAggregation) Source() (interface{}, error) {
	// Example:
	// {
	//     "aggs" : {
	//         "articles_over_time" : {
	//             "auto_date_histogram" : {
	//                 "field" : "date",
	//                 "buckets" : 10
	//             }
	//         }
	//     }
	// }
	//
	// This method returns only the { "auto_date_histogram" : { ... } } part.

	source := make(map[string]interface{})
	opts := make(map[string]interface{})
	source["auto_date_histogram"] = opts

	// ValuesSourceAggregationBuilder
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
	if a.missing != nil {
		opts["missing"] = a.missing
	}

	if a.buckets > 0 {
		opts["buckets"] = a.buckets
	}

	if a.minDocCount != nil {
		opts["min_doc_count"] = *a.minDocCount
	}
	if a.timeZone != "" {
		opts["time_zone"] = a.timeZone
	}
	if a.format != "" {
		opts["format"] = a.format
	}
	if a.minimumInterval != "" {
		opts["minimum_interval"] = a.minimumInterval
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
