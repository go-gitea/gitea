// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// CompositeAggregation is a multi-bucket values source based aggregation
// that can be used to calculate unique composite values from source documents.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-bucket-composite-aggregation.html
// for details.
type CompositeAggregation struct {
	after           map[string]interface{}
	size            *int
	sources         []CompositeAggregationValuesSource
	subAggregations map[string]Aggregation
	meta            map[string]interface{}
}

// NewCompositeAggregation creates a new CompositeAggregation.
func NewCompositeAggregation() *CompositeAggregation {
	return &CompositeAggregation{
		sources:         make([]CompositeAggregationValuesSource, 0),
		subAggregations: make(map[string]Aggregation),
	}
}

// Size represents the number of composite buckets to return.
// Defaults to 10 as of Elasticsearch 6.1.
func (a *CompositeAggregation) Size(size int) *CompositeAggregation {
	a.size = &size
	return a
}

// AggregateAfter sets the values that indicate which composite bucket this
// request should "aggregate after".
func (a *CompositeAggregation) AggregateAfter(after map[string]interface{}) *CompositeAggregation {
	a.after = after
	return a
}

// Sources specifies the list of CompositeAggregationValuesSource instances to
// use in the aggregation.
func (a *CompositeAggregation) Sources(sources ...CompositeAggregationValuesSource) *CompositeAggregation {
	a.sources = append(a.sources, sources...)
	return a
}

// SubAggregations of this aggregation.
func (a *CompositeAggregation) SubAggregation(name string, subAggregation Aggregation) *CompositeAggregation {
	a.subAggregations[name] = subAggregation
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *CompositeAggregation) Meta(metaData map[string]interface{}) *CompositeAggregation {
	a.meta = metaData
	return a
}

// Source returns the serializable JSON for this aggregation.
func (a *CompositeAggregation) Source() (interface{}, error) {
	// Example:
	// {
	//     "aggs" : {
	//         "my_composite_agg" : {
	//             "composite" : {
	//                 "sources": [
	//				      {"my_term": { "terms": { "field": "product" }}},
	//				      {"my_histo": { "histogram": { "field": "price", "interval": 5 }}},
	//				      {"my_date": { "date_histogram": { "field": "timestamp", "interval": "1d" }}},
	//                 ],
	//                 "size" : 10,
	//                 "after" : ["a", 2, "c"]
	//             }
	//         }
	//     }
	// }
	//
	// This method returns only the { "histogram" : { ... } } part.

	source := make(map[string]interface{})
	opts := make(map[string]interface{})
	source["composite"] = opts

	sources := make([]interface{}, len(a.sources))
	for i, s := range a.sources {
		src, err := s.Source()
		if err != nil {
			return nil, err
		}
		sources[i] = src
	}
	opts["sources"] = sources

	if a.size != nil {
		opts["size"] = *a.size
	}

	if a.after != nil {
		opts["after"] = a.after
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

// -- Generic interface for CompositeAggregationValues --

// CompositeAggregationValuesSource specifies the interface that
// all implementations for CompositeAggregation's Sources method
// need to implement.
//
// The different implementations are described in
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-bucket-composite-aggregation.html#_values_source_2.
type CompositeAggregationValuesSource interface {
	Source() (interface{}, error)
}

// -- CompositeAggregationTermsValuesSource --

// CompositeAggregationTermsValuesSource is a source for the CompositeAggregation that handles terms
// it works very similar to a terms aggregation with slightly different syntax
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-bucket-composite-aggregation.html#_terms
// for details.
type CompositeAggregationTermsValuesSource struct {
	name          string
	field         string
	script        *Script
	valueType     string
	missing       interface{}
	missingBucket *bool
	order         string
}

// NewCompositeAggregationTermsValuesSource creates and initializes
// a new CompositeAggregationTermsValuesSource.
func NewCompositeAggregationTermsValuesSource(name string) *CompositeAggregationTermsValuesSource {
	return &CompositeAggregationTermsValuesSource{
		name: name,
	}
}

// Field to use for this source.
func (a *CompositeAggregationTermsValuesSource) Field(field string) *CompositeAggregationTermsValuesSource {
	a.field = field
	return a
}

// Script to use for this source.
func (a *CompositeAggregationTermsValuesSource) Script(script *Script) *CompositeAggregationTermsValuesSource {
	a.script = script
	return a
}

// ValueType specifies the type of values produced by this source,
// e.g. "string" or "date".
func (a *CompositeAggregationTermsValuesSource) ValueType(valueType string) *CompositeAggregationTermsValuesSource {
	a.valueType = valueType
	return a
}

// Order specifies the order in the values produced by this source.
// It can be either "asc" or "desc".
func (a *CompositeAggregationTermsValuesSource) Order(order string) *CompositeAggregationTermsValuesSource {
	a.order = order
	return a
}

// Asc ensures the order of the values produced is ascending.
func (a *CompositeAggregationTermsValuesSource) Asc() *CompositeAggregationTermsValuesSource {
	a.order = "asc"
	return a
}

// Desc ensures the order of the values produced is descending.
func (a *CompositeAggregationTermsValuesSource) Desc() *CompositeAggregationTermsValuesSource {
	a.order = "desc"
	return a
}

// Missing specifies the value to use when the source finds a missing
// value in a document.
//
// Deprecated: Use MissingBucket instead.
func (a *CompositeAggregationTermsValuesSource) Missing(missing interface{}) *CompositeAggregationTermsValuesSource {
	a.missing = missing
	return a
}

// MissingBucket, if true, will create an explicit null bucket which represents
// documents with missing values.
func (a *CompositeAggregationTermsValuesSource) MissingBucket(missingBucket bool) *CompositeAggregationTermsValuesSource {
	a.missingBucket = &missingBucket
	return a
}

// Source returns the serializable JSON for this values source.
func (a *CompositeAggregationTermsValuesSource) Source() (interface{}, error) {
	source := make(map[string]interface{})
	name := make(map[string]interface{})
	source[a.name] = name
	values := make(map[string]interface{})
	name["terms"] = values

	// field
	if a.field != "" {
		values["field"] = a.field
	}

	// script
	if a.script != nil {
		src, err := a.script.Source()
		if err != nil {
			return nil, err
		}
		values["script"] = src
	}

	// missing
	if a.missing != nil {
		values["missing"] = a.missing
	}

	// missing_bucket
	if a.missingBucket != nil {
		values["missing_bucket"] = *a.missingBucket
	}

	// value_type
	if a.valueType != "" {
		values["value_type"] = a.valueType
	}

	// order
	if a.order != "" {
		values["order"] = a.order
	}

	return source, nil

}

// -- CompositeAggregationHistogramValuesSource --

// CompositeAggregationHistogramValuesSource is a source for the CompositeAggregation that handles histograms
// it works very similar to a terms histogram with slightly different syntax
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-bucket-composite-aggregation.html#_histogram
// for details.
type CompositeAggregationHistogramValuesSource struct {
	name          string
	field         string
	script        *Script
	valueType     string
	missing       interface{}
	missingBucket *bool
	order         string
	interval      float64
}

// NewCompositeAggregationHistogramValuesSource creates and initializes
// a new CompositeAggregationHistogramValuesSource.
func NewCompositeAggregationHistogramValuesSource(name string, interval float64) *CompositeAggregationHistogramValuesSource {
	return &CompositeAggregationHistogramValuesSource{
		name:     name,
		interval: interval,
	}
}

// Field to use for this source.
func (a *CompositeAggregationHistogramValuesSource) Field(field string) *CompositeAggregationHistogramValuesSource {
	a.field = field
	return a
}

// Script to use for this source.
func (a *CompositeAggregationHistogramValuesSource) Script(script *Script) *CompositeAggregationHistogramValuesSource {
	a.script = script
	return a
}

// ValueType specifies the type of values produced by this source,
// e.g. "string" or "date".
func (a *CompositeAggregationHistogramValuesSource) ValueType(valueType string) *CompositeAggregationHistogramValuesSource {
	a.valueType = valueType
	return a
}

// Missing specifies the value to use when the source finds a missing
// value in a document.
//
// Deprecated: Use MissingBucket instead.
func (a *CompositeAggregationHistogramValuesSource) Missing(missing interface{}) *CompositeAggregationHistogramValuesSource {
	a.missing = missing
	return a
}

// MissingBucket, if true, will create an explicit null bucket which represents
// documents with missing values.
func (a *CompositeAggregationHistogramValuesSource) MissingBucket(missingBucket bool) *CompositeAggregationHistogramValuesSource {
	a.missingBucket = &missingBucket
	return a
}

// Order specifies the order in the values produced by this source.
// It can be either "asc" or "desc".
func (a *CompositeAggregationHistogramValuesSource) Order(order string) *CompositeAggregationHistogramValuesSource {
	a.order = order
	return a
}

// Asc ensures the order of the values produced is ascending.
func (a *CompositeAggregationHistogramValuesSource) Asc() *CompositeAggregationHistogramValuesSource {
	a.order = "asc"
	return a
}

// Desc ensures the order of the values produced is descending.
func (a *CompositeAggregationHistogramValuesSource) Desc() *CompositeAggregationHistogramValuesSource {
	a.order = "desc"
	return a
}

// Interval specifies the interval to use.
func (a *CompositeAggregationHistogramValuesSource) Interval(interval float64) *CompositeAggregationHistogramValuesSource {
	a.interval = interval
	return a
}

// Source returns the serializable JSON for this values source.
func (a *CompositeAggregationHistogramValuesSource) Source() (interface{}, error) {
	source := make(map[string]interface{})
	name := make(map[string]interface{})
	source[a.name] = name
	values := make(map[string]interface{})
	name["histogram"] = values

	// field
	if a.field != "" {
		values["field"] = a.field
	}

	// script
	if a.script != nil {
		src, err := a.script.Source()
		if err != nil {
			return nil, err
		}
		values["script"] = src
	}

	// missing
	if a.missing != nil {
		values["missing"] = a.missing
	}

	// missing_bucket
	if a.missingBucket != nil {
		values["missing_bucket"] = *a.missingBucket
	}

	// value_type
	if a.valueType != "" {
		values["value_type"] = a.valueType
	}

	// order
	if a.order != "" {
		values["order"] = a.order
	}

	// Histogram-related properties
	values["interval"] = a.interval

	return source, nil

}

// -- CompositeAggregationDateHistogramValuesSource --

// CompositeAggregationDateHistogramValuesSource is a source for the CompositeAggregation that handles date histograms
// it works very similar to a date histogram aggregation with slightly different syntax
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.4/search-aggregations-bucket-composite-aggregation.html#_date_histogram
// for details.
type CompositeAggregationDateHistogramValuesSource struct {
	name             string
	field            string
	script           *Script
	valueType        string
	missing          interface{}
	missingBucket    *bool
	order            string
	interval         interface{}
	fixedInterval    interface{}
	calendarInterval interface{}
	format           string
	timeZone         string
}

// NewCompositeAggregationDateHistogramValuesSource creates and initializes
// a new CompositeAggregationDateHistogramValuesSource.
func NewCompositeAggregationDateHistogramValuesSource(name string) *CompositeAggregationDateHistogramValuesSource {
	return &CompositeAggregationDateHistogramValuesSource{
		name: name,
	}
}

// Field to use for this source.
func (a *CompositeAggregationDateHistogramValuesSource) Field(field string) *CompositeAggregationDateHistogramValuesSource {
	a.field = field
	return a
}

// Script to use for this source.
func (a *CompositeAggregationDateHistogramValuesSource) Script(script *Script) *CompositeAggregationDateHistogramValuesSource {
	a.script = script
	return a
}

// ValueType specifies the type of values produced by this source,
// e.g. "string" or "date".
func (a *CompositeAggregationDateHistogramValuesSource) ValueType(valueType string) *CompositeAggregationDateHistogramValuesSource {
	a.valueType = valueType
	return a
}

// Missing specifies the value to use when the source finds a missing
// value in a document.
//
// Deprecated: Use MissingBucket instead.
func (a *CompositeAggregationDateHistogramValuesSource) Missing(missing interface{}) *CompositeAggregationDateHistogramValuesSource {
	a.missing = missing
	return a
}

// MissingBucket, if true, will create an explicit null bucket which represents
// documents with missing values.
func (a *CompositeAggregationDateHistogramValuesSource) MissingBucket(missingBucket bool) *CompositeAggregationDateHistogramValuesSource {
	a.missingBucket = &missingBucket
	return a
}

// Order specifies the order in the values produced by this source.
// It can be either "asc" or "desc".
func (a *CompositeAggregationDateHistogramValuesSource) Order(order string) *CompositeAggregationDateHistogramValuesSource {
	a.order = order
	return a
}

// Asc ensures the order of the values produced is ascending.
func (a *CompositeAggregationDateHistogramValuesSource) Asc() *CompositeAggregationDateHistogramValuesSource {
	a.order = "asc"
	return a
}

// Desc ensures the order of the values produced is descending.
func (a *CompositeAggregationDateHistogramValuesSource) Desc() *CompositeAggregationDateHistogramValuesSource {
	a.order = "desc"
	return a
}

// Interval to use for the date histogram, e.g. "1d" or a numeric value like "60".
//
// Deprecated: Use FixedInterval or CalendarInterval instead.
func (a *CompositeAggregationDateHistogramValuesSource) Interval(interval interface{}) *CompositeAggregationDateHistogramValuesSource {
	a.interval = interval
	return a
}

// FixedInterval to use for the date histogram, e.g. "1d" or a numeric value like "60".
func (a *CompositeAggregationDateHistogramValuesSource) FixedInterval(fixedInterval interface{}) *CompositeAggregationDateHistogramValuesSource {
	a.fixedInterval = fixedInterval
	return a
}

// CalendarInterval to use for the date histogram, e.g. "1d" or a numeric value like "60".
func (a *CompositeAggregationDateHistogramValuesSource) CalendarInterval(calendarInterval interface{}) *CompositeAggregationDateHistogramValuesSource {
	a.calendarInterval = calendarInterval
	return a
}

// Format to use for the date histogram, e.g. "strict_date_optional_time"
func (a *CompositeAggregationDateHistogramValuesSource) Format(format string) *CompositeAggregationDateHistogramValuesSource {
	a.format = format
	return a
}

// TimeZone to use for the dates.
func (a *CompositeAggregationDateHistogramValuesSource) TimeZone(timeZone string) *CompositeAggregationDateHistogramValuesSource {
	a.timeZone = timeZone
	return a
}

// Source returns the serializable JSON for this values source.
func (a *CompositeAggregationDateHistogramValuesSource) Source() (interface{}, error) {
	source := make(map[string]interface{})
	name := make(map[string]interface{})
	source[a.name] = name
	values := make(map[string]interface{})
	name["date_histogram"] = values

	// field
	if a.field != "" {
		values["field"] = a.field
	}

	// script
	if a.script != nil {
		src, err := a.script.Source()
		if err != nil {
			return nil, err
		}
		values["script"] = src
	}

	// missing
	if a.missing != nil {
		values["missing"] = a.missing
	}

	// missing_bucket
	if a.missingBucket != nil {
		values["missing_bucket"] = *a.missingBucket
	}

	// value_type
	if a.valueType != "" {
		values["value_type"] = a.valueType
	}

	// order
	if a.order != "" {
		values["order"] = a.order
	}

	if a.format != "" {
		values["format"] = a.format
	}

	// DateHistogram-related properties
	if v := a.interval; v != nil {
		values["interval"] = v
	}
	if v := a.fixedInterval; v != nil {
		values["fixed_interval"] = v
	}
	if v := a.calendarInterval; v != nil {
		values["calendar_interval"] = v
	}

	// timeZone
	if a.timeZone != "" {
		values["time_zone"] = a.timeZone
	}

	return source, nil
}
