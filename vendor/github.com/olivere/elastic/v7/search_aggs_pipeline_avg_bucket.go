// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// AvgBucketAggregation is a sibling pipeline aggregation which calculates
// the (mean) average value of a specified metric in a sibling aggregation.
// The specified metric must be numeric and the sibling aggregation must
// be a multi-bucket aggregation.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-pipeline-avg-bucket-aggregation.html
type AvgBucketAggregation struct {
	format    string
	gapPolicy string

	meta         map[string]interface{}
	bucketsPaths []string
}

// NewAvgBucketAggregation creates and initializes a new AvgBucketAggregation.
func NewAvgBucketAggregation() *AvgBucketAggregation {
	return &AvgBucketAggregation{
		bucketsPaths: make([]string, 0),
	}
}

// Format to use on the output of this aggregation.
func (a *AvgBucketAggregation) Format(format string) *AvgBucketAggregation {
	a.format = format
	return a
}

// GapPolicy defines what should be done when a gap in the series is discovered.
// Valid values include "insert_zeros" or "skip". Default is "insert_zeros".
func (a *AvgBucketAggregation) GapPolicy(gapPolicy string) *AvgBucketAggregation {
	a.gapPolicy = gapPolicy
	return a
}

// GapInsertZeros inserts zeros for gaps in the series.
func (a *AvgBucketAggregation) GapInsertZeros() *AvgBucketAggregation {
	a.gapPolicy = "insert_zeros"
	return a
}

// GapSkip skips gaps in the series.
func (a *AvgBucketAggregation) GapSkip() *AvgBucketAggregation {
	a.gapPolicy = "skip"
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *AvgBucketAggregation) Meta(metaData map[string]interface{}) *AvgBucketAggregation {
	a.meta = metaData
	return a
}

// BucketsPath sets the paths to the buckets to use for this pipeline aggregator.
func (a *AvgBucketAggregation) BucketsPath(bucketsPaths ...string) *AvgBucketAggregation {
	a.bucketsPaths = append(a.bucketsPaths, bucketsPaths...)
	return a
}

// Source returns the a JSON-serializable interface.
func (a *AvgBucketAggregation) Source() (interface{}, error) {
	source := make(map[string]interface{})
	params := make(map[string]interface{})
	source["avg_bucket"] = params

	if a.format != "" {
		params["format"] = a.format
	}
	if a.gapPolicy != "" {
		params["gap_policy"] = a.gapPolicy
	}

	// Add buckets paths
	switch len(a.bucketsPaths) {
	case 0:
	case 1:
		params["buckets_path"] = a.bucketsPaths[0]
	default:
		params["buckets_path"] = a.bucketsPaths
	}

	// Add Meta data if available
	if len(a.meta) > 0 {
		source["meta"] = a.meta
	}

	return source, nil
}
