package elastic

var (
	_ IntervalQueryRule = (*IntervalQueryRuleAllOf)(nil)
)

// IntervalQueryRuleAllOf is an implementation of IntervalQueryRule.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.5/query-dsl-intervals-query.html#intervals-all_of
// for details.
type IntervalQueryRuleAllOf struct {
	intervals []IntervalQueryRule
	maxGaps   *int
	ordered   *bool
	filter    *IntervalQueryFilter
}

// NewIntervalQueryRuleAllOf initializes and returns a new instance
// of IntervalQueryRuleAllOf.
func NewIntervalQueryRuleAllOf(intervals ...IntervalQueryRule) *IntervalQueryRuleAllOf {
	return &IntervalQueryRuleAllOf{intervals: intervals}
}

// MaxGaps specifies the maximum number of positions between the matching
// terms. Terms further apart than this are considered matches. Defaults to -1.
func (r *IntervalQueryRuleAllOf) MaxGaps(maxGaps int) *IntervalQueryRuleAllOf {
	r.maxGaps = &maxGaps
	return r
}

// Ordered, if true, indicates that matching terms must appear in their specified
// order. Defaults to false.
func (r *IntervalQueryRuleAllOf) Ordered(ordered bool) *IntervalQueryRuleAllOf {
	r.ordered = &ordered
	return r
}

// Filter adds an additional interval filter.
func (r *IntervalQueryRuleAllOf) Filter(filter *IntervalQueryFilter) *IntervalQueryRuleAllOf {
	r.filter = filter
	return r
}

// Source returns JSON for the function score query.
func (r *IntervalQueryRuleAllOf) Source() (interface{}, error) {
	source := make(map[string]interface{})

	intervalSources := make([]interface{}, 0)
	for _, interval := range r.intervals {
		src, err := interval.Source()
		if err != nil {
			return nil, err
		}

		intervalSources = append(intervalSources, src)
	}
	source["intervals"] = intervalSources

	if r.ordered != nil {
		source["ordered"] = *r.ordered
	}
	if r.maxGaps != nil {
		source["max_gaps"] = *r.maxGaps
	}
	if r.filter != nil {
		src, err := r.filter.Source()
		if err != nil {
			return nil, err
		}

		source["filter"] = src
	}

	return map[string]interface{}{
		"all_of": source,
	}, nil
}

// isIntervalQueryRule implements the marker interface.
func (r *IntervalQueryRuleAllOf) isIntervalQueryRule() bool {
	return true
}
