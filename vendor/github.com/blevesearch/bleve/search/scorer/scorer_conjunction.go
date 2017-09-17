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
	"github.com/blevesearch/bleve/search"
)

type ConjunctionQueryScorer struct {
	options search.SearcherOptions
}

func NewConjunctionQueryScorer(options search.SearcherOptions) *ConjunctionQueryScorer {
	return &ConjunctionQueryScorer{
		options: options,
	}
}

func (s *ConjunctionQueryScorer) Score(ctx *search.SearchContext, constituents []*search.DocumentMatch) *search.DocumentMatch {
	var sum float64
	var childrenExplanations []*search.Explanation
	if s.options.Explain {
		childrenExplanations = make([]*search.Explanation, len(constituents))
	}

	locations := []search.FieldTermLocationMap{}
	for i, docMatch := range constituents {
		sum += docMatch.Score
		if s.options.Explain {
			childrenExplanations[i] = docMatch.Expl
		}
		if docMatch.Locations != nil {
			locations = append(locations, docMatch.Locations)
		}
	}
	newScore := sum
	var newExpl *search.Explanation
	if s.options.Explain {
		newExpl = &search.Explanation{Value: sum, Message: "sum of:", Children: childrenExplanations}
	}

	// reuse constituents[0] as the return value
	rv := constituents[0]
	rv.Score = newScore
	rv.Expl = newExpl
	if len(locations) == 1 {
		rv.Locations = locations[0]
	} else if len(locations) > 1 {
		rv.Locations = search.MergeLocations(locations)
	}

	return rv
}
