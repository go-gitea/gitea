// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"code.gitea.io/gitea/modules/optional"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

// NumericEqualityQuery generates a numeric equality query for the given value and field
func NumericEqualityQuery(value int64, field string) *query.NumericRangeQuery {
	f := float64(value)
	tru := true
	q := bleve.NewNumericRangeInclusiveQuery(&f, &f, &tru, &tru)
	q.SetField(field)
	return q
}

// MatchPhraseQuery generates a match phrase query for the given phrase, field and analyzer
func MatchPhraseQuery(matchPhrase, field, analyzer string, fuzziness int) *query.MatchPhraseQuery {
	q := bleve.NewMatchPhraseQuery(matchPhrase)
	q.FieldVal = field
	q.Analyzer = analyzer
	q.Fuzziness = fuzziness
	return q
}

// MatchAndQuery generates a match query for the given phrase, field and analyzer
func MatchAndQuery(matchPhrase, field, analyzer string, fuzziness int) *query.MatchQuery {
	q := bleve.NewMatchQuery(matchPhrase)
	q.FieldVal = field
	q.Analyzer = analyzer
	q.Fuzziness = fuzziness
	q.Operator = query.MatchQueryOperatorAnd
	return q
}

// BoolFieldQuery generates a bool field query for the given value and field
func BoolFieldQuery(value bool, field string) *query.BoolFieldQuery {
	q := bleve.NewBoolFieldQuery(value)
	q.SetField(field)
	return q
}

func NumericRangeInclusiveQuery(minOption, maxOption optional.Option[int64], field string) *query.NumericRangeQuery {
	var minF, maxF *float64
	var minI, maxI *bool
	if minOption.Has() {
		minF = new(float64)
		*minF = float64(minOption.Value())
		minI = new(bool)
		*minI = true
	}
	if maxOption.Has() {
		maxF = new(float64)
		*maxF = float64(maxOption.Value())
		maxI = new(bool)
		*maxI = true
	}
	q := bleve.NewNumericRangeInclusiveQuery(minF, maxF, minI, maxI)
	q.SetField(field)
	return q
}
