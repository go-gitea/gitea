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
	"math"
	"time"

	"github.com/blevesearch/bleve/analysis/datetime/optional"
	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/mapping"
	"github.com/blevesearch/bleve/numeric"
	"github.com/blevesearch/bleve/registry"
	"github.com/blevesearch/bleve/search"
	"github.com/blevesearch/bleve/search/searcher"
)

// QueryDateTimeParser controls the default query date time parser
var QueryDateTimeParser = optional.Name

// QueryDateTimeFormat controls the format when Marshaling to JSON
var QueryDateTimeFormat = time.RFC3339

var cache = registry.NewCache()

type BleveQueryTime struct {
	time.Time
}

func queryTimeFromString(t string) (time.Time, error) {
	dateTimeParser, err := cache.DateTimeParserNamed(QueryDateTimeParser)
	if err != nil {
		return time.Time{}, err
	}
	rv, err := dateTimeParser.ParseDateTime(t)
	if err != nil {
		return time.Time{}, err
	}
	return rv, nil
}

func (t *BleveQueryTime) MarshalJSON() ([]byte, error) {
	tt := time.Time(t.Time)
	return []byte("\"" + tt.Format(QueryDateTimeFormat) + "\""), nil
}

func (t *BleveQueryTime) UnmarshalJSON(data []byte) error {
	var timeString string
	err := json.Unmarshal(data, &timeString)
	if err != nil {
		return err
	}
	dateTimeParser, err := cache.DateTimeParserNamed(QueryDateTimeParser)
	if err != nil {
		return err
	}
	t.Time, err = dateTimeParser.ParseDateTime(timeString)
	if err != nil {
		return err
	}
	return nil
}

type DateRangeQuery struct {
	Start          BleveQueryTime `json:"start,omitempty"`
	End            BleveQueryTime `json:"end,omitempty"`
	InclusiveStart *bool          `json:"inclusive_start,omitempty"`
	InclusiveEnd   *bool          `json:"inclusive_end,omitempty"`
	FieldVal       string         `json:"field,omitempty"`
	BoostVal       *Boost         `json:"boost,omitempty"`
}

// NewDateRangeQuery creates a new Query for ranges
// of date values.
// Date strings are parsed using the DateTimeParser configured in the
//  top-level config.QueryDateTimeParser
// Either, but not both endpoints can be nil.
func NewDateRangeQuery(start, end time.Time) *DateRangeQuery {
	return NewDateRangeInclusiveQuery(start, end, nil, nil)
}

// NewDateRangeInclusiveQuery creates a new Query for ranges
// of date values.
// Date strings are parsed using the DateTimeParser configured in the
//  top-level config.QueryDateTimeParser
// Either, but not both endpoints can be nil.
// startInclusive and endInclusive control inclusion of the endpoints.
func NewDateRangeInclusiveQuery(start, end time.Time, startInclusive, endInclusive *bool) *DateRangeQuery {
	return &DateRangeQuery{
		Start:          BleveQueryTime{start},
		End:            BleveQueryTime{end},
		InclusiveStart: startInclusive,
		InclusiveEnd:   endInclusive,
	}
}

func (q *DateRangeQuery) SetBoost(b float64) {
	boost := Boost(b)
	q.BoostVal = &boost
}

func (q *DateRangeQuery) Boost() float64 {
	return q.BoostVal.Value()
}

func (q *DateRangeQuery) SetField(f string) {
	q.FieldVal = f
}

func (q *DateRangeQuery) Field() string {
	return q.FieldVal
}

func (q *DateRangeQuery) Searcher(i index.IndexReader, m mapping.IndexMapping, options search.SearcherOptions) (search.Searcher, error) {
	min, max, err := q.parseEndpoints()
	if err != nil {
		return nil, err
	}

	field := q.FieldVal
	if q.FieldVal == "" {
		field = m.DefaultSearchField()
	}

	return searcher.NewNumericRangeSearcher(i, min, max, q.InclusiveStart, q.InclusiveEnd, field, q.BoostVal.Value(), options)
}

func (q *DateRangeQuery) parseEndpoints() (*float64, *float64, error) {
	min := math.Inf(-1)
	max := math.Inf(1)
	if !q.Start.IsZero() {
		min = numeric.Int64ToFloat64(q.Start.UnixNano())
	}
	if !q.End.IsZero() {
		max = numeric.Int64ToFloat64(q.End.UnixNano())
	}

	return &min, &max, nil
}

func (q *DateRangeQuery) Validate() error {
	if q.Start.IsZero() && q.End.IsZero() {
		return fmt.Errorf("must specify start or end")
	}
	_, _, err := q.parseEndpoints()
	if err != nil {
		return err
	}
	return nil
}
