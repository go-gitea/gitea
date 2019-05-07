//  Copyright (c) 2017 Couchbase, Inc.
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

func NewMultiTermSearcher(indexReader index.IndexReader, terms []string,
	field string, boost float64, options search.SearcherOptions, limit bool) (
	search.Searcher, error) {
	if limit && tooManyClauses(len(terms)) {
		return nil, tooManyClausesErr(len(terms))
	}

	qsearchers := make([]search.Searcher, len(terms))
	qsearchersClose := func() {
		for _, searcher := range qsearchers {
			if searcher != nil {
				_ = searcher.Close()
			}
		}
	}
	for i, term := range terms {
		var err error
		qsearchers[i], err = NewTermSearcher(indexReader, term, field, boost, options)
		if err != nil {
			qsearchersClose()
			return nil, err
		}
	}
	// build disjunction searcher of these ranges
	return newMultiTermSearcherBytes(indexReader, qsearchers, field, boost,
		options, limit)
}

func NewMultiTermSearcherBytes(indexReader index.IndexReader, terms [][]byte,
	field string, boost float64, options search.SearcherOptions, limit bool) (
	search.Searcher, error) {
	if limit && tooManyClauses(len(terms)) {
		return nil, tooManyClausesErr(len(terms))
	}

	qsearchers := make([]search.Searcher, len(terms))
	qsearchersClose := func() {
		for _, searcher := range qsearchers {
			if searcher != nil {
				_ = searcher.Close()
			}
		}
	}
	for i, term := range terms {
		var err error
		qsearchers[i], err = NewTermSearcherBytes(indexReader, term, field, boost, options)
		if err != nil {
			qsearchersClose()
			return nil, err
		}
	}
	return newMultiTermSearcherBytes(indexReader, qsearchers, field, boost,
		options, limit)
}

func newMultiTermSearcherBytes(indexReader index.IndexReader,
	searchers []search.Searcher, field string, boost float64,
	options search.SearcherOptions, limit bool) (
	search.Searcher, error) {

	// build disjunction searcher of these ranges
	searcher, err := newDisjunctionSearcher(indexReader, searchers, 0, options,
		limit)
	if err != nil {
		for _, s := range searchers {
			_ = s.Close()
		}
		return nil, err
	}

	return searcher, nil
}
