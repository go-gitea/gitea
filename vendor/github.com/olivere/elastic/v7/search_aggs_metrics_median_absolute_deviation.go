// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// MedianAbsoluteDeviationAggregation is a measure of variability.
// It is a robust statistic, meaning that it is useful for describing data
// that may have outliers, or may not be normally distributed.
// For such data it can be more descriptive than standard deviation.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.6/search-aggregations-metrics-median-absolute-deviation-aggregation.html
// for details.
type MedianAbsoluteDeviationAggregation struct {
	field           string
	compression     *float64
	script          *Script
	format          string
	missing         interface{}
	subAggregations map[string]Aggregation
	meta            map[string]interface{}
}

func NewMedianAbsoluteDeviationAggregation() *MedianAbsoluteDeviationAggregation {
	return &MedianAbsoluteDeviationAggregation{
		subAggregations: make(map[string]Aggregation),
	}
}

func (a *MedianAbsoluteDeviationAggregation) Field(field string) *MedianAbsoluteDeviationAggregation {
	a.field = field
	return a
}

func (a *MedianAbsoluteDeviationAggregation) Compression(compression float64) *MedianAbsoluteDeviationAggregation {
	a.compression = &compression
	return a
}

func (a *MedianAbsoluteDeviationAggregation) Script(script *Script) *MedianAbsoluteDeviationAggregation {
	a.script = script
	return a
}

func (a *MedianAbsoluteDeviationAggregation) Format(format string) *MedianAbsoluteDeviationAggregation {
	a.format = format
	return a
}

func (a *MedianAbsoluteDeviationAggregation) Missing(missing interface{}) *MedianAbsoluteDeviationAggregation {
	a.missing = missing
	return a
}

func (a *MedianAbsoluteDeviationAggregation) SubAggregation(name string, subAggregation Aggregation) *MedianAbsoluteDeviationAggregation {
	a.subAggregations[name] = subAggregation
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *MedianAbsoluteDeviationAggregation) Meta(metaData map[string]interface{}) *MedianAbsoluteDeviationAggregation {
	a.meta = metaData
	return a
}

func (a *MedianAbsoluteDeviationAggregation) Source() (interface{}, error) {
	// Example:
	//	{
	//    "aggs" : {
	//      "review_variability" : { "median_absolute_deviation" : { "field" : "rating" } }
	//    }
	//	}
	// This method returns only the { "median_absolute_deviation" : { "field" : "rating" } } part.

	source := make(map[string]interface{})
	opts := make(map[string]interface{})
	source["median_absolute_deviation"] = opts

	// ValuesSourceAggregationBuilder
	if a.field != "" {
		opts["field"] = a.field
	}
	if v := a.compression; v != nil {
		opts["compression"] = *v
	}
	if a.script != nil {
		src, err := a.script.Source()
		if err != nil {
			return nil, err
		}
		opts["script"] = src
	}
	if a.format != "" {
		opts["format"] = a.format
	}
	if a.missing != nil {
		opts["missing"] = a.missing
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
