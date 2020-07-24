// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// StatsBucketAggregation is a sibling pipeline aggregation which calculates
// a variety of stats across all bucket of a specified metric in a sibling aggregation.
// The specified metric must be numeric and the sibling aggregation must
// be a multi-bucket aggregation.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-pipeline-stats-bucket-aggregation.html
type StatsBucketAggregation struct {
	format    string
	gapPolicy string

	meta         map[string]interface{}
	bucketsPaths []string
}

// NewStatsBucketAggregation creates and initializes a new StatsBucketAggregation.
func NewStatsBucketAggregation() *StatsBucketAggregation {
	return &StatsBucketAggregation{
		bucketsPaths: make([]string, 0),
	}
}

// Format to use on the output of this aggregation.
func (s *StatsBucketAggregation) Format(format string) *StatsBucketAggregation {
	s.format = format
	return s
}

// GapPolicy defines what should be done when a gap in the series is discovered.
// Valid values include "insert_zeros" or "skip". Default is "insert_zeros".
func (s *StatsBucketAggregation) GapPolicy(gapPolicy string) *StatsBucketAggregation {
	s.gapPolicy = gapPolicy
	return s
}

// GapInsertZeros inserts zeros for gaps in the series.
func (s *StatsBucketAggregation) GapInsertZeros() *StatsBucketAggregation {
	s.gapPolicy = "insert_zeros"
	return s
}

// GapSkip skips gaps in the series.
func (s *StatsBucketAggregation) GapSkip() *StatsBucketAggregation {
	s.gapPolicy = "skip"
	return s
}

// Meta sets the meta data to be included in the aggregation response.
func (s *StatsBucketAggregation) Meta(metaData map[string]interface{}) *StatsBucketAggregation {
	s.meta = metaData
	return s
}

// BucketsPath sets the paths to the buckets to use for this pipeline aggregator.
func (s *StatsBucketAggregation) BucketsPath(bucketsPaths ...string) *StatsBucketAggregation {
	s.bucketsPaths = append(s.bucketsPaths, bucketsPaths...)
	return s
}

// Source returns the a JSON-serializable interface.
func (s *StatsBucketAggregation) Source() (interface{}, error) {
	source := make(map[string]interface{})
	params := make(map[string]interface{})
	source["stats_bucket"] = params

	if s.format != "" {
		params["format"] = s.format
	}
	if s.gapPolicy != "" {
		params["gap_policy"] = s.gapPolicy
	}

	// Add buckets paths
	switch len(s.bucketsPaths) {
	case 0:
	case 1:
		params["buckets_path"] = s.bucketsPaths[0]
	default:
		params["buckets_path"] = s.bucketsPaths
	}

	// Add Meta data if available
	if len(s.meta) > 0 {
		source["meta"] = s.meta
	}

	return source, nil
}
