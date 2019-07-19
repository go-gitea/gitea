// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import "errors"

// CompletionSuggester is a fast suggester for e.g. type-ahead completion.
// See https://www.elastic.co/guide/en/elasticsearch/reference/5.2/search-suggesters-completion.html
// for more details.
type CompletionSuggester struct {
	Suggester
	name           string
	text           string
	prefix         string
	regex          string
	field          string
	analyzer       string
	size           *int
	shardSize      *int
	contextQueries []SuggesterContextQuery
	payload        interface{}

	fuzzyOptions *FuzzyCompletionSuggesterOptions
	regexOptions *RegexCompletionSuggesterOptions
}

// Creates a new completion suggester.
func NewCompletionSuggester(name string) *CompletionSuggester {
	return &CompletionSuggester{
		name: name,
	}
}

func (q *CompletionSuggester) Name() string {
	return q.name
}

func (q *CompletionSuggester) Text(text string) *CompletionSuggester {
	q.text = text
	return q
}

func (q *CompletionSuggester) Prefix(prefix string) *CompletionSuggester {
	q.prefix = prefix
	return q
}

func (q *CompletionSuggester) PrefixWithEditDistance(prefix string, editDistance interface{}) *CompletionSuggester {
	q.prefix = prefix
	q.fuzzyOptions = NewFuzzyCompletionSuggesterOptions().EditDistance(editDistance)
	return q
}

func (q *CompletionSuggester) PrefixWithOptions(prefix string, options *FuzzyCompletionSuggesterOptions) *CompletionSuggester {
	q.prefix = prefix
	q.fuzzyOptions = options
	return q
}

func (q *CompletionSuggester) FuzzyOptions(options *FuzzyCompletionSuggesterOptions) *CompletionSuggester {
	q.fuzzyOptions = options
	return q
}

func (q *CompletionSuggester) Regex(regex string) *CompletionSuggester {
	q.regex = regex
	return q
}

func (q *CompletionSuggester) RegexWithOptions(regex string, options *RegexCompletionSuggesterOptions) *CompletionSuggester {
	q.regex = regex
	q.regexOptions = options
	return q
}

func (q *CompletionSuggester) RegexOptions(options *RegexCompletionSuggesterOptions) *CompletionSuggester {
	q.regexOptions = options
	return q
}

func (q *CompletionSuggester) Field(field string) *CompletionSuggester {
	q.field = field
	return q
}

func (q *CompletionSuggester) Analyzer(analyzer string) *CompletionSuggester {
	q.analyzer = analyzer
	return q
}

func (q *CompletionSuggester) Size(size int) *CompletionSuggester {
	q.size = &size
	return q
}

func (q *CompletionSuggester) ShardSize(shardSize int) *CompletionSuggester {
	q.shardSize = &shardSize
	return q
}

func (q *CompletionSuggester) ContextQuery(query SuggesterContextQuery) *CompletionSuggester {
	q.contextQueries = append(q.contextQueries, query)
	return q
}

func (q *CompletionSuggester) ContextQueries(queries ...SuggesterContextQuery) *CompletionSuggester {
	q.contextQueries = append(q.contextQueries, queries...)
	return q
}

// completionSuggesterRequest is necessary because the order in which
// the JSON elements are routed to Elasticsearch is relevant.
// We got into trouble when using plain maps because the text element
// needs to go before the completion element.
type completionSuggesterRequest struct {
	Text       string      `json:"text,omitempty"`
	Prefix     string      `json:"prefix,omitempty"`
	Regex      string      `json:"regex,omitempty"`
	Completion interface{} `json:"completion,omitempty"`
}

// Source creates the JSON data for the completion suggester.
func (q *CompletionSuggester) Source(includeName bool) (interface{}, error) {
	cs := &completionSuggesterRequest{}

	if q.text != "" {
		cs.Text = q.text
	}
	if q.prefix != "" {
		cs.Prefix = q.prefix
	}
	if q.regex != "" {
		cs.Regex = q.regex
	}

	suggester := make(map[string]interface{})
	cs.Completion = suggester

	if q.analyzer != "" {
		suggester["analyzer"] = q.analyzer
	}
	if q.field != "" {
		suggester["field"] = q.field
	}
	if q.size != nil {
		suggester["size"] = *q.size
	}
	if q.shardSize != nil {
		suggester["shard_size"] = *q.shardSize
	}
	switch len(q.contextQueries) {
	case 0:
	case 1:
		src, err := q.contextQueries[0].Source()
		if err != nil {
			return nil, err
		}
		suggester["context"] = src
	default:
		ctxq := make(map[string]interface{})
		for _, query := range q.contextQueries {
			src, err := query.Source()
			if err != nil {
				return nil, err
			}
			// Merge the dictionary into ctxq
			m, ok := src.(map[string]interface{})
			if !ok {
				return nil, errors.New("elastic: context query is not a map")
			}
			for k, v := range m {
				ctxq[k] = v
			}
		}
		suggester["contexts"] = ctxq
	}

	// Fuzzy options
	if q.fuzzyOptions != nil {
		src, err := q.fuzzyOptions.Source()
		if err != nil {
			return nil, err
		}
		suggester["fuzzy"] = src
	}

	// Regex options
	if q.regexOptions != nil {
		src, err := q.regexOptions.Source()
		if err != nil {
			return nil, err
		}
		suggester["regex"] = src
	}

	// TODO(oe) Add completion-suggester specific parameters here

	if !includeName {
		return cs, nil
	}

	source := make(map[string]interface{})
	source[q.name] = cs
	return source, nil
}

// -- Fuzzy options --

// FuzzyCompletionSuggesterOptions represents the options for fuzzy completion suggester.
type FuzzyCompletionSuggesterOptions struct {
	editDistance          interface{}
	transpositions        *bool
	minLength             *int
	prefixLength          *int
	unicodeAware          *bool
	maxDeterminizedStates *int
}

// NewFuzzyCompletionSuggesterOptions initializes a new FuzzyCompletionSuggesterOptions instance.
func NewFuzzyCompletionSuggesterOptions() *FuzzyCompletionSuggesterOptions {
	return &FuzzyCompletionSuggesterOptions{}
}

// EditDistance specifies the maximum number of edits, e.g. a number like "1" or "2"
// or a string like "0..2" or ">5". See https://www.elastic.co/guide/en/elasticsearch/reference/5.6/common-options.html#fuzziness
// for details.
func (o *FuzzyCompletionSuggesterOptions) EditDistance(editDistance interface{}) *FuzzyCompletionSuggesterOptions {
	o.editDistance = editDistance
	return o
}

// Transpositions, if set to true, are counted as one change instead of two (defaults to true).
func (o *FuzzyCompletionSuggesterOptions) Transpositions(transpositions bool) *FuzzyCompletionSuggesterOptions {
	o.transpositions = &transpositions
	return o
}

// MinLength represents the minimum length of the input before fuzzy suggestions are returned (defaults to 3).
func (o *FuzzyCompletionSuggesterOptions) MinLength(minLength int) *FuzzyCompletionSuggesterOptions {
	o.minLength = &minLength
	return o
}

// PrefixLength represents the minimum length of the input, which is not checked for
// fuzzy alternatives (defaults to 1).
func (o *FuzzyCompletionSuggesterOptions) PrefixLength(prefixLength int) *FuzzyCompletionSuggesterOptions {
	o.prefixLength = &prefixLength
	return o
}

// UnicodeAware, if true, all measurements (like fuzzy edit distance, transpositions, and lengths)
// are measured in Unicode code points instead of in bytes. This is slightly slower than
// raw bytes, so it is set to false by default.
func (o *FuzzyCompletionSuggesterOptions) UnicodeAware(unicodeAware bool) *FuzzyCompletionSuggesterOptions {
	o.unicodeAware = &unicodeAware
	return o
}

// MaxDeterminizedStates is currently undocumented in Elasticsearch. It represents
// the maximum automaton states allowed for fuzzy expansion.
func (o *FuzzyCompletionSuggesterOptions) MaxDeterminizedStates(max int) *FuzzyCompletionSuggesterOptions {
	o.maxDeterminizedStates = &max
	return o
}

// Source creates the JSON data.
func (o *FuzzyCompletionSuggesterOptions) Source() (interface{}, error) {
	out := make(map[string]interface{})

	if o.editDistance != nil {
		out["fuzziness"] = o.editDistance
	}
	if o.transpositions != nil {
		out["transpositions"] = *o.transpositions
	}
	if o.minLength != nil {
		out["min_length"] = *o.minLength
	}
	if o.prefixLength != nil {
		out["prefix_length"] = *o.prefixLength
	}
	if o.unicodeAware != nil {
		out["unicode_aware"] = *o.unicodeAware
	}
	if o.maxDeterminizedStates != nil {
		out["max_determinized_states"] = *o.maxDeterminizedStates
	}

	return out, nil
}

// -- Regex options --

// RegexCompletionSuggesterOptions represents the options for regex completion suggester.
type RegexCompletionSuggesterOptions struct {
	flags                 interface{} // string or int
	maxDeterminizedStates *int
}

// NewRegexCompletionSuggesterOptions initializes a new RegexCompletionSuggesterOptions instance.
func NewRegexCompletionSuggesterOptions() *RegexCompletionSuggesterOptions {
	return &RegexCompletionSuggesterOptions{}
}

// Flags represents internal regex flags. See https://www.elastic.co/guide/en/elasticsearch/reference/5.6/search-suggesters-completion.html#regex
// for details.
func (o *RegexCompletionSuggesterOptions) Flags(flags interface{}) *RegexCompletionSuggesterOptions {
	o.flags = flags
	return o
}

// MaxDeterminizedStates represents the maximum automaton states allowed for regex expansion.
// See https://www.elastic.co/guide/en/elasticsearch/reference/5.6/search-suggesters-completion.html#regex
// for details.
func (o *RegexCompletionSuggesterOptions) MaxDeterminizedStates(max int) *RegexCompletionSuggesterOptions {
	o.maxDeterminizedStates = &max
	return o
}

// Source creates the JSON data.
func (o *RegexCompletionSuggesterOptions) Source() (interface{}, error) {
	out := make(map[string]interface{})

	if o.flags != nil {
		out["flags"] = o.flags
	}
	if o.maxDeterminizedStates != nil {
		out["max_determinized_states"] = *o.maxDeterminizedStates
	}

	return out, nil
}
