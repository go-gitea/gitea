package elastic

var (
	_ IntervalQueryRule = (*IntervalQueryRuleAnyOf)(nil)
)

// IntervalQueryRuleAnyOf is an implementation of IntervalQueryRule.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.5/query-dsl-intervals-query.html#intervals-any_of
// for details.
type IntervalQueryRuleAnyOf struct {
	intervals []IntervalQueryRule
	filter    *IntervalQueryFilter
}

// NewIntervalQueryRuleAnyOf initializes and returns a new instance
// of IntervalQueryRuleAnyOf.
func NewIntervalQueryRuleAnyOf(intervals ...IntervalQueryRule) *IntervalQueryRuleAnyOf {
	return &IntervalQueryRuleAnyOf{intervals: intervals}
}

// Filter adds an additional interval filter.
func (r *IntervalQueryRuleAnyOf) Filter(filter *IntervalQueryFilter) *IntervalQueryRuleAnyOf {
	r.filter = filter
	return r
}

// Source returns JSON for the function score query.
func (r *IntervalQueryRuleAnyOf) Source() (interface{}, error) {
	source := make(map[string]interface{})

	var intervalSources []interface{}
	for _, interval := range r.intervals {
		src, err := interval.Source()
		if err != nil {
			return nil, err
		}

		intervalSources = append(intervalSources, src)
	}
	source["intervals"] = intervalSources

	if r.filter != nil {
		src, err := r.filter.Source()
		if err != nil {
			return nil, err
		}

		source["filter"] = src
	}

	return map[string]interface{}{
		"any_of": source,
	}, nil
}

// isIntervalQueryRule implements the marker interface.
func (r *IntervalQueryRuleAnyOf) isIntervalQueryRule() bool {
	return true
}
