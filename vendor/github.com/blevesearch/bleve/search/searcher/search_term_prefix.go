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

type TermPrefixSearcher struct {
	indexReader index.IndexReader
	prefix      string
	field       string
	explain     bool
	searcher    *DisjunctionSearcher
}

func NewTermPrefixSearcher(indexReader index.IndexReader, prefix string, field string, boost float64, explain bool) (*TermPrefixSearcher, error) {
	// find the terms with this prefix
	fieldDict, err := indexReader.FieldDictPrefix(field, []byte(prefix))
	if err != nil {
		return nil, err
	}

	// enumerate all the terms in the range
	qsearchers := make([]search.Searcher, 0, 25)
	qsearchersClose := func() {
		for _, searcher := range qsearchers {
			_ = searcher.Close()
		}
	}

	tfd, err := fieldDict.Next()
	for err == nil && tfd != nil {
		var qsearcher *TermSearcher
		qsearcher, err = NewTermSearcher(indexReader, string(tfd.Term), field, 1.0, explain)
		if err != nil {
			qsearchersClose()
			_ = fieldDict.Close()
			return nil, err
		}
		qsearchers = append(qsearchers, qsearcher)
		tfd, err = fieldDict.Next()
	}

	err = fieldDict.Close()
	if err != nil {
		qsearchersClose()
		return nil, err
	}

	// build disjunction searcher of these ranges
	searcher, err := NewDisjunctionSearcher(indexReader, qsearchers, 0, explain)
	if err != nil {
		qsearchersClose()
		return nil, err
	}

	return &TermPrefixSearcher{
		indexReader: indexReader,
		prefix:      prefix,
		field:       field,
		explain:     explain,
		searcher:    searcher,
	}, nil
}

func (s *TermPrefixSearcher) Count() uint64 {
	return s.searcher.Count()
}

func (s *TermPrefixSearcher) Weight() float64 {
	return s.searcher.Weight()
}

func (s *TermPrefixSearcher) SetQueryNorm(qnorm float64) {
	s.searcher.SetQueryNorm(qnorm)
}

func (s *TermPrefixSearcher) Next(ctx *search.SearchContext) (*search.DocumentMatch, error) {
	return s.searcher.Next(ctx)

}

func (s *TermPrefixSearcher) Advance(ctx *search.SearchContext, ID index.IndexInternalID) (*search.DocumentMatch, error) {
	return s.searcher.Advance(ctx, ID)
}

func (s *TermPrefixSearcher) Close() error {
	return s.searcher.Close()
}

func (s *TermPrefixSearcher) Min() int {
	return 0
}

func (s *TermPrefixSearcher) DocumentMatchPoolSize() int {
	return s.searcher.DocumentMatchPoolSize()
}
