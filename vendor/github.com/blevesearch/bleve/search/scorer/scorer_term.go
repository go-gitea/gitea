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
	queryTerm              string
	queryField             string
	queryBoost             float64
	docTerm                uint64
	docTotal               uint64
	idf                    float64
	explain                bool
	idfExplanation         *search.Explanation
	queryNorm              float64
	queryWeight            float64
	queryWeightExplanation *search.Explanation
}

func NewTermQueryScorer(queryTerm string, queryField string, queryBoost float64, docTotal, docTerm uint64, explain bool) *TermQueryScorer {
	rv := TermQueryScorer{
		queryTerm:   queryTerm,
		queryField:  queryField,
		queryBoost:  queryBoost,
		docTerm:     docTerm,
		docTotal:    docTotal,
		idf:         1.0 + math.Log(float64(docTotal)/float64(docTerm+1.0)),
		explain:     explain,
		queryWeight: 1.0,
	}

	if explain {
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

	if s.explain {
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

	if s.explain {
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
		if s.explain {
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
	if s.explain {
		rv.Expl = scoreExplanation
	}

	if termMatch.Vectors != nil && len(termMatch.Vectors) > 0 {

		rv.Locations = make(search.FieldTermLocationMap)
		for _, v := range termMatch.Vectors {
			tlm := rv.Locations[v.Field]
			if tlm == nil {
				tlm = make(search.TermLocationMap)
			}

			loc := search.Location{
				Pos:   float64(v.Pos),
				Start: float64(v.Start),
				End:   float64(v.End),
			}

			if len(v.ArrayPositions) > 0 {
				loc.ArrayPositions = make([]float64, len(v.ArrayPositions))
				for i, ap := range v.ArrayPositions {
					loc.ArrayPositions[i] = float64(ap)
				}
			}

			locations := tlm[s.queryTerm]
			if locations == nil {
				locations = make(search.Locations, 1)
				locations[0] = &loc
			} else {
				locations = append(locations, &loc)
			}
			tlm[s.queryTerm] = locations

			rv.Locations[v.Field] = tlm
		}

	}

	return rv
}
