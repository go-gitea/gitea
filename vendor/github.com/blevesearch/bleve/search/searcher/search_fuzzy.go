//  Copyright (c) 2014 Couchbase, Inc.
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
	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/search"
)

func NewFuzzySearcher(indexReader index.IndexReader, term string,
	prefix, fuzziness int, field string, boost float64,
	options search.SearcherOptions) (search.Searcher, error) {
	// Note: we don't byte slice the term for a prefix because of runes.
	prefixTerm := ""
	for i, r := range term {
		if i < prefix {
			prefixTerm += string(r)
		} else {
			break
		}
	}

	candidateTerms, err := findFuzzyCandidateTerms(indexReader, term, fuzziness,
		field, prefixTerm)
	if err != nil {
		return nil, err
	}

	return NewMultiTermSearcher(indexReader, candidateTerms, field,
		boost, options, true)
}

func findFuzzyCandidateTerms(indexReader index.IndexReader, term string,
	fuzziness int, field, prefixTerm string) (rv []string, err error) {
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

	// enumerate terms and check levenshtein distance
	tfd, err := fieldDict.Next()
	for err == nil && tfd != nil {
		ld, exceeded := search.LevenshteinDistanceMax(term, tfd.Term, fuzziness)
		if !exceeded && ld <= fuzziness {
			rv = append(rv, tfd.Term)
			if tooManyClauses(len(rv)) {
				return rv, tooManyClausesErr()
			}
		}
		tfd, err = fieldDict.Next()
	}

	return rv, err
}
