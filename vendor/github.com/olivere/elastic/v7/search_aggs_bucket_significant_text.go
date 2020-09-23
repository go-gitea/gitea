// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// SignificantTextAggregation returns interesting or unusual occurrences
// of free-text terms in a set.
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-bucket-significanttext-aggregation.html
type SignificantTextAggregation struct {
	field           string
	subAggregations map[string]Aggregation
	meta            map[string]interface{}

	sourceFieldNames      []string
	filterDuplicateText   *bool
	includeExclude        *TermsAggregationIncludeExclude
	filter                Query
	bucketCountThresholds *BucketCountThresholds
	significanceHeuristic SignificanceHeuristic
}

func NewSignificantTextAggregation() *SignificantTextAggregation {
	return &SignificantTextAggregation{
		subAggregations: make(map[string]Aggregation),
	}
}

func (a *SignificantTextAggregation) Field(field string) *SignificantTextAggregation {
	a.field = field
	return a
}

func (a *SignificantTextAggregation) SubAggregation(name string, subAggregation Aggregation) *SignificantTextAggregation {
	a.subAggregations[name] = subAggregation
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *SignificantTextAggregation) Meta(metaData map[string]interface{}) *SignificantTextAggregation {
	a.meta = metaData
	return a
}

func (a *SignificantTextAggregation) SourceFieldNames(names ...string) *SignificantTextAggregation {
	a.sourceFieldNames = names
	return a
}

func (a *SignificantTextAggregation) FilterDuplicateText(filter bool) *SignificantTextAggregation {
	a.filterDuplicateText = &filter
	return a
}

func (a *SignificantTextAggregation) MinDocCount(minDocCount int64) *SignificantTextAggregation {
	if a.bucketCountThresholds == nil {
		a.bucketCountThresholds = &BucketCountThresholds{}
	}
	a.bucketCountThresholds.MinDocCount = &minDocCount
	return a
}

func (a *SignificantTextAggregation) ShardMinDocCount(shardMinDocCount int64) *SignificantTextAggregation {
	if a.bucketCountThresholds == nil {
		a.bucketCountThresholds = &BucketCountThresholds{}
	}
	a.bucketCountThresholds.ShardMinDocCount = &shardMinDocCount
	return a
}

func (a *SignificantTextAggregation) Size(size int) *SignificantTextAggregation {
	if a.bucketCountThresholds == nil {
		a.bucketCountThresholds = &BucketCountThresholds{}
	}
	a.bucketCountThresholds.RequiredSize = &size
	return a
}

func (a *SignificantTextAggregation) ShardSize(shardSize int) *SignificantTextAggregation {
	if a.bucketCountThresholds == nil {
		a.bucketCountThresholds = &BucketCountThresholds{}
	}
	a.bucketCountThresholds.ShardSize = &shardSize
	return a
}

func (a *SignificantTextAggregation) BackgroundFilter(filter Query) *SignificantTextAggregation {
	a.filter = filter
	return a
}

func (a *SignificantTextAggregation) SignificanceHeuristic(heuristic SignificanceHeuristic) *SignificantTextAggregation {
	a.significanceHeuristic = heuristic
	return a
}

func (a *SignificantTextAggregation) Include(regexp string) *SignificantTextAggregation {
	if a.includeExclude == nil {
		a.includeExclude = &TermsAggregationIncludeExclude{}
	}
	a.includeExclude.Include = regexp
	return a
}

func (a *SignificantTextAggregation) IncludeValues(values ...interface{}) *SignificantTextAggregation {
	if a.includeExclude == nil {
		a.includeExclude = &TermsAggregationIncludeExclude{}
	}
	a.includeExclude.IncludeValues = append(a.includeExclude.IncludeValues, values...)
	return a
}

func (a *SignificantTextAggregation) Exclude(regexp string) *SignificantTextAggregation {
	if a.includeExclude == nil {
		a.includeExclude = &TermsAggregationIncludeExclude{}
	}
	a.includeExclude.Exclude = regexp
	return a
}

func (a *SignificantTextAggregation) ExcludeValues(values ...interface{}) *SignificantTextAggregation {
	if a.includeExclude == nil {
		a.includeExclude = &TermsAggregationIncludeExclude{}
	}
	a.includeExclude.ExcludeValues = append(a.includeExclude.ExcludeValues, values...)
	return a
}

func (a *SignificantTextAggregation) Partition(p int) *SignificantTextAggregation {
	if a.includeExclude == nil {
		a.includeExclude = &TermsAggregationIncludeExclude{}
	}
	a.includeExclude.Partition = p
	return a
}

func (a *SignificantTextAggregation) NumPartitions(n int) *SignificantTextAggregation {
	if a.includeExclude == nil {
		a.includeExclude = &TermsAggregationIncludeExclude{}
	}
	a.includeExclude.NumPartitions = n
	return a
}

func (a *SignificantTextAggregation) IncludeExclude(includeExclude *TermsAggregationIncludeExclude) *SignificantTextAggregation {
	a.includeExclude = includeExclude
	return a
}

func (a *SignificantTextAggregation) Source() (interface{}, error) {
	// Example:
	// {
	//     "query" : {
	//         "match" : {"content" : "Bird flu"}
	//     },
	//     "aggregations" : {
	//         "my_sample" : {
	//             "sampler": {
	//                 "shard_size" : 100
	//             },
	//             "aggregations": {
	//                 "keywords" : {
	//                     "significant_text" : { "field" : "content" }
	//                 }
	//             }
	//         }
	//     }
	// }
	//
	// This method returns only the
	//   { "significant_text" : { "field" : "content" }
	// part.

	source := make(map[string]interface{})
	opts := make(map[string]interface{})
	source["significant_text"] = opts

	if a.field != "" {
		opts["field"] = a.field
	}
	if a.bucketCountThresholds != nil {
		if a.bucketCountThresholds.RequiredSize != nil {
			opts["size"] = (*a.bucketCountThresholds).RequiredSize
		}
		if a.bucketCountThresholds.ShardSize != nil {
			opts["shard_size"] = (*a.bucketCountThresholds).ShardSize
		}
		if a.bucketCountThresholds.MinDocCount != nil {
			opts["min_doc_count"] = (*a.bucketCountThresholds).MinDocCount
		}
		if a.bucketCountThresholds.ShardMinDocCount != nil {
			opts["shard_min_doc_count"] = (*a.bucketCountThresholds).ShardMinDocCount
		}
	}
	if a.filter != nil {
		src, err := a.filter.Source()
		if err != nil {
			return nil, err
		}
		opts["background_filter"] = src
	}
	if a.significanceHeuristic != nil {
		name := a.significanceHeuristic.Name()
		src, err := a.significanceHeuristic.Source()
		if err != nil {
			return nil, err
		}
		opts[name] = src
	}
	// Include/Exclude
	if ie := a.includeExclude; ie != nil {
		// Include
		if ie.Include != "" {
			opts["include"] = ie.Include
		} else if len(ie.IncludeValues) > 0 {
			opts["include"] = ie.IncludeValues
		} else if ie.NumPartitions > 0 {
			inc := make(map[string]interface{})
			inc["partition"] = ie.Partition
			inc["num_partitions"] = ie.NumPartitions
			opts["include"] = inc
		}
		// Exclude
		if ie.Exclude != "" {
			opts["exclude"] = ie.Exclude
		} else if len(ie.ExcludeValues) > 0 {
			opts["exclude"] = ie.ExcludeValues
		}
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
