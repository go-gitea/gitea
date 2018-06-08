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
	"fmt"
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
	terms        [][]string
	initialized  bool
}

func NewPhraseSearcher(indexReader index.IndexReader, terms []string, field string, options search.SearcherOptions) (*PhraseSearcher, error) {
	// turn flat terms []string into [][]string
	mterms := make([][]string, len(terms))
	for i, term := range terms {
		mterms[i] = []string{term}
	}
	return NewMultiPhraseSearcher(indexReader, mterms, field, options)
}

func NewMultiPhraseSearcher(indexReader index.IndexReader, terms [][]string, field string, options search.SearcherOptions) (*PhraseSearcher, error) {
	options.IncludeTermVectors = true
	var termPositionSearchers []search.Searcher
	for _, termPos := range terms {
		if len(termPos) == 1 && termPos[0] != "" {
			// single term
			ts, err := NewTermSearcher(indexReader, termPos[0], field, 1.0, options)
			if err != nil {
				// close any searchers already opened
				for _, ts := range termPositionSearchers {
					_ = ts.Close()
				}
				return nil, fmt.Errorf("phrase searcher error building term searcher: %v", err)
			}
			termPositionSearchers = append(termPositionSearchers, ts)
		} else if len(termPos) > 1 {
			// multiple terms
			var termSearchers []search.Searcher
			for _, term := range termPos {
				if term == "" {
					continue
				}
				ts, err := NewTermSearcher(indexReader, term, field, 1.0, options)
				if err != nil {
					// close any searchers already opened
					for _, ts := range termPositionSearchers {
						_ = ts.Close()
					}
					return nil, fmt.Errorf("phrase searcher error building term searcher: %v", err)
				}
				termSearchers = append(termSearchers, ts)
			}
			disjunction, err := NewDisjunctionSearcher(indexReader, termSearchers, 1, options)
			if err != nil {
				// close any searchers already opened
				for _, ts := range termPositionSearchers {
					_ = ts.Close()
				}
				return nil, fmt.Errorf("phrase searcher error building term position disjunction searcher: %v", err)
			}
			termPositionSearchers = append(termPositionSearchers, disjunction)
		}
	}

	mustSearcher, err := NewConjunctionSearcher(indexReader, termPositionSearchers, options)
	if err != nil {
		// close any searchers already opened
		for _, ts := range termPositionSearchers {
			_ = ts.Close()
		}
		return nil, fmt.Errorf("phrase searcher error building conjunction searcher: %v", err)
	}

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

	for s.currMust != nil {
		// check this match against phrase constraints
		rv := s.checkCurrMustMatch(ctx)

		// prepare for next iteration (either loop or subsequent call to Next())
		err := s.advanceNextMust(ctx)
		if err != nil {
			return nil, err
		}

		// if match satisfied phrase constraints return it as a hit
		if rv != nil {
			return rv, nil
		}
	}

	return nil, nil
}

// checkCurrMustMatch is soley concerned with determining if the DocumentMatch
// pointed to by s.currMust (which satisifies the pre-condition searcher)
// also satisfies the phase constraints.  if so, it returns a DocumentMatch
// for this document, otherwise nil
func (s *PhraseSearcher) checkCurrMustMatch(ctx *search.SearchContext) *search.DocumentMatch {
	rvftlm := make(search.FieldTermLocationMap, 0)
	freq := 0
	// typically we would expect there to only actually be results in
	// one field, but we allow for this to not be the case
	// but, we note that phrase constraints can only be satisfied within
	// a single field, so we can check them each independently
	for field, tlm := range s.currMust.Locations {

		f, rvtlm := s.checkCurrMustMatchField(ctx, tlm)
		if f > 0 {
			freq += f
			rvftlm[field] = rvtlm
		}
	}

	if freq > 0 {
		// return match
		rv := s.currMust
		rv.Locations = rvftlm
		return rv
	}

	return nil
}

// checkCurrMustMatchField is soley concerned with determining if one particular
// field within the currMust DocumentMatch Locations satisfies the phase
// constraints (possibly more than once).  if so, the number of times it was
// satisfied, and these locations are returned.  otherwise 0 and either
// a nil or empty TermLocationMap
func (s *PhraseSearcher) checkCurrMustMatchField(ctx *search.SearchContext, tlm search.TermLocationMap) (int, search.TermLocationMap) {
	paths := findPhrasePaths(0, nil, s.terms, tlm, nil, 0)
	rv := make(search.TermLocationMap, len(s.terms))
	for _, p := range paths {
		p.MergeInto(rv)
	}
	return len(paths), rv
}

type phrasePart struct {
	term string
	loc  *search.Location
}

func (p *phrasePart) String() string {
	return fmt.Sprintf("[%s %v]", p.term, p.loc)
}

type phrasePath []*phrasePart

func (p phrasePath) MergeInto(in search.TermLocationMap) {
	for _, pp := range p {
		in[pp.term] = append(in[pp.term], pp.loc)
	}
}

// findPhrasePaths is a function to identify phase matches from a set of known
// term locations.  the implementation is recursive, so care must be taken
// with arguments and return values.
//
// prev - the previous location, nil on first invocation
// phraseTerms - slice containing the phrase terms themselves
//               may contain empty string as placeholder (don't care)
// tlm - the Term Location Map containing all relevant term locations
// offset - the offset from the previous that this next term must match
// p - the current path being explored (appended to in recursive calls)
//     this is the primary state being built during the traversal
//
// returns slice of paths, or nil if invocation did not find any successul paths
func findPhrasePaths(prevPos uint64, ap search.ArrayPositions, phraseTerms [][]string, tlm search.TermLocationMap, p phrasePath, remainingSlop int) []phrasePath {

	// no more terms
	if len(phraseTerms) < 1 {
		return []phrasePath{p}
	}

	car := phraseTerms[0]
	cdr := phraseTerms[1:]

	// empty term is treated as match (continue)
	if len(car) == 0 || (len(car) == 1 && car[0] == "") {
		nextPos := prevPos + 1
		if prevPos == 0 {
			// if prevPos was 0, don't set it to 1 (as thats not a real abs pos)
			nextPos = 0 // don't advance nextPos if prevPos was 0
		}
		return findPhrasePaths(nextPos, ap, cdr, tlm, p, remainingSlop)
	}

	var rv []phrasePath
	// locations for this term
	for _, carTerm := range car {
		locations := tlm[carTerm]
		for _, loc := range locations {
			if prevPos != 0 && !loc.ArrayPositions.Equals(ap) {
				// if the array positions are wrong, can't match, try next location
				continue
			}

			// compute distance from previous phrase term
			dist := 0
			if prevPos != 0 {
				dist = editDistance(prevPos+1, loc.Pos)
			}

			// if enough slop reamining, continue recursively
			if prevPos == 0 || (remainingSlop-dist) >= 0 {
				// this location works, add it to the path (but not for empty term)
				px := append(p, &phrasePart{term: carTerm, loc: loc})
				rv = append(rv, findPhrasePaths(loc.Pos, loc.ArrayPositions, cdr, tlm, px, remainingSlop-dist)...)
			}
		}
	}
	return rv
}

func editDistance(p1, p2 uint64) int {
	dist := int(p1 - p2)
	if dist < 0 {
		return -dist
	}
	return dist
}

func (s *PhraseSearcher) Advance(ctx *search.SearchContext, ID index.IndexInternalID) (*search.DocumentMatch, error) {
	if !s.initialized {
		err := s.initSearchers(ctx)
		if err != nil {
			return nil, err
		}
	}
	if s.currMust != nil {
		if s.currMust.IndexInternalID.Compare(ID) >= 0 {
			return s.Next(ctx)
		}
		ctx.DocumentMatchPool.Put(s.currMust)
	}
	if s.currMust == nil {
		return nil, nil
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
