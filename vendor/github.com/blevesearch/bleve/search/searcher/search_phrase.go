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
	"math"

	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/search"
)

type PhraseSearcher struct {
	indexReader  index.IndexReader
	mustSearcher *ConjunctionSearcher
	queryNorm    float64
	currMust     *search.DocumentMatch
	slop         int
	terms        []string
	initialized  bool
}

func NewPhraseSearcher(indexReader index.IndexReader, mustSearcher *ConjunctionSearcher, terms []string) (*PhraseSearcher, error) {
	// build our searcher
	rv := PhraseSearcher{
		indexReader:  indexReader,
		mustSearcher: mustSearcher,
		terms:        terms,
	}
	rv.computeQueryNorm()
	return &rv, nil
}

func (s *PhraseSearcher) computeQueryNorm() {
	// first calculate sum of squared weights
	sumOfSquaredWeights := 0.0
	if s.mustSearcher != nil {
		sumOfSquaredWeights += s.mustSearcher.Weight()
	}

	// now compute query norm from this
	s.queryNorm = 1.0 / math.Sqrt(sumOfSquaredWeights)
	// finally tell all the downstream searchers the norm
	if s.mustSearcher != nil {
		s.mustSearcher.SetQueryNorm(s.queryNorm)
	}
}

func (s *PhraseSearcher) initSearchers(ctx *search.SearchContext) error {
	err := s.advanceNextMust(ctx)
	if err != nil {
		return err
	}

	s.initialized = true
	return nil
}

func (s *PhraseSearcher) advanceNextMust(ctx *search.SearchContext) error {
	var err error

	if s.mustSearcher != nil {
		s.currMust, err = s.mustSearcher.Next(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *PhraseSearcher) Weight() float64 {
	return s.mustSearcher.Weight()
}

func (s *PhraseSearcher) SetQueryNorm(qnorm float64) {
	s.mustSearcher.SetQueryNorm(qnorm)
}

func (s *PhraseSearcher) Next(ctx *search.SearchContext) (*search.DocumentMatch, error) {
	if !s.initialized {
		err := s.initSearchers(ctx)
		if err != nil {
			return nil, err
		}
	}

	var rv *search.DocumentMatch
	for s.currMust != nil {
		rvftlm := make(search.FieldTermLocationMap, 0)
		freq := 0
		firstTerm := s.terms[0]
		for field, termLocMap := range s.currMust.Locations {
			rvtlm := make(search.TermLocationMap, 0)
			locations, ok := termLocMap[firstTerm]
			if ok {
			OUTER:
				for _, location := range locations {
					crvtlm := make(search.TermLocationMap, 0)
				INNER:
					for i := 0; i < len(s.terms); i++ {
						nextTerm := s.terms[i]
						if nextTerm != "" {
							// look through all these term locations
							// to try and find the correct offsets
							nextLocations, ok := termLocMap[nextTerm]
							if ok {
								for _, nextLocation := range nextLocations {
									if nextLocation.Pos == location.Pos+float64(i) && nextLocation.SameArrayElement(location) {
										// found a location match for this term
										crvtlm.AddLocation(nextTerm, nextLocation)
										continue INNER
									}
								}
								// if we got here we didn't find a location match for this term
								continue OUTER
							} else {
								continue OUTER
							}
						}
					}
					// if we got here all the terms matched
					freq++
					search.MergeTermLocationMaps(rvtlm, crvtlm)
					rvftlm[field] = rvtlm
				}
			}
		}

		if freq > 0 {
			// return match
			rv = s.currMust
			rv.Locations = rvftlm
			err := s.advanceNextMust(ctx)
			if err != nil {
				return nil, err
			}
			return rv, nil
		}

		err := s.advanceNextMust(ctx)
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (s *PhraseSearcher) Advance(ctx *search.SearchContext, ID index.IndexInternalID) (*search.DocumentMatch, error) {
	if !s.initialized {
		err := s.initSearchers(ctx)
		if err != nil {
			return nil, err
		}
	}
	var err error
	s.currMust, err = s.mustSearcher.Advance(ctx, ID)
	if err != nil {
		return nil, err
	}
	return s.Next(ctx)
}

func (s *PhraseSearcher) Count() uint64 {
	// for now return a worst case
	return s.mustSearcher.Count()
}

func (s *PhraseSearcher) Close() error {
	if s.mustSearcher != nil {
		err := s.mustSearcher.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *PhraseSearcher) Min() int {
	return 0
}

func (s *PhraseSearcher) DocumentMatchPoolSize() int {
	return s.mustSearcher.DocumentMatchPoolSize() + 1
}
