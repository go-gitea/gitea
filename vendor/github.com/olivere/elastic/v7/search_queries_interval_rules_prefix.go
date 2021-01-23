package elastic

var (
	_ IntervalQueryRule = (*IntervalQueryRulePrefix)(nil)
)

// IntervalQueryRulePrefix is an implementation of IntervalQueryRule.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.5/query-dsl-intervals-query.html#intervals-prefix
// for details.
type IntervalQueryRulePrefix struct {
	prefix   string
	analyzer string
	useField string
}

// NewIntervalQueryRulePrefix initializes and returns a new instance
// of IntervalQueryRulePrefix.
func NewIntervalQueryRulePrefix(prefix string) *IntervalQueryRulePrefix {
	return &IntervalQueryRulePrefix{prefix: prefix}
}

// Analyzer specifies the analyzer used to analyze terms in the query.
func (r *IntervalQueryRulePrefix) Analyzer(analyzer string) *IntervalQueryRulePrefix {
	r.analyzer = analyzer
	return r
}

// UseField, if specified, matches the intervals from this field rather than
// the top-level field.
func (r *IntervalQueryRulePrefix) UseField(useField string) *IntervalQueryRulePrefix {
	r.useField = useField
	return r
}

// Source returns JSON for the function score query.
func (r *IntervalQueryRulePrefix) Source() (interface{}, error) {
	source := make(map[string]interface{})

	source["query"] = r.prefix

	if r.analyzer != "" {
		source["analyzer"] = r.analyzer
	}
	if r.useField != "" {
		source["use_field"] = r.useField
	}

	return map[string]interface{}{
		"prefix": source,
	}, nil
}

// isIntervalQueryRule implements the marker interface.
func (r *IntervalQueryRulePrefix) isIntervalQueryRule() bool {
	return true
}
