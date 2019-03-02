// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// DiversifiedSamplerAggregation Like the ‘sampler` aggregation this is a filtering aggregation used to limit any
// sub aggregations’ processing to a sample of the top-scoring documents. The diversified_sampler aggregation adds
// the ability to limit the number of matches that share a common value such as an "author".
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/5.6/search-aggregations-bucket-diversified-sampler-aggregation.html
type DiversifiedSamplerAggregation struct {
	subAggregations map[string]Aggregation
	meta            map[string]interface{}
	field           string
	script          *Script
	shardSize       int
	maxDocsPerValue int
	executionHint   string
}

func NewDiversifiedSamplerAggregation() *DiversifiedSamplerAggregation {
	return &DiversifiedSamplerAggregation{
		shardSize:       -1,
		maxDocsPerValue: -1,
		subAggregations: make(map[string]Aggregation),
	}
}

func (a *DiversifiedSamplerAggregation) SubAggregation(name string, subAggregation Aggregation) *DiversifiedSamplerAggregation {
	a.subAggregations[name] = subAggregation
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *DiversifiedSamplerAggregation) Meta(metaData map[string]interface{}) *DiversifiedSamplerAggregation {
	a.meta = metaData
	return a
}

// Field on which the aggregation is processed.
func (a *DiversifiedSamplerAggregation) Field(field string) *DiversifiedSamplerAggregation {
	a.field = field
	return a
}

func (a *DiversifiedSamplerAggregation) Script(script *Script) *DiversifiedSamplerAggregation {
	a.script = script
	return a
}

// ShardSize sets the maximum number of docs returned from each shard.
func (a *DiversifiedSamplerAggregation) ShardSize(shardSize int) *DiversifiedSamplerAggregation {
	a.shardSize = shardSize
	return a
}

func (a *DiversifiedSamplerAggregation) MaxDocsPerValue(maxDocsPerValue int) *DiversifiedSamplerAggregation {
	a.maxDocsPerValue = maxDocsPerValue
	return a
}

func (a *DiversifiedSamplerAggregation) ExecutionHint(hint string) *DiversifiedSamplerAggregation {
	a.executionHint = hint
	return a
}

func (a *DiversifiedSamplerAggregation) Source() (interface{}, error) {
	// Example:
	// {
	//     "aggs": {
	//			"my_unbiased_sample": {
	//				"diversified_sampler": {
	//					"shard_size": 200,
	//					"field" : "author"
	//				}
	//			}
	//		}
	// }
	//
	// This method returns only the { "diversified_sampler" : { ... } } part.

	source := make(map[string]interface{})
	opts := make(map[string]interface{})
	source["diversified_sampler"] = opts

	if a.field != "" {
		opts["field"] = a.field
	}
	if a.script != nil {
		src, err := a.script.Source()
		if err != nil {
			return nil, err
		}
		opts["script"] = src
	}
	if a.shardSize >= 0 {
		opts["shard_size"] = a.shardSize
	}
	if a.maxDocsPerValue >= 0 {
		opts["max_docs_per_value"] = a.maxDocsPerValue
	}
	if a.executionHint != "" {
		opts["execution_hint"] = a.executionHint
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
