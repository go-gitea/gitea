// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"fmt"
	"strings"
)

// SimpleQueryStringQuery is a query that uses the SimpleQueryParser
// to parse its context. Unlike the regular query_string query,
// the simple_query_string query will never throw an exception,
// and discards invalid parts of the query.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/query-dsl-simple-query-string-query.html
type SimpleQueryStringQuery struct {
	queryText                       string
	analyzer                        string
	quoteFieldSuffix                string
	defaultOperator                 string
	fields                          []string
	fieldBoosts                     map[string]*float64
	minimumShouldMatch              string
	flags                           string
	boost                           *float64
	lowercaseExpandedTerms          *bool // deprecated
	lenient                         *bool
	analyzeWildcard                 *bool
	locale                          string // deprecated
	queryName                       string
	autoGenerateSynonymsPhraseQuery *bool
	fuzzyPrefixLength               int
	fuzzyMaxExpansions              int
	fuzzyTranspositions             *bool
}

// NewSimpleQueryStringQuery creates and initializes a new SimpleQueryStringQuery.
func NewSimpleQueryStringQuery(text string) *SimpleQueryStringQuery {
	return &SimpleQueryStringQuery{
		queryText:          text,
		fields:             make([]string, 0),
		fieldBoosts:        make(map[string]*float64),
		fuzzyPrefixLength:  -1,
		fuzzyMaxExpansions: -1,
	}
}

// Field adds a field to run the query against.
func (q *SimpleQueryStringQuery) Field(field string) *SimpleQueryStringQuery {
	q.fields = append(q.fields, field)
	return q
}

// Field adds a field to run the query against with a specific boost.
func (q *SimpleQueryStringQuery) FieldWithBoost(field string, boost float64) *SimpleQueryStringQuery {
	q.fields = append(q.fields, field)
	q.fieldBoosts[field] = &boost
	return q
}

// Boost sets the boost for this query.
func (q *SimpleQueryStringQuery) Boost(boost float64) *SimpleQueryStringQuery {
	q.boost = &boost
	return q
}

// QuoteFieldSuffix is an optional field name suffix to automatically
// try and add to the field searched when using quoted text.
func (q *SimpleQueryStringQuery) QuoteFieldSuffix(quoteFieldSuffix string) *SimpleQueryStringQuery {
	q.quoteFieldSuffix = quoteFieldSuffix
	return q
}

// QueryName sets the query name for the filter that can be used when
// searching for matched_filters per hit.
func (q *SimpleQueryStringQuery) QueryName(queryName string) *SimpleQueryStringQuery {
	q.queryName = queryName
	return q
}

// Analyzer specifies the analyzer to use for the query.
func (q *SimpleQueryStringQuery) Analyzer(analyzer string) *SimpleQueryStringQuery {
	q.analyzer = analyzer
	return q
}

// DefaultOperator specifies the default operator for the query.
func (q *SimpleQueryStringQuery) DefaultOperator(defaultOperator string) *SimpleQueryStringQuery {
	q.defaultOperator = defaultOperator
	return q
}

// Flags sets the flags for the query.
func (q *SimpleQueryStringQuery) Flags(flags string) *SimpleQueryStringQuery {
	q.flags = flags
	return q
}

// LowercaseExpandedTerms indicates whether terms of wildcard, prefix, fuzzy
// and range queries are automatically lower-cased or not. Default is true.
//
// Deprecated: Decision is now made by the analyzer.
func (q *SimpleQueryStringQuery) LowercaseExpandedTerms(lowercaseExpandedTerms bool) *SimpleQueryStringQuery {
	q.lowercaseExpandedTerms = &lowercaseExpandedTerms
	return q
}

// Locale to be used in the query.
//
// Deprecated: Decision is now made by the analyzer.
func (q *SimpleQueryStringQuery) Locale(locale string) *SimpleQueryStringQuery {
	q.locale = locale
	return q
}

// Lenient indicates whether the query string parser should be lenient
// when parsing field values. It defaults to the index setting and if not
// set, defaults to false.
func (q *SimpleQueryStringQuery) Lenient(lenient bool) *SimpleQueryStringQuery {
	q.lenient = &lenient
	return q
}

// AnalyzeWildcard indicates whether to enabled analysis on wildcard and prefix queries.
func (q *SimpleQueryStringQuery) AnalyzeWildcard(analyzeWildcard bool) *SimpleQueryStringQuery {
	q.analyzeWildcard = &analyzeWildcard
	return q
}

// MinimumShouldMatch specifies the minimumShouldMatch to apply to the
// resulting query should that be a Boolean query.
func (q *SimpleQueryStringQuery) MinimumShouldMatch(minimumShouldMatch string) *SimpleQueryStringQuery {
	q.minimumShouldMatch = minimumShouldMatch
	return q
}

// AutoGenerateSynonymsPhraseQuery indicates whether phrase queries should be
// automatically generated for multi terms synonyms. Defaults to true.
func (q *SimpleQueryStringQuery) AutoGenerateSynonymsPhraseQuery(enable bool) *SimpleQueryStringQuery {
	q.autoGenerateSynonymsPhraseQuery = &enable
	return q
}

// FuzzyPrefixLength defines the prefix length in fuzzy queries.
func (q *SimpleQueryStringQuery) FuzzyPrefixLength(fuzzyPrefixLength int) *SimpleQueryStringQuery {
	q.fuzzyPrefixLength = fuzzyPrefixLength
	return q
}

// FuzzyMaxExpansions defines the number of terms fuzzy queries will expand to.
func (q *SimpleQueryStringQuery) FuzzyMaxExpansions(fuzzyMaxExpansions int) *SimpleQueryStringQuery {
	q.fuzzyMaxExpansions = fuzzyMaxExpansions
	return q
}

// FuzzyTranspositions defines whether to use transpositions in fuzzy queries.
func (q *SimpleQueryStringQuery) FuzzyTranspositions(fuzzyTranspositions bool) *SimpleQueryStringQuery {
	q.fuzzyTranspositions = &fuzzyTranspositions
	return q
}

// Source returns JSON for the query.
func (q *SimpleQueryStringQuery) Source() (interface{}, error) {
	// {
	//    "simple_query_string" : {
	//      "query" : "\"fried eggs\" +(eggplant | potato) -frittata",
	//			"analyzer" : "snowball",
	//      "fields" : ["body^5","_all"],
	//      "default_operator" : "and"
	//    }
	// }

	source := make(map[string]interface{})

	query := make(map[string]interface{})
	source["simple_query_string"] = query

	query["query"] = q.queryText

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

	if q.flags != "" {
		query["flags"] = q.flags
	}
	if q.analyzer != "" {
		query["analyzer"] = q.analyzer
	}
	if q.defaultOperator != "" {
		query["default_operator"] = strings.ToLower(q.defaultOperator)
	}
	if q.lowercaseExpandedTerms != nil {
		query["lowercase_expanded_terms"] = *q.lowercaseExpandedTerms
	}
	if q.lenient != nil {
		query["lenient"] = *q.lenient
	}
	if q.analyzeWildcard != nil {
		query["analyze_wildcard"] = *q.analyzeWildcard
	}
	if q.locale != "" {
		query["locale"] = q.locale
	}
	if q.queryName != "" {
		query["_name"] = q.queryName
	}
	if q.minimumShouldMatch != "" {
		query["minimum_should_match"] = q.minimumShouldMatch
	}
	if q.quoteFieldSuffix != "" {
		query["quote_field_suffix"] = q.quoteFieldSuffix
	}
	if q.boost != nil {
		query["boost"] = *q.boost
	}
	if v := q.autoGenerateSynonymsPhraseQuery; v != nil {
		query["auto_generate_synonyms_phrase_query"] = *v
	}
	if v := q.fuzzyPrefixLength; v != -1 {
		query["fuzzy_prefix_length"] = v
	}
	if v := q.fuzzyMaxExpansions; v != -1 {
		query["fuzzy_max_expansions"] = v
	}
	if v := q.fuzzyTranspositions; v != nil {
		query["fuzzy_transpositions"] = *v
	}

	return source, nil
}
