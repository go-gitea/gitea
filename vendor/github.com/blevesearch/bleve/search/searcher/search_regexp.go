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

func NewRegexpSearcher(indexReader index.IndexReader, pattern *regexp.Regexp, field string, boost float64, explain bool) (search.Searcher, error) {

	prefixTerm, complete := pattern.LiteralPrefix()
	var candidateTerms []string
	if complete {
		// there is no pattern
		candidateTerms = []string{prefixTerm}
	} else {
		var err error
		candidateTerms, err = findRegexpCandidateTerms(indexReader, pattern, field, prefixTerm)
		if err != nil {
			return nil, err
		}
	}

	// enumerate all the terms in the range
	qsearchers := make([]search.Searcher, 0, len(candidateTerms))
	qsearchersClose := func() {
		for _, searcher := range qsearchers {
			_ = searcher.Close()
		}
	}
	for _, cterm := range candidateTerms {
		qsearcher, err := NewTermSearcher(indexReader, cterm, field, boost, explain)
		if err != nil {
			qsearchersClose()
			return nil, err
		}
		qsearchers = append(qsearchers, qsearcher)
	}

	// build disjunction searcher of these ranges
	searcher, err := NewDisjunctionSearcher(indexReader, qsearchers, 0, explain)
	if err != nil {
		qsearchersClose()
		return nil, err
	}

	return searcher, err
}

func findRegexpCandidateTerms(indexReader index.IndexReader, pattern *regexp.Regexp, field, prefixTerm string) (rv []string, err error) {
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
		if pattern.MatchString(tfd.Term) {
			rv = append(rv, tfd.Term)
			if tooManyClauses(len(rv)) {
				return rv, tooManyClausesErr()
			}
		}
		tfd, err = fieldDict.Next()
	}

	return rv, err
}
