// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// RareTermsAggregation is a multi-bucket value source based aggregation
// which finds "rare" terms — terms that are at the long-tail of the distribution
// and are not frequent. Conceptually, this is like a terms aggregation that
// is sorted by _count ascending.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/current/search-aggregations-bucket-rare-terms-aggregation.html
// for details.
type RareTermsAggregation struct {
	field           string
	subAggregations map[string]Aggregation
	meta            map[string]interface{}

	includeExclude *TermsAggregationIncludeExclude
	maxDocCount    *int
	precision      *float64
	missing        interface{}
}

func NewRareTermsAggregation() *RareTermsAggregation {
	return &RareTermsAggregation{
		subAggregations: make(map[string]Aggregation),
	}
}

func (a *RareTermsAggregation) Field(field string) *RareTermsAggregation {
	a.field = field
	return a
}

func (a *RareTermsAggregation) SubAggregation(name string, subAggregation Aggregation) *RareTermsAggregation {
	a.subAggregations[name] = subAggregation
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *RareTermsAggregation) Meta(metaData map[string]interface{}) *RareTermsAggregation {
	a.meta = metaData
	return a
}

func (a *RareTermsAggregation) MaxDocCount(maxDocCount int) *RareTermsAggregation {
	a.maxDocCount = &maxDocCount
	return a
}

func (a *RareTermsAggregation) Precision(precision float64) *RareTermsAggregation {
	a.precision = &precision
	return a
}

func (a *RareTermsAggregation) Missing(missing interface{}) *RareTermsAggregation {
	a.missing = missing
	return a
}

func (a *RareTermsAggregation) Include(regexp string) *RareTermsAggregation {
	if a.includeExclude == nil {
		a.includeExclude = &TermsAggregationIncludeExclude{}
	}
	a.includeExclude.Include = regexp
	return a
}

func (a *RareTermsAggregation) IncludeValues(values ...interface{}) *RareTermsAggregation {
	if a.includeExclude == nil {
		a.includeExclude = &TermsAggregationIncludeExclude{}
	}
	a.includeExclude.IncludeValues = append(a.includeExclude.IncludeValues, values...)
	return a
}

func (a *RareTermsAggregation) Exclude(regexp string) *RareTermsAggregation {
	if a.includeExclude == nil {
		a.includeExclude = &TermsAggregationIncludeExclude{}
	}
	a.includeExclude.Exclude = regexp
	return a
}

func (a *RareTermsAggregation) ExcludeValues(values ...interface{}) *RareTermsAggregation {
	if a.includeExclude == nil {
		a.includeExclude = &TermsAggregationIncludeExclude{}
	}
	a.includeExclude.ExcludeValues = append(a.includeExclude.ExcludeValues, values...)
	return a
}

func (a *RareTermsAggregation) IncludeExclude(includeExclude *TermsAggregationIncludeExclude) *RareTermsAggregation {
	a.includeExclude = includeExclude
	return a
}

func (a *RareTermsAggregation) Source() (interface{}, error) {
	// Example:
	// {
	//     "aggregations" : {
	//         "genres" : {
	//             "rare_terms" : { "field" : "genre" }
	//         }
	//     }
	// }
	//
	// This method returns only the
	//   "rare_terms" : { "field" : "genre" }
	// part.

	source := make(map[string]interface{})
	opts := make(map[string]interface{})
	source["rare_terms"] = opts

	if a.field != "" {
		opts["field"] = a.field
	}
	if a.maxDocCount != nil {
		opts["max_doc_count"] = *a.maxDocCount
	}
	if a.precision != nil {
		opts["precision"] = *a.precision
	}
	if a.missing != nil {
		opts["missing"] = a.missing
	}

	// Include/Exclude
	if ie := a.includeExclude; ie != nil {
		if err := ie.MergeInto(opts); err != nil {
			return nil, err
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
