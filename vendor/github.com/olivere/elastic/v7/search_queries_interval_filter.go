package elastic

var (
	_ IntervalQueryRule = (*IntervalQueryFilter)(nil)
)

// IntervalQueryFilter specifies filters used in some
// IntervalQueryRule implementations, e.g. IntervalQueryRuleAllOf.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.5/query-dsl-intervals-query.html#interval_filter
// for details.
type IntervalQueryFilter struct {
	after          IntervalQueryRule
	before         IntervalQueryRule
	containedBy    IntervalQueryRule
	containing     IntervalQueryRule
	overlapping    IntervalQueryRule
	notContainedBy IntervalQueryRule
	notContaining  IntervalQueryRule
	notOverlapping IntervalQueryRule
	script         *Script
}

// NewIntervalQueryFilter initializes and creates a new
// IntervalQueryFilter.
func NewIntervalQueryFilter() *IntervalQueryFilter {
	return &IntervalQueryFilter{}
}

// After specifies the query to be used to return intervals that follow
// an interval from the filter rule.
func (r *IntervalQueryFilter) After(after IntervalQueryRule) *IntervalQueryFilter {
	r.after = after
	return r
}

// Before specifies the query to be used to return intervals that occur
// before an interval from the filter rule.
func (r *IntervalQueryFilter) Before(before IntervalQueryRule) *IntervalQueryFilter {
	r.before = before
	return r
}

// ContainedBy specifies the query to be used to return intervals contained
// by an interval from the filter rule.
func (r *IntervalQueryFilter) ContainedBy(containedBy IntervalQueryRule) *IntervalQueryFilter {
	r.containedBy = containedBy
	return r
}

// Containing specifies the query to be used to return intervals that contain an
// interval from the filter rule.
func (r *IntervalQueryFilter) Containing(containing IntervalQueryRule) *IntervalQueryFilter {
	r.containing = containing
	return r
}

// Overlapping specifies the query to be used to return intervals that overlap
// with an interval from the filter rule.
func (r *IntervalQueryFilter) Overlapping(overlapping IntervalQueryRule) *IntervalQueryFilter {
	r.overlapping = overlapping
	return r
}

// NotContainedBy specifies the query to be used to return intervals that are NOT
// contained by an interval from the filter rule.
func (r *IntervalQueryFilter) NotContainedBy(notContainedBy IntervalQueryRule) *IntervalQueryFilter {
	r.notContainedBy = notContainedBy
	return r
}

// NotContaining specifies the query to be used to return intervals that do NOT
// contain an interval from the filter rule.
func (r *IntervalQueryFilter) NotContaining(notContaining IntervalQueryRule) *IntervalQueryFilter {
	r.notContaining = notContaining
	return r
}

// NotOverlapping specifies the query to be used to return intervals that do NOT
// overlap with an interval from the filter rule.
func (r *IntervalQueryFilter) NotOverlapping(notOverlapping IntervalQueryRule) *IntervalQueryFilter {
	r.notOverlapping = notOverlapping
	return r
}

// Script allows a script to be used to return matching documents. The script
// must return a boolean value, true or false.
func (r *IntervalQueryFilter) Script(script *Script) *IntervalQueryFilter {
	r.script = script
	return r
}

// Source returns JSON for the function score query.
func (r *IntervalQueryFilter) Source() (interface{}, error) {
	source := make(map[string]interface{})

	if r.before != nil {
		src, err := r.before.Source()
		if err != nil {
			return nil, err
		}
		source["before"] = src
	}

	if r.after != nil {
		src, err := r.after.Source()
		if err != nil {
			return nil, err
		}
		source["after"] = src
	}

	if r.containedBy != nil {
		src, err := r.containedBy.Source()
		if err != nil {
			return nil, err
		}
		source["contained_by"] = src
	}

	if r.containing != nil {
		src, err := r.containing.Source()
		if err != nil {
			return nil, err
		}
		source["containing"] = src
	}

	if r.overlapping != nil {
		src, err := r.overlapping.Source()
		if err != nil {
			return nil, err
		}
		source["overlapping"] = src
	}

	if r.notContainedBy != nil {
		src, err := r.notContainedBy.Source()
		if err != nil {
			return nil, err
		}
		source["not_contained_by"] = src
	}

	if r.notContaining != nil {
		src, err := r.notContaining.Source()
		if err != nil {
			return nil, err
		}
		source["not_containing"] = src
	}

	if r.notOverlapping != nil {
		src, err := r.notOverlapping.Source()
		if err != nil {
			return nil, err
		}
		source["not_overlapping"] = src
	}

	if r.script != nil {
		src, err := r.script.Source()
		if err != nil {
			return nil, err
		}
		source["script"] = src
	}

	return source, nil
}

// isIntervalQueryRule implements the marker interface.
func (r *IntervalQueryFilter) isIntervalQueryRule() bool {
	return true
}
