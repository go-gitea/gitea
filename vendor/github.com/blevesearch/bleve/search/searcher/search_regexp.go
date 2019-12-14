//  Copyright (c) 2015 Couchbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package searcher

import (
	"regexp"

	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/search"
)

// NewRegexpStringSearcher is similar to NewRegexpSearcher, but
// additionally optimizes for index readers that handle regexp's.
func NewRegexpStringSearcher(indexReader index.IndexReader, pattern string,
	field string, boost float64, options search.SearcherOptions) (
	search.Searcher, error) {
	ir, ok := indexReader.(index.IndexReaderRegexp)
	if !ok {
		r, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}

		return NewRegexpSearcher(indexReader, r, field, boost, options)
	}

	fieldDict, err := ir.FieldDictRegexp(field, pattern)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := fieldDict.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	var candidateTerms []string

	tfd, err := fieldDict.Next()
	for err == nil && tfd != nil {
		candidateTerms = append(candidateTerms, tfd.Term)
		tfd, err = fieldDict.Next()
	}
	if err != nil {
		return nil, err
	}

	return NewMultiTermSearcher(indexReader, candidateTerms, field, boost,
		options, true)
}

// NewRegexpSearcher creates a searcher which will match documents that
// contain terms which match the pattern regexp.  The match must be EXACT
// matching the entire term.  The provided regexp SHOULD NOT start with ^
// or end with $ as this can intefere with the implementation.  Separately,
// matches will be checked to ensure they match the entire term.
func NewRegexpSearcher(indexReader index.IndexReader, pattern index.Regexp,
	field string, boost float64, options search.SearcherOptions) (
	search.Searcher, error) {
	var candidateTerms []string

	prefixTerm, complete := pattern.LiteralPrefix()
	if complete {
		// there is no pattern
		candidateTerms = []string{prefixTerm}
	} else {
		var err error
		candidateTerms, err = findRegexpCandidateTerms(indexReader, pattern, field,
			prefixTerm)
		if err != nil {
			return nil, err
		}
	}

	return NewMultiTermSearcher(indexReader, candidateTerms, field, boost,
		options, true)
}

func findRegexpCandidateTerms(indexReader index.IndexReader,
	pattern index.Regexp, field, prefixTerm string) (rv []string, err error) {
	rv = make([]string, 0)
	var fieldDict index.FieldDict
	if len(prefixTerm) > 0 {
		fieldDict, err = indexReader.FieldDictPrefix(field, []byte(prefixTerm))
	} else {
		fieldDict, err = indexReader.FieldDict(field)
	}
	defer func() {
		if cerr := fieldDict.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	// enumerate the terms and check against regexp
	tfd, err := fieldDict.Next()
	for err == nil && tfd != nil {
		matchPos := pattern.FindStringIndex(tfd.Term)
		if matchPos != nil && matchPos[0] == 0 && matchPos[1] == len(tfd.Term) {
			rv = append(rv, tfd.Term)
			if tooManyClauses(len(rv)) {
				return rv, tooManyClausesErr(len(rv))
			}
		}
		tfd, err = fieldDict.Next()
	}

	return rv, err
}
