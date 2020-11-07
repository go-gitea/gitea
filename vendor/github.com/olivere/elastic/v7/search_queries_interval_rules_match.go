package elastic

var (
	_ IntervalQueryRule = (*IntervalQueryRuleMatch)(nil)
)

// IntervalQueryRuleMatch is an implementation of IntervalQueryRule.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.5/query-dsl-intervals-query.html#intervals-match
// for details.
type IntervalQueryRuleMatch struct {
	query    string
	maxGaps  *int
	ordered  *bool
	analyzer string
	useField string
	filter   *IntervalQueryFilter
}

// NewIntervalQueryRuleMatch initializes and returns a new instance
// of IntervalQueryRuleMatch.
func NewIntervalQueryRuleMatch(query string) *IntervalQueryRuleMatch {
	return &IntervalQueryRuleMatch{query: query}
}

// MaxGaps specifies the maximum number of positions between the matching
// terms. Terms further apart than this are considered matches. Defaults to -1.
func (r *IntervalQueryRuleMatch) MaxGaps(maxGaps int) *IntervalQueryRuleMatch {
	r.maxGaps = &maxGaps
	return r
}

// Ordered, if true, indicates that matching terms must appear in their specified
// order. Defaults to false.
func (r *IntervalQueryRuleMatch) Ordered(ordered bool) *IntervalQueryRuleMatch {
	r.ordered = &ordered
	return r
}

// Analyzer specifies the analyzer used to analyze terms in the query.
func (r *IntervalQueryRuleMatch) Analyzer(analyzer string) *IntervalQueryRuleMatch {
	r.analyzer = analyzer
	return r
}

// UseField, if specified, matches the intervals from this field rather than
// the top-level field.
func (r *IntervalQueryRuleMatch) UseField(useField string) *IntervalQueryRuleMatch {
	r.useField = useField
	return r
}

// Filter adds an additional interval filter.
func (r *IntervalQueryRuleMatch) Filter(filter *IntervalQueryFilter) *IntervalQueryRuleMatch {
	r.filter = filter
	return r
}

// Source returns JSON for the function score query.
func (r *IntervalQueryRuleMatch) Source() (interface{}, error) {
	source := make(map[string]interface{})

	source["query"] = r.query

	if r.ordered != nil {
		source["ordered"] = *r.ordered
	}
	if r.maxGaps != nil {
		source["max_gaps"] = *r.maxGaps
	}
	if r.analyzer != "" {
		source["analyzer"] = r.analyzer
	}
	if r.useField != "" {
		source["use_field"] = r.useField
	}
	if r.filter != nil {
		filterRuleSource, err := r.filter.Source()
		if err != nil {
			return nil, err
		}

		source["filter"] = filterRuleSource
	}

	return map[string]interface{}{
		"match": source,
	}, nil
}

// isIntervalQueryRule implements the marker interface.
func (r *IntervalQueryRuleMatch) isIntervalQueryRule() bool {
	return true
}
