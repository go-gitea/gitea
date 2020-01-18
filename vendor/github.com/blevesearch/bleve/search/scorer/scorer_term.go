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
	"reflect"

	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/search"
	"github.com/blevesearch/bleve/size"
)

var reflectStaticSizeTermQueryScorer int

func init() {
	var tqs TermQueryScorer
	reflectStaticSizeTermQueryScorer = int(reflect.TypeOf(tqs).Size())
}

type TermQueryScorer struct {
	queryTerm              string
	queryField             string
	queryBoost             float64
	docTerm                uint64
	docTotal               uint64
	idf                    float64
	options                search.SearcherOptions
	idfExplanation         *search.Explanation
	includeScore           bool
	queryNorm              float64
	queryWeight            float64
	queryWeightExplanation *search.Explanation
}

func (s *TermQueryScorer) Size() int {
	sizeInBytes := reflectStaticSizeTermQueryScorer + size.SizeOfPtr +
		len(s.queryTerm) + len(s.queryField)

	if s.idfExplanation != nil {
		sizeInBytes += s.idfExplanation.Size()
	}

	if s.queryWeightExplanation != nil {
		sizeInBytes += s.queryWeightExplanation.Size()
	}

	return sizeInBytes
}

func NewTermQueryScorer(queryTerm []byte, queryField string, queryBoost float64, docTotal, docTerm uint64, options search.SearcherOptions) *TermQueryScorer {
	rv := TermQueryScorer{
		queryTerm:    string(queryTerm),
		queryField:   queryField,
		queryBoost:   queryBoost,
		docTerm:      docTerm,
		docTotal:     docTotal,
		idf:          1.0 + math.Log(float64(docTotal)/float64(docTerm+1.0)),
		options:      options,
		queryWeight:  1.0,
		includeScore: options.Score != "none",
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
			Message:  fmt.Sprintf("queryWeight(%s:%s^%f), product of:", s.queryField, s.queryTerm, s.queryBoost),
			Children: childrenExplanations,
		}
	}
}

func (s *TermQueryScorer) Score(ctx *search.SearchContext, termMatch *index.TermFieldDoc) *search.DocumentMatch {
	rv := ctx.DocumentMatchPool.Get()
	// perform any score computations only when needed
	if s.includeScore || s.options.Explain {
		var scoreExplanation *search.Explanation
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
				Message: fmt.Sprintf("tf(termFreq(%s:%s)=%d", s.queryField, s.queryTerm, termMatch.Freq),
			}
			childrenExplanations[1] = &search.Explanation{
				Value:   termMatch.Norm,
				Message: fmt.Sprintf("fieldNorm(field=%s, doc=%s)", s.queryField, termMatch.ID),
			}
			childrenExplanations[2] = s.idfExplanation
			scoreExplanation = &search.Explanation{
				Value:    score,
				Message:  fmt.Sprintf("fieldWeight(%s:%s in %s), product of:", s.queryField, s.queryTerm, termMatch.ID),
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
					Message:  fmt.Sprintf("weight(%s:%s^%f in %s), product of:", s.queryField, s.queryTerm, s.queryBoost, termMatch.ID),
					Children: childExplanations,
				}
			}
		}

		if s.includeScore {
			rv.Score = score
		}

		if s.options.Explain {
			rv.Expl = scoreExplanation
		}
	}

	rv.IndexInternalID = append(rv.IndexInternalID, termMatch.ID...)

	if len(termMatch.Vectors) > 0 {
		if cap(rv.FieldTermLocations) < len(termMatch.Vectors) {
			rv.FieldTermLocations = make([]search.FieldTermLocation, 0, len(termMatch.Vectors))
		}

		for _, v := range termMatch.Vectors {
			var ap search.ArrayPositions
			if len(v.ArrayPositions) > 0 {
				n := len(rv.FieldTermLocations)
				if n < cap(rv.FieldTermLocations) { // reuse ap slice if available
					ap = rv.FieldTermLocations[:n+1][n].Location.ArrayPositions[:0]
				}
				ap = append(ap, v.ArrayPositions...)
			}
			rv.FieldTermLocations =
				append(rv.FieldTermLocations, search.FieldTermLocation{
					Field: v.Field,
					Term:  s.queryTerm,
					Location: search.Location{
						Pos:            v.Pos,
						Start:          v.Start,
						End:            v.End,
						ArrayPositions: ap,
					},
				})
		}
	}

	return rv
}
