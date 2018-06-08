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
	"encoding/json"
	"fmt"

	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/mapping"
	"github.com/blevesearch/bleve/search"
	"github.com/blevesearch/bleve/search/searcher"
)

type MultiPhraseQuery struct {
	Terms    [][]string `json:"terms"`
	Field    string     `json:"field,omitempty"`
	BoostVal *Boost     `json:"boost,omitempty"`
}

// NewMultiPhraseQuery creates a new Query for finding
// term phrases in the index.
// It is like PhraseQuery, but each position in the
// phrase may be satisfied by a list of terms
// as opposed to just one.
// At least one of the terms must exist in the correct
// order, at the correct index offsets, in the
// specified field. Queried field must have been indexed with
// IncludeTermVectors set to true.
func NewMultiPhraseQuery(terms [][]string, field string) *MultiPhraseQuery {
	return &MultiPhraseQuery{
		Terms: terms,
		Field: field,
	}
}

func (q *MultiPhraseQuery) SetBoost(b float64) {
	boost := Boost(b)
	q.BoostVal = &boost
}

func (q *MultiPhraseQuery) Boost() float64 {
	return q.BoostVal.Value()
}

func (q *MultiPhraseQuery) Searcher(i index.IndexReader, m mapping.IndexMapping, options search.SearcherOptions) (search.Searcher, error) {
	return searcher.NewMultiPhraseSearcher(i, q.Terms, q.Field, options)
}

func (q *MultiPhraseQuery) Validate() error {
	if len(q.Terms) < 1 {
		return fmt.Errorf("phrase query must contain at least one term")
	}
	return nil
}

func (q *MultiPhraseQuery) UnmarshalJSON(data []byte) error {
	type _mphraseQuery MultiPhraseQuery
	tmp := _mphraseQuery{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	q.Terms = tmp.Terms
	q.Field = tmp.Field
	q.BoostVal = tmp.BoostVal
	return nil
}
