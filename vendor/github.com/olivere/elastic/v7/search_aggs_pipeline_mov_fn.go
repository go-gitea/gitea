// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// MovFnAggregation, given an ordered series of data, will slice a window across
// the data and allow the user to specify a custom script that is executed for
// each window of data.
//
// You must pass a script to process the values. There are a number of predefined
// script functions you can use as described here:
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-pipeline-movfn-aggregation.html#_pre_built_functions.
//
// Example:
//   agg := elastic.NewMovFnAggregation(
//     "the_sum", // bucket path
//     elastic.NewScript("MovingFunctions.stdDev(values, MovingFunctions.unweightedAvg(values))"),
//     10,        // window size
//   )
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-pipeline-movfn-aggregation.html.
type MovFnAggregation struct {
	script    *Script
	format    string
	gapPolicy string
	window    int

	meta         map[string]interface{}
	bucketsPaths []string
}

// NewMovFnAggregation creates and initializes a new MovFnAggregation.
//
// Deprecated: The MovFnAggregation has been deprecated in 6.4.0. Use the more generate MovFnAggregation instead.
func NewMovFnAggregation(bucketsPath string, script *Script, window int) *MovFnAggregation {
	return &MovFnAggregation{
		bucketsPaths: []string{bucketsPath},
		script:       script,
		window:       window,
	}
}

// Script is the script to run.
func (a *MovFnAggregation) Script(script *Script) *MovFnAggregation {
	a.script = script
	return a
}

// Format to use on the output of this aggregation.
func (a *MovFnAggregation) Format(format string) *MovFnAggregation {
	a.format = format
	return a
}

// GapPolicy defines what should be done when a gap in the series is discovered.
// Valid values include "insert_zeros" or "skip". Default is "insert_zeros".
func (a *MovFnAggregation) GapPolicy(gapPolicy string) *MovFnAggregation {
	a.gapPolicy = gapPolicy
	return a
}

// GapInsertZeros inserts zeros for gaps in the series.
func (a *MovFnAggregation) GapInsertZeros() *MovFnAggregation {
	a.gapPolicy = "insert_zeros"
	return a
}

// GapSkip skips gaps in the series.
func (a *MovFnAggregation) GapSkip() *MovFnAggregation {
	a.gapPolicy = "skip"
	return a
}

// Window sets the window size for this aggregation.
func (a *MovFnAggregation) Window(window int) *MovFnAggregation {
	a.window = window
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *MovFnAggregation) Meta(metaData map[string]interface{}) *MovFnAggregation {
	a.meta = metaData
	return a
}

// BucketsPath sets the paths to the buckets to use for this pipeline aggregator.
func (a *MovFnAggregation) BucketsPath(bucketsPaths ...string) *MovFnAggregation {
	a.bucketsPaths = append(a.bucketsPaths, bucketsPaths...)
	return a
}

// Source returns the a JSON-serializable interface.
func (a *MovFnAggregation) Source() (interface{}, error) {
	source := make(map[string]interface{})
	params := make(map[string]interface{})
	source["moving_fn"] = params

	// Add buckets paths
	switch len(a.bucketsPaths) {
	case 0:
	case 1:
		params["buckets_path"] = a.bucketsPaths[0]
	default:
		params["buckets_path"] = a.bucketsPaths
	}

	// Script
	if a.script != nil {
		src, err := a.script.Source()
		if err != nil {
			return nil, err
		}
		params["script"] = src
	}

	if a.format != "" {
		params["format"] = a.format
	}
	if a.gapPolicy != "" {
		params["gap_policy"] = a.gapPolicy
	}
	params["window"] = a.window

	// Add Meta data if available
	if len(a.meta) > 0 {
		source["meta"] = a.meta
	}

	return source, nil
}
