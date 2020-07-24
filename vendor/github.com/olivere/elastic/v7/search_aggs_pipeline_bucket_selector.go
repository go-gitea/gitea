// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// BucketSelectorAggregation is a parent pipeline aggregation which
// determines whether the current bucket will be retained in the parent
// multi-bucket aggregation. The specific metric must be numeric and
// the script must return a boolean value. If the script language is
// expression then a numeric return value is permitted. In this case 0.0
// will be evaluated as false and all other values will evaluate to true.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-pipeline-bucket-selector-aggregation.html
type BucketSelectorAggregation struct {
	format    string
	gapPolicy string
	script    *Script

	meta            map[string]interface{}
	bucketsPathsMap map[string]string
}

// NewBucketSelectorAggregation creates and initializes a new BucketSelectorAggregation.
func NewBucketSelectorAggregation() *BucketSelectorAggregation {
	return &BucketSelectorAggregation{
		bucketsPathsMap: make(map[string]string),
	}
}

// Format to use on the output of this aggregation.
func (a *BucketSelectorAggregation) Format(format string) *BucketSelectorAggregation {
	a.format = format
	return a
}

// GapPolicy defines what should be done when a gap in the series is discovered.
// Valid values include "insert_zeros" or "skip". Default is "insert_zeros".
func (a *BucketSelectorAggregation) GapPolicy(gapPolicy string) *BucketSelectorAggregation {
	a.gapPolicy = gapPolicy
	return a
}

// GapInsertZeros inserts zeros for gaps in the series.
func (a *BucketSelectorAggregation) GapInsertZeros() *BucketSelectorAggregation {
	a.gapPolicy = "insert_zeros"
	return a
}

// GapSkip skips gaps in the series.
func (a *BucketSelectorAggregation) GapSkip() *BucketSelectorAggregation {
	a.gapPolicy = "skip"
	return a
}

// Script is the script to run.
func (a *BucketSelectorAggregation) Script(script *Script) *BucketSelectorAggregation {
	a.script = script
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *BucketSelectorAggregation) Meta(metaData map[string]interface{}) *BucketSelectorAggregation {
	a.meta = metaData
	return a
}

// BucketsPathsMap sets the paths to the buckets to use for this pipeline aggregator.
func (a *BucketSelectorAggregation) BucketsPathsMap(bucketsPathsMap map[string]string) *BucketSelectorAggregation {
	a.bucketsPathsMap = bucketsPathsMap
	return a
}

// AddBucketsPath adds a bucket path to use for this pipeline aggregator.
func (a *BucketSelectorAggregation) AddBucketsPath(name, path string) *BucketSelectorAggregation {
	if a.bucketsPathsMap == nil {
		a.bucketsPathsMap = make(map[string]string)
	}
	a.bucketsPathsMap[name] = path
	return a
}

// Source returns the a JSON-serializable interface.
func (a *BucketSelectorAggregation) Source() (interface{}, error) {
	source := make(map[string]interface{})
	params := make(map[string]interface{})
	source["bucket_selector"] = params

	if a.format != "" {
		params["format"] = a.format
	}
	if a.gapPolicy != "" {
		params["gap_policy"] = a.gapPolicy
	}
	if a.script != nil {
		src, err := a.script.Source()
		if err != nil {
			return nil, err
		}
		params["script"] = src
	}

	// Add buckets paths
	if len(a.bucketsPathsMap) > 0 {
		params["buckets_path"] = a.bucketsPathsMap
	}

	// Add Meta data if available
	if len(a.meta) > 0 {
		source["meta"] = a.meta
	}

	return source, nil
}
