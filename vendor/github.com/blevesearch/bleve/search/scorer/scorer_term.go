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

package scorer

import (
	"fmt"
	"math"

	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/search"
)

type TermQueryScorer struct {
	queryTerm              []byte
	queryField             string
	queryBoost             float64
	docTerm                uint64
	docTotal               uint64
	idf                    float64
	options                search.SearcherOptions
	idfExplanation         *search.Explanation
	queryNorm              float64
	queryWeight            float64
	queryWeightExplanation *search.Explanation
}

func NewTermQueryScorer(queryTerm []byte, queryField string, queryBoost float64, docTotal, docTerm uint64, options search.SearcherOptions) *TermQueryScorer {
	rv := TermQueryScorer{
		queryTerm:   queryTerm,
		queryField:  queryField,
		queryBoost:  queryBoost,
		docTerm:     docTerm,
		docTotal:    docTotal,
		idf:         1.0 + math.Log(float64(docTotal)/float64(docTerm+1.0)),
		options:     options,
		queryWeight: 1.0,
	}

	if options.Explain {
		rv.idfExplanation = &search.Explanation{
			Value:   rv.idf,
			Message: fmt.Sprintf("idf(docFreq=%d, maxDocs=%d)", docTerm, docTotal),
		}
	}

	return &rv
}

func (s *TermQueryScorer) Weight() float64 {
	sum := s.queryBoost * s.idf
	return sum * sum
}

func (s *TermQueryScorer) SetQueryNorm(qnorm float64) {
	s.queryNorm = qnorm

	// update the query weight
	s.queryWeight = s.queryBoost * s.idf * s.queryNorm

	if s.options.Explain {
		childrenExplanations := make([]*search.Explanation, 3)
		childrenExplanations[0] = &search.Explanation{
			Value:   s.queryBoost,
			Message: "boost",
		}
		childrenExplanations[1] = s.idfExplanation
		childrenExplanations[2] = &search.Explanation{
			Value:   s.queryNorm,
			Message: "queryNorm",
		}
		s.queryWeightExplanation = &search.Explanation{
			Value:    s.queryWeight,
			Message:  fmt.Sprintf("queryWeight(%s:%s^%f), product of:", s.queryField, string(s.queryTerm), s.queryBoost),
			Children: childrenExplanations,
		}
	}
}

func (s *TermQueryScorer) Score(ctx *search.SearchContext, termMatch *index.TermFieldDoc) *search.DocumentMatch {
	var scoreExplanation *search.Explanation

	// need to compute score
	var tf float64
	if termMatch.Freq < MaxSqrtCache {
		tf = SqrtCache[int(termMatch.Freq)]
	} else {
		tf = math.Sqrt(float64(termMatch.Freq))
	}
	score := tf * termMatch.Norm * s.idf

	if s.options.Explain {
		childrenExplanations := make([]*search.Explanation, 3)
		childrenExplanations[0] = &search.Explanation{
			Value:   tf,
			Message: fmt.Sprintf("tf(termFreq(%s:%s)=%d", s.queryField, string(s.queryTerm), termMatch.Freq),
		}
		childrenExplanations[1] = &search.Explanation{
			Value:   termMatch.Norm,
			Message: fmt.Sprintf("fieldNorm(field=%s, doc=%s)", s.queryField, termMatch.ID),
		}
		childrenExplanations[2] = s.idfExplanation
		scoreExplanation = &search.Explanation{
			Value:    score,
			Message:  fmt.Sprintf("fieldWeight(%s:%s in %s), product of:", s.queryField, string(s.queryTerm), termMatch.ID),
			Children: childrenExplanations,
		}
	}

	// if the query weight isn't 1, multiply
	if s.queryWeight != 1.0 {
		score = score * s.queryWeight
		if s.options.Explain {
			childExplanations := make([]*search.Explanation, 2)
			childExplanations[0] = s.queryWeightExplanation
			childExplanations[1] = scoreExplanation
			scoreExplanation = &search.Explanation{
				Value:    score,
				Message:  fmt.Sprintf("weight(%s:%s^%f in %s), product of:", s.queryField, string(s.queryTerm), s.queryBoost, termMatch.ID),
				Children: childExplanations,
			}
		}
	}

	rv := ctx.DocumentMatchPool.Get()
	rv.IndexInternalID = append(rv.IndexInternalID, termMatch.ID...)
	rv.Score = score
	if s.options.Explain {
		rv.Expl = scoreExplanation
	}

	if termMatch.Vectors != nil && len(termMatch.Vectors) > 0 {
		locs := make([]search.Location, len(termMatch.Vectors))
		locsUsed := 0

		totalPositions := 0
		for _, v := range termMatch.Vectors {
			totalPositions += len(v.ArrayPositions)
		}
		positions := make(search.ArrayPositions, totalPositions)
		positionsUsed := 0

		rv.Locations = make(search.FieldTermLocationMap)
		for _, v := range termMatch.Vectors {
			tlm := rv.Locations[v.Field]
			if tlm == nil {
				tlm = make(search.TermLocationMap)
				rv.Locations[v.Field] = tlm
			}

			loc := &locs[locsUsed]
			locsUsed++

			loc.Pos = v.Pos
			loc.Start = v.Start
			loc.End = v.End

			if len(v.ArrayPositions) > 0 {
				loc.ArrayPositions = positions[positionsUsed : positionsUsed+len(v.ArrayPositions)]
				for i, ap := range v.ArrayPositions {
					loc.ArrayPositions[i] = ap
				}
				positionsUsed += len(v.ArrayPositions)
			}

			tlm[string(s.queryTerm)] = append(tlm[string(s.queryTerm)], loc)
		}
	}

	return rv
}
