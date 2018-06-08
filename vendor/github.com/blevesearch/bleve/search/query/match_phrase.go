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

package query

import (
	"fmt"

	"github.com/blevesearch/bleve/analysis"
	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/mapping"
	"github.com/blevesearch/bleve/search"
)

type MatchPhraseQuery struct {
	MatchPhrase string `json:"match_phrase"`
	FieldVal    string `json:"field,omitempty"`
	Analyzer    string `json:"analyzer,omitempty"`
	BoostVal    *Boost `json:"boost,omitempty"`
}

// NewMatchPhraseQuery creates a new Query object
// for matching phrases in the index.
// An Analyzer is chosen based on the field.
// Input text is analyzed using this analyzer.
// Token terms resulting from this analysis are
// used to build a search phrase.  Result documents
// must match this phrase. Queried field must have been indexed with
// IncludeTermVectors set to true.
func NewMatchPhraseQuery(matchPhrase string) *MatchPhraseQuery {
	return &MatchPhraseQuery{
		MatchPhrase: matchPhrase,
	}
}

func (q *MatchPhraseQuery) SetBoost(b float64) {
	boost := Boost(b)
	q.BoostVal = &boost
}

func (q *MatchPhraseQuery) Boost() float64 {
	return q.BoostVal.Value()
}

func (q *MatchPhraseQuery) SetField(f string) {
	q.FieldVal = f
}

func (q *MatchPhraseQuery) Field() string {
	return q.FieldVal
}

func (q *MatchPhraseQuery) Searcher(i index.IndexReader, m mapping.IndexMapping, options search.SearcherOptions) (search.Searcher, error) {
	field := q.FieldVal
	if q.FieldVal == "" {
		field = m.DefaultSearchField()
	}

	analyzerName := ""
	if q.Analyzer != "" {
		analyzerName = q.Analyzer
	} else {
		analyzerName = m.AnalyzerNameForPath(field)
	}
	analyzer := m.AnalyzerNamed(analyzerName)
	if analyzer == nil {
		return nil, fmt.Errorf("no analyzer named '%s' registered", q.Analyzer)
	}

	tokens := analyzer.Analyze([]byte(q.MatchPhrase))
	if len(tokens) > 0 {
		phrase := tokenStreamToPhrase(tokens)
		phraseQuery := NewMultiPhraseQuery(phrase, field)
		phraseQuery.SetBoost(q.BoostVal.Value())
		return phraseQuery.Searcher(i, m, options)
	}
	noneQuery := NewMatchNoneQuery()
	return noneQuery.Searcher(i, m, options)
}

func tokenStreamToPhrase(tokens analysis.TokenStream) [][]string {
	firstPosition := int(^uint(0) >> 1)
	lastPosition := 0
	for _, token := range tokens {
		if token.Position < firstPosition {
			firstPosition = token.Position
		}
		if token.Position > lastPosition {
			lastPosition = token.Position
		}
	}
	phraseLen := lastPosition - firstPosition + 1
	if phraseLen > 0 {
		rv := make([][]string, phraseLen)
		for _, token := range tokens {
			pos := token.Position - firstPosition
			rv[pos] = append(rv[pos], string(token.Term))
		}
		return rv
	}
	return nil
}
