// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// BucketSortAggregation parent pipeline aggregation which sorts the buckets
// of its parent multi-bucket aggregation. Zero or more sort fields may be
// specified together with the corresponding sort order. Each bucket may be
// sorted based on its _key, _count or its sub-aggregations. In addition,
// parameters from and size may be set in order to truncate the result buckets.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-pipeline-bucket-sort-aggregation.html
type BucketSortAggregation struct {
	sorters   []Sorter
	from      int
	size      int
	gapPolicy string

	meta map[string]interface{}
}

// NewBucketSortAggregation creates and initializes a new BucketSortAggregation.
func NewBucketSortAggregation() *BucketSortAggregation {
	return &BucketSortAggregation{
		size: -1,
	}
}

// Sort adds a sort order to the list of sorters.
func (a *BucketSortAggregation) Sort(field string, ascending bool) *BucketSortAggregation {
	a.sorters = append(a.sorters, SortInfo{Field: field, Ascending: ascending})
	return a
}

// SortWithInfo adds a SortInfo to the list of sorters.
func (a *BucketSortAggregation) SortWithInfo(info SortInfo) *BucketSortAggregation {
	a.sorters = append(a.sorters, info)
	return a
}

// From adds the "from" parameter to the aggregation.
func (a *BucketSortAggregation) From(from int) *BucketSortAggregation {
	a.from = from
	return a
}

// Size adds the "size" parameter to the aggregation.
func (a *BucketSortAggregation) Size(size int) *BucketSortAggregation {
	a.size = size
	return a
}

// GapPolicy defines what should be done when a gap in the series is discovered.
// Valid values include "insert_zeros" or "skip". Default is "skip".
func (a *BucketSortAggregation) GapPolicy(gapPolicy string) *BucketSortAggregation {
	a.gapPolicy = gapPolicy
	return a
}

// GapInsertZeros inserts zeros for gaps in the series.
func (a *BucketSortAggregation) GapInsertZeros() *BucketSortAggregation {
	a.gapPolicy = "insert_zeros"
	return a
}

// GapSkip skips gaps in the series.
func (a *BucketSortAggregation) GapSkip() *BucketSortAggregation {
	a.gapPolicy = "skip"
	return a
}

// Meta sets the meta data in the aggregation.
// Although metadata is supported for this aggregation by Elasticsearch, it's important to
// note that there's no use to it because this aggregation does not include new data in the
// response. It merely reorders parent buckets.
func (a *BucketSortAggregation) Meta(meta map[string]interface{}) *BucketSortAggregation {
	a.meta = meta
	return a
}

// Source returns the a JSON-serializable interface.
func (a *BucketSortAggregation) Source() (interface{}, error) {
	source := make(map[string]interface{})
	params := make(map[string]interface{})
	source["bucket_sort"] = params

	if a.from != 0 {
		params["from"] = a.from
	}
	if a.size != -1 {
		params["size"] = a.size
	}

	if a.gapPolicy != "" {
		params["gap_policy"] = a.gapPolicy
	}

	// Parses sorters to JSON-serializable interface.
	if len(a.sorters) > 0 {
		sorters := make([]interface{}, len(a.sorters))
		params["sort"] = sorters
		for idx, sorter := range a.sorters {
			src, err := sorter.Source()
			if err != nil {
				return nil, err
			}
			sorters[idx] = src
		}
	}

	// Add metadata if available.
	if len(a.meta) > 0 {
		source["meta"] = a.meta
	}

	return source, nil
}
