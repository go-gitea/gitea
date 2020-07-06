// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// ExtendedStatsBucketAggregation is a sibling pipeline aggregation which calculates
// a variety of stats across all bucket of a specified metric in a sibling aggregation.
// The specified metric must be numeric and the sibling aggregation must
// be a multi-bucket aggregation.
//
// This aggregation provides a few more statistics (sum of squares, standard deviation, etc)
// compared to the stats_bucket aggregation.
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-pipeline-extended-stats-bucket-aggregation.html
type ExtendedStatsBucketAggregation struct {
	format       string
	gapPolicy    string
	sigma        *float32
	meta         map[string]interface{}
	bucketsPaths []string
}

// NewExtendedStatsBucketAggregation creates and initializes a new ExtendedStatsBucketAggregation.
func NewExtendedStatsBucketAggregation() *ExtendedStatsBucketAggregation {
	return &ExtendedStatsBucketAggregation{
		bucketsPaths: make([]string, 0),
	}
}

// Format to use on the output of this aggregation.
func (s *ExtendedStatsBucketAggregation) Format(format string) *ExtendedStatsBucketAggregation {
	s.format = format
	return s
}

// GapPolicy defines what should be done when a gap in the series is discovered.
// Valid values include "insert_zeros" or "skip". Default is "insert_zeros".
func (s *ExtendedStatsBucketAggregation) GapPolicy(gapPolicy string) *ExtendedStatsBucketAggregation {
	s.gapPolicy = gapPolicy
	return s
}

// GapInsertZeros inserts zeros for gaps in the series.
func (s *ExtendedStatsBucketAggregation) GapInsertZeros() *ExtendedStatsBucketAggregation {
	s.gapPolicy = "insert_zeros"
	return s
}

// GapSkip skips gaps in the series.
func (s *ExtendedStatsBucketAggregation) GapSkip() *ExtendedStatsBucketAggregation {
	s.gapPolicy = "skip"
	return s
}

// Meta sets the meta data to be included in the aggregation response.
func (s *ExtendedStatsBucketAggregation) Meta(metaData map[string]interface{}) *ExtendedStatsBucketAggregation {
	s.meta = metaData
	return s
}

// BucketsPath sets the paths to the buckets to use for this pipeline aggregator.
func (s *ExtendedStatsBucketAggregation) BucketsPath(bucketsPaths ...string) *ExtendedStatsBucketAggregation {
	s.bucketsPaths = append(s.bucketsPaths, bucketsPaths...)
	return s
}

// Sigma sets number of standard deviations above/below the mean to display
func (s *ExtendedStatsBucketAggregation) Sigma(sigma float32) *ExtendedStatsBucketAggregation {
	s.sigma = &sigma
	return s
}

// Source returns the a JSON-serializable interface.
func (s *ExtendedStatsBucketAggregation) Source() (interface{}, error) {
	source := make(map[string]interface{})
	params := make(map[string]interface{})
	source["extended_stats_bucket"] = params

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

	// Add sigma is not zero or less
	if s.sigma != nil && *s.sigma >= 0 {
		params["sigma"] = *s.sigma
	}

	// Add Meta data if available
	if len(s.meta) > 0 {
		source["meta"] = s.meta
	}

	return source, nil
}
