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
	"reflect"

	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/search"
	"github.com/blevesearch/bleve/search/scorer"
	"github.com/blevesearch/bleve/size"
)

var reflectStaticSizeTermSearcher int

func init() {
	var ts TermSearcher
	reflectStaticSizeTermSearcher = int(reflect.TypeOf(ts).Size())
}

type TermSearcher struct {
	indexReader index.IndexReader
	reader      index.TermFieldReader
	scorer      *scorer.TermQueryScorer
	tfd         index.TermFieldDoc
}

func NewTermSearcher(indexReader index.IndexReader, term string, field string, boost float64, options search.SearcherOptions) (*TermSearcher, error) {
	return NewTermSearcherBytes(indexReader, []byte(term), field, boost, options)
}

func NewTermSearcherBytes(indexReader index.IndexReader, term []byte, field string, boost float64, options search.SearcherOptions) (*TermSearcher, error) {
	needFreqNorm := options.Score != "none"
	reader, err := indexReader.TermFieldReader(term, field, needFreqNorm, needFreqNorm, options.IncludeTermVectors)
	if err != nil {
		return nil, err
	}
	return newTermSearcherFromReader(indexReader, reader, term, field, boost, options)
}

func newTermSearcherFromReader(indexReader index.IndexReader, reader index.TermFieldReader,
	term []byte, field string, boost float64, options search.SearcherOptions) (*TermSearcher, error) {
	count, err := indexReader.DocCount()
	if err != nil {
		_ = reader.Close()
		return nil, err
	}
	scorer := scorer.NewTermQueryScorer(term, field, boost, count, reader.Count(), options)
	return &TermSearcher{
		indexReader: indexReader,
		reader:      reader,
		scorer:      scorer,
	}, nil
}

func (s *TermSearcher) Size() int {
	return reflectStaticSizeTermSearcher + size.SizeOfPtr +
		s.reader.Size() +
		s.tfd.Size() +
		s.scorer.Size()
}

func (s *TermSearcher) Count() uint64 {
	return s.reader.Count()
}

func (s *TermSearcher) Weight() float64 {
	return s.scorer.Weight()
}

func (s *TermSearcher) SetQueryNorm(qnorm float64) {
	s.scorer.SetQueryNorm(qnorm)
}

func (s *TermSearcher) Next(ctx *search.SearchContext) (*search.DocumentMatch, error) {
	termMatch, err := s.reader.Next(s.tfd.Reset())
	if err != nil {
		return nil, err
	}

	if termMatch == nil {
		return nil, nil
	}

	// score match
	docMatch := s.scorer.Score(ctx, termMatch)
	// return doc match
	return docMatch, nil

}

func (s *TermSearcher) Advance(ctx *search.SearchContext, ID index.IndexInternalID) (*search.DocumentMatch, error) {
	termMatch, err := s.reader.Advance(ID, s.tfd.Reset())
	if err != nil {
		return nil, err
	}

	if termMatch == nil {
		return nil, nil
	}

	// score match
	docMatch := s.scorer.Score(ctx, termMatch)

	// return doc match
	return docMatch, nil
}

func (s *TermSearcher) Close() error {
	return s.reader.Close()
}

func (s *TermSearcher) Min() int {
	return 0
}

func (s *TermSearcher) DocumentMatchPoolSize() int {
	return 1
}

func (s *TermSearcher) Optimize(kind string, octx index.OptimizableContext) (
	index.OptimizableContext, error) {
	o, ok := s.reader.(index.Optimizable)
	if ok {
		return o.Optimize(kind, octx)
	}

	return octx, nil
}
