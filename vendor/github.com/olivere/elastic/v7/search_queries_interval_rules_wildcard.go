package elastic

var (
	_ IntervalQueryRule = (*IntervalQueryRuleWildcard)(nil)
)

// IntervalQueryRuleWildcard is an implementation of IntervalQueryRule.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.5/query-dsl-intervals-query.html#intervals-wildcard
// for details.
type IntervalQueryRuleWildcard struct {
	pattern  string
	analyzer string
	useField string
}

// NewIntervalQueryRuleWildcard initializes and returns a new instance
// of IntervalQueryRuleWildcard.
func NewIntervalQueryRuleWildcard(pattern string) *IntervalQueryRuleWildcard {
	return &IntervalQueryRuleWildcard{pattern: pattern}
}

// Analyzer specifies the analyzer used to analyze terms in the query.
func (r *IntervalQueryRuleWildcard) Analyzer(analyzer string) *IntervalQueryRuleWildcard {
	r.analyzer = analyzer
	return r
}

// UseField, if specified, matches the intervals from this field rather than
// the top-level field.
func (r *IntervalQueryRuleWildcard) UseField(useField string) *IntervalQueryRuleWildcard {
	r.useField = useField
	return r
}

// Source returns JSON for the function score query.
func (r *IntervalQueryRuleWildcard) Source() (interface{}, error) {
	source := make(map[string]interface{})

	source["pattern"] = r.pattern

	if r.analyzer != "" {
		source["analyzer"] = r.analyzer
	}
	if r.useField != "" {
		source["use_field"] = r.useField
	}

	return map[string]interface{}{
		"wildcard": source,
	}, nil
}

// isIntervalQueryRule implements the marker interface.
func (r *IntervalQueryRuleWildcard) isIntervalQueryRule() bool {
	return true
}
