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
	"io/ioutil"
	"log"

	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/mapping"
	"github.com/blevesearch/bleve/search"
)

var logger = log.New(ioutil.Discard, "bleve mapping ", log.LstdFlags)

// SetLog sets the logger used for logging
// by default log messages are sent to ioutil.Discard
func SetLog(l *log.Logger) {
	logger = l
}

// A Query represents a description of the type
// and parameters for a query into the index.
type Query interface {
	Searcher(i index.IndexReader, m mapping.IndexMapping,
		options search.SearcherOptions) (search.Searcher, error)
}

// A BoostableQuery represents a Query which can be boosted
// relative to other queries.
type BoostableQuery interface {
	Query
	SetBoost(b float64)
	Boost() float64
}

// A FieldableQuery represents a Query which can be restricted
// to a single field.
type FieldableQuery interface {
	Query
	SetField(f string)
	Field() string
}

// A ValidatableQuery represents a Query which can be validated
// prior to execution.
type ValidatableQuery interface {
	Query
	Validate() error
}

// ParseQuery deserializes a JSON representation of
// a Query object.
func ParseQuery(input []byte) (Query, error) {
	var tmp map[string]interface{}
	err := json.Unmarshal(input, &tmp)
	if err != nil {
		return nil, err
	}
	_, isMatchQuery := tmp["match"]
	_, hasFuzziness := tmp["fuzziness"]
	if hasFuzziness && !isMatchQuery {
		var rv FuzzyQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, isTermQuery := tmp["term"]
	if isTermQuery {
		var rv TermQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	if isMatchQuery {
		var rv MatchQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, isMatchPhraseQuery := tmp["match_phrase"]
	if isMatchPhraseQuery {
		var rv MatchPhraseQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasMust := tmp["must"]
	_, hasShould := tmp["should"]
	_, hasMustNot := tmp["must_not"]
	if hasMust || hasShould || hasMustNot {
		var rv BooleanQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasTerms := tmp["terms"]
	if hasTerms {
		var rv PhraseQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			// now try multi-phrase
			var rv2 MultiPhraseQuery
			err = json.Unmarshal(input, &rv2)
			if err != nil {
				return nil, err
			}
			return &rv2, nil
		}
		return &rv, nil
	}
	_, hasConjuncts := tmp["conjuncts"]
	if hasConjuncts {
		var rv ConjunctionQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasDisjuncts := tmp["disjuncts"]
	if hasDisjuncts {
		var rv DisjunctionQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}

	_, hasSyntaxQuery := tmp["query"]
	if hasSyntaxQuery {
		var rv QueryStringQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasMin := tmp["min"].(float64)
	_, hasMax := tmp["max"].(float64)
	if hasMin || hasMax {
		var rv NumericRangeQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasMinStr := tmp["min"].(string)
	_, hasMaxStr := tmp["max"].(string)
	if hasMinStr || hasMaxStr {
		var rv TermRangeQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasStart := tmp["start"]
	_, hasEnd := tmp["end"]
	if hasStart || hasEnd {
		var rv DateRangeQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasPrefix := tmp["prefix"]
	if hasPrefix {
		var rv PrefixQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasRegexp := tmp["regexp"]
	if hasRegexp {
		var rv RegexpQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasWildcard := tmp["wildcard"]
	if hasWildcard {
		var rv WildcardQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasMatchAll := tmp["match_all"]
	if hasMatchAll {
		var rv MatchAllQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasMatchNone := tmp["match_none"]
	if hasMatchNone {
		var rv MatchNoneQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasDocIds := tmp["ids"]
	if hasDocIds {
		var rv DocIDQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasBool := tmp["bool"]
	if hasBool {
		var rv BoolFieldQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasTopLeft := tmp["top_left"]
	_, hasBottomRight := tmp["bottom_right"]
	if hasTopLeft && hasBottomRight {
		var rv GeoBoundingBoxQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasDistance := tmp["distance"]
	if hasDistance {
		var rv GeoDistanceQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	_, hasPoints := tmp["polygon_points"]
	if hasPoints {
		var rv GeoBoundingPolygonQuery
		err := json.Unmarshal(input, &rv)
		if err != nil {
			return nil, err
		}
		return &rv, nil
	}
	return nil, fmt.Errorf("unknown query type")
}

// expandQuery traverses the input query tree and returns a new tree where
// query string queries have been expanded into base queries. Returned tree may
// reference queries from the input tree or new queries.
func expandQuery(m mapping.IndexMapping, query Query) (Query, error) {
	var expand func(query Query) (Query, error)
	var expandSlice func(queries []Query) ([]Query, error)

	expandSlice = func(queries []Query) ([]Query, error) {
		expanded := []Query{}
		for _, q := range queries {
			exp, err := expand(q)
			if err != nil {
				return nil, err
			}
			expanded = append(expanded, exp)
		}
		return expanded, nil
	}

	expand = func(query Query) (Query, error) {
		switch q := query.(type) {
		case *QueryStringQuery:
			parsed, err := parseQuerySyntax(q.Query)
			if err != nil {
				return nil, fmt.Errorf("could not parse '%s': %s", q.Query, err)
			}
			return expand(parsed)
		case *ConjunctionQuery:
			children, err := expandSlice(q.Conjuncts)
			if err != nil {
				return nil, err
			}
			q.Conjuncts = children
			return q, nil
		case *DisjunctionQuery:
			children, err := expandSlice(q.Disjuncts)
			if err != nil {
				return nil, err
			}
			q.Disjuncts = children
			return q, nil
		case *BooleanQuery:
			var err error
			q.Must, err = expand(q.Must)
			if err != nil {
				return nil, err
			}
			q.Should, err = expand(q.Should)
			if err != nil {
				return nil, err
			}
			q.MustNot, err = expand(q.MustNot)
			if err != nil {
				return nil, err
			}
			return q, nil
		default:
			return query, nil
		}
	}
	return expand(query)
}

// DumpQuery returns a string representation of the query tree, where query
// string queries have been expanded into base queries. The output format is
// meant for debugging purpose and may change in the future.
func DumpQuery(m mapping.IndexMapping, query Query) (string, error) {
	q, err := expandQuery(m, query)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(q, "", "  ")
	return string(data), err
}
