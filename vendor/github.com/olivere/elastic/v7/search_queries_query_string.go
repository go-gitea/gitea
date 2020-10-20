// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"fmt"
)

// QueryStringQuery uses the query parser in order to parse its content.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/query-dsl-query-string-query.html
type QueryStringQuery struct {
	queryString              string
	defaultField             string
	defaultOperator          string
	analyzer                 string
	quoteAnalyzer            string
	quoteFieldSuffix         string
	allowLeadingWildcard     *bool
	lowercaseExpandedTerms   *bool // Deprecated: Decision is now made by the analyzer
	enablePositionIncrements *bool
	analyzeWildcard          *bool
	locale                   string // Deprecated: Decision is now made by the analyzer
	boost                    *float64
	fuzziness                string
	fuzzyPrefixLength        *int
	fuzzyMaxExpansions       *int
	fuzzyRewrite             string
	phraseSlop               *int
	fields                   []string
	fieldBoosts              map[string]*float64
	tieBreaker               *float64
	rewrite                  string
	minimumShouldMatch       string
	lenient                  *bool
	queryName                string
	timeZone                 string
	maxDeterminizedStates    *int
	escape                   *bool
	typ                      string
}

// NewQueryStringQuery creates and initializes a new QueryStringQuery.
func NewQueryStringQuery(queryString string) *QueryStringQuery {
	return &QueryStringQuery{
		queryString: queryString,
		fields:      make([]string, 0),
		fieldBoosts: make(map[string]*float64),
	}
}

// DefaultField specifies the field to run against when no prefix field
// is specified. Only relevant when not explicitly adding fields the query
// string will run against.
func (q *QueryStringQuery) DefaultField(defaultField string) *QueryStringQuery {
	q.defaultField = defaultField
	return q
}

// Field adds a field to run the query string against.
func (q *QueryStringQuery) Field(field string) *QueryStringQuery {
	q.fields = append(q.fields, field)
	return q
}

// Type sets how multiple fields should be combined to build textual part queries,
// e.g. "best_fields".
func (q *QueryStringQuery) Type(typ string) *QueryStringQuery {
	q.typ = typ
	return q
}

// FieldWithBoost adds a field to run the query string against with a specific boost.
func (q *QueryStringQuery) FieldWithBoost(field string, boost float64) *QueryStringQuery {
	q.fields = append(q.fields, field)
	q.fieldBoosts[field] = &boost
	return q
}

// TieBreaker is used when more than one field is used with the query string,
// and combined queries are using dismax.
func (q *QueryStringQuery) TieBreaker(tieBreaker float64) *QueryStringQuery {
	q.tieBreaker = &tieBreaker
	return q
}

// DefaultOperator sets the boolean operator of the query parser used to
// parse the query string.
//
// In default mode (OR) terms without any modifiers
// are considered optional, e.g. "capital of Hungary" is equal to
// "capital OR of OR Hungary".
//
// In AND mode, terms are considered to be in conjunction. The above mentioned
// query is then parsed as "capital AND of AND Hungary".
func (q *QueryStringQuery) DefaultOperator(operator string) *QueryStringQuery {
	q.defaultOperator = operator
	return q
}

// Analyzer is an optional analyzer used to analyze the query string.
// Note, if a field has search analyzer defined for it, then it will be used
// automatically. Defaults to the smart search analyzer.
func (q *QueryStringQuery) Analyzer(analyzer string) *QueryStringQuery {
	q.analyzer = analyzer
	return q
}

// QuoteAnalyzer is an optional analyzer to be used to analyze the query string
// for phrase searches. Note, if a field has search analyzer defined for it,
// then it will be used automatically. Defaults to the smart search analyzer.
func (q *QueryStringQuery) QuoteAnalyzer(quoteAnalyzer string) *QueryStringQuery {
	q.quoteAnalyzer = quoteAnalyzer
	return q
}

// MaxDeterminizedState protects against too-difficult regular expression queries.
func (q *QueryStringQuery) MaxDeterminizedState(maxDeterminizedStates int) *QueryStringQuery {
	q.maxDeterminizedStates = &maxDeterminizedStates
	return q
}

// AllowLeadingWildcard specifies whether leading wildcards should be allowed
// or not (defaults to true).
func (q *QueryStringQuery) AllowLeadingWildcard(allowLeadingWildcard bool) *QueryStringQuery {
	q.allowLeadingWildcard = &allowLeadingWildcard
	return q
}

// LowercaseExpandedTerms indicates whether terms of wildcard, prefix, fuzzy
// and range queries are automatically lower-cased or not. Default is true.
//
// Deprecated: Decision is now made by the analyzer.
func (q *QueryStringQuery) LowercaseExpandedTerms(lowercaseExpandedTerms bool) *QueryStringQuery {
	q.lowercaseExpandedTerms = &lowercaseExpandedTerms
	return q
}

// EnablePositionIncrements indicates whether to enable position increments
// in result query. Defaults to true.
//
// When set, result phrase and multi-phrase queries will be aware of position
// increments. Useful when e.g. a StopFilter increases the position increment
// of the token that follows an omitted token.
func (q *QueryStringQuery) EnablePositionIncrements(enablePositionIncrements bool) *QueryStringQuery {
	q.enablePositionIncrements = &enablePositionIncrements
	return q
}

// Fuzziness sets the edit distance for fuzzy queries. Default is "AUTO".
func (q *QueryStringQuery) Fuzziness(fuzziness string) *QueryStringQuery {
	q.fuzziness = fuzziness
	return q
}

// FuzzyPrefixLength sets the minimum prefix length for fuzzy queries.
// Default is 1.
func (q *QueryStringQuery) FuzzyPrefixLength(fuzzyPrefixLength int) *QueryStringQuery {
	q.fuzzyPrefixLength = &fuzzyPrefixLength
	return q
}

func (q *QueryStringQuery) FuzzyMaxExpansions(fuzzyMaxExpansions int) *QueryStringQuery {
	q.fuzzyMaxExpansions = &fuzzyMaxExpansions
	return q
}

func (q *QueryStringQuery) FuzzyRewrite(fuzzyRewrite string) *QueryStringQuery {
	q.fuzzyRewrite = fuzzyRewrite
	return q
}

// PhraseSlop sets the default slop for phrases. If zero, then exact matches
// are required. Default value is zero.
func (q *QueryStringQuery) PhraseSlop(phraseSlop int) *QueryStringQuery {
	q.phraseSlop = &phraseSlop
	return q
}

// AnalyzeWildcard indicates whether to enabled analysis on wildcard and prefix queries.
func (q *QueryStringQuery) AnalyzeWildcard(analyzeWildcard bool) *QueryStringQuery {
	q.analyzeWildcard = &analyzeWildcard
	return q
}

func (q *QueryStringQuery) Rewrite(rewrite string) *QueryStringQuery {
	q.rewrite = rewrite
	return q
}

func (q *QueryStringQuery) MinimumShouldMatch(minimumShouldMatch string) *QueryStringQuery {
	q.minimumShouldMatch = minimumShouldMatch
	return q
}

// Boost sets the boost for this query.
func (q *QueryStringQuery) Boost(boost float64) *QueryStringQuery {
	q.boost = &boost
	return q
}

// QuoteFieldSuffix is an optional field name suffix to automatically
// try and add to the field searched when using quoted text.
func (q *QueryStringQuery) QuoteFieldSuffix(quoteFieldSuffix string) *QueryStringQuery {
	q.quoteFieldSuffix = quoteFieldSuffix
	return q
}

// Lenient indicates whether the query string parser should be lenient
// when parsing field values. It defaults to the index setting and if not
// set, defaults to false.
func (q *QueryStringQuery) Lenient(lenient bool) *QueryStringQuery {
	q.lenient = &lenient
	return q
}

// QueryName sets the query name for the filter that can be used when
// searching for matched_filters per hit.
func (q *QueryStringQuery) QueryName(queryName string) *QueryStringQuery {
	q.queryName = queryName
	return q
}

// Locale specifies the locale to be used for string conversions.
//
// Deprecated: Decision is now made by the analyzer.
func (q *QueryStringQuery) Locale(locale string) *QueryStringQuery {
	q.locale = locale
	return q
}

// TimeZone can be used to automatically adjust to/from fields using a
// timezone. Only used with date fields, of course.
func (q *QueryStringQuery) TimeZone(timeZone string) *QueryStringQuery {
	q.timeZone = timeZone
	return q
}

// Escape performs escaping of the query string.
func (q *QueryStringQuery) Escape(escape bool) *QueryStringQuery {
	q.escape = &escape
	return q
}

// Source returns JSON for the query.
func (q *QueryStringQuery) Source() (interface{}, error) {
	source := make(map[string]interface{})
	query := make(map[string]interface{})
	source["query_string"] = query

	query["query"] = q.queryString

	if q.defaultField != "" {
		query["default_field"] = q.defaultField
	}

	if len(q.fields) > 0 {
		var fields []string
		for _, field := range q.fields {
			if boost, found := q.fieldBoosts[field]; found {
				if boost != nil {
					fields = append(fields, fmt.Sprintf("%s^%f", field, *boost))
				} else {
					fields = append(fields, field)
				}
			} else {
				fields = append(fields, field)
			}
		}
		query["fields"] = fields
	}

	if q.tieBreaker != nil {
		query["tie_breaker"] = *q.tieBreaker
	}
	if q.defaultOperator != "" {
		query["default_operator"] = q.defaultOperator
	}
	if q.analyzer != "" {
		query["analyzer"] = q.analyzer
	}
	if q.quoteAnalyzer != "" {
		query["quote_analyzer"] = q.quoteAnalyzer
	}
	if q.maxDeterminizedStates != nil {
		query["max_determinized_states"] = *q.maxDeterminizedStates
	}
	if q.allowLeadingWildcard != nil {
		query["allow_leading_wildcard"] = *q.allowLeadingWildcard
	}
	if q.lowercaseExpandedTerms != nil {
		query["lowercase_expanded_terms"] = *q.lowercaseExpandedTerms
	}
	if q.enablePositionIncrements != nil {
		query["enable_position_increments"] = *q.enablePositionIncrements
	}
	if q.fuzziness != "" {
		query["fuzziness"] = q.fuzziness
	}
	if q.boost != nil {
		query["boost"] = *q.boost
	}
	if q.fuzzyPrefixLength != nil {
		query["fuzzy_prefix_length"] = *q.fuzzyPrefixLength
	}
	if q.fuzzyMaxExpansions != nil {
		query["fuzzy_max_expansions"] = *q.fuzzyMaxExpansions
	}
	if q.fuzzyRewrite != "" {
		query["fuzzy_rewrite"] = q.fuzzyRewrite
	}
	if q.phraseSlop != nil {
		query["phrase_slop"] = *q.phraseSlop
	}
	if q.analyzeWildcard != nil {
		query["analyze_wildcard"] = *q.analyzeWildcard
	}
	if q.rewrite != "" {
		query["rewrite"] = q.rewrite
	}
	if q.minimumShouldMatch != "" {
		query["minimum_should_match"] = q.minimumShouldMatch
	}
	if q.quoteFieldSuffix != "" {
		query["quote_field_suffix"] = q.quoteFieldSuffix
	}
	if q.lenient != nil {
		query["lenient"] = *q.lenient
	}
	if q.queryName != "" {
		query["_name"] = q.queryName
	}
	if q.locale != "" {
		query["locale"] = q.locale
	}
	if q.timeZone != "" {
		query["time_zone"] = q.timeZone
	}
	if q.escape != nil {
		query["escape"] = *q.escape
	}
	if q.typ != "" {
		query["type"] = q.typ
	}

	return source, nil
}
