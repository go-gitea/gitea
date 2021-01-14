// Copyright 2012-2015 Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// PercentilesBucketAggregation is a sibling pipeline aggregation which calculates
// percentiles across all bucket of a specified metric in a sibling aggregation.
// The specified metric must be numeric and the sibling aggregation must
// be a multi-bucket aggregation.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-pipeline-percentiles-bucket-aggregation.html
type PercentilesBucketAggregation struct {
	format       string
	gapPolicy    string
	percents     []float64
	bucketsPaths []string

	meta map[string]interface{}
}

// NewPercentilesBucketAggregation creates and initializes a new PercentilesBucketAggregation.
func NewPercentilesBucketAggregation() *PercentilesBucketAggregation {
	return &PercentilesBucketAggregation{}
}

// Format to apply the output value of this aggregation.
func (p *PercentilesBucketAggregation) Format(format string) *PercentilesBucketAggregation {
	p.format = format
	return p
}

// Percents to calculate percentiles for in this aggregation.
func (p *PercentilesBucketAggregation) Percents(percents ...float64) *PercentilesBucketAggregation {
	p.percents = percents
	return p
}

// GapPolicy defines what should be done when a gap in the series is discovered.
// Valid values include "insert_zeros" or "skip". Default is "insert_zeros".
func (p *PercentilesBucketAggregation) GapPolicy(gapPolicy string) *PercentilesBucketAggregation {
	p.gapPolicy = gapPolicy
	return p
}

// GapInsertZeros inserts zeros for gaps in the series.
func (p *PercentilesBucketAggregation) GapInsertZeros() *PercentilesBucketAggregation {
	p.gapPolicy = "insert_zeros"
	return p
}

// GapSkip skips gaps in the series.
func (p *PercentilesBucketAggregation) GapSkip() *PercentilesBucketAggregation {
	p.gapPolicy = "skip"
	return p
}

// Meta sets the meta data to be included in the aggregation response.
func (p *PercentilesBucketAggregation) Meta(metaData map[string]interface{}) *PercentilesBucketAggregation {
	p.meta = metaData
	return p
}

// BucketsPath sets the paths to the buckets to use for this pipeline aggregator.
func (p *PercentilesBucketAggregation) BucketsPath(bucketsPaths ...string) *PercentilesBucketAggregation {
	p.bucketsPaths = append(p.bucketsPaths, bucketsPaths...)
	return p
}

// Source returns the a JSON-serializable interface.
func (p *PercentilesBucketAggregation) Source() (interface{}, error) {
	source := make(map[string]interface{})
	params := make(map[string]interface{})
	source["percentiles_bucket"] = params

	if p.format != "" {
		params["format"] = p.format
	}
	if p.gapPolicy != "" {
		params["gap_policy"] = p.gapPolicy
	}

	// Add buckets paths
	switch len(p.bucketsPaths) {
	case 0:
	case 1:
		params["buckets_path"] = p.bucketsPaths[0]
	default:
		params["buckets_path"] = p.bucketsPaths
	}

	// Add percents
	if len(p.percents) > 0 {
		params["percents"] = p.percents
	}

	// Add Meta data if available
	if len(p.meta) > 0 {
		source["meta"] = p.meta
	}

	return source, nil
}
