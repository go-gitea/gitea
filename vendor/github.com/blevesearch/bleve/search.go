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

package bleve

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/blevesearch/bleve/analysis"
	"github.com/blevesearch/bleve/analysis/datetime/optional"
	"github.com/blevesearch/bleve/registry"
	"github.com/blevesearch/bleve/search"
	"github.com/blevesearch/bleve/search/query"
)

var cache = registry.NewCache()

const defaultDateTimeParser = optional.Name

type numericRange struct {
	Name string   `json:"name,omitempty"`
	Min  *float64 `json:"min,omitempty"`
	Max  *float64 `json:"max,omitempty"`
}

type dateTimeRange struct {
	Name        string    `json:"name,omitempty"`
	Start       time.Time `json:"start,omitempty"`
	End         time.Time `json:"end,omitempty"`
	startString *string
	endString   *string
}

func (dr *dateTimeRange) ParseDates(dateTimeParser analysis.DateTimeParser) (start, end time.Time) {
	start = dr.Start
	if dr.Start.IsZero() && dr.startString != nil {
		s, err := dateTimeParser.ParseDateTime(*dr.startString)
		if err == nil {
			start = s
		}
	}
	end = dr.End
	if dr.End.IsZero() && dr.endString != nil {
		e, err := dateTimeParser.ParseDateTime(*dr.endString)
		if err == nil {
			end = e
		}
	}
	return start, end
}

func (dr *dateTimeRange) UnmarshalJSON(input []byte) error {
	var temp struct {
		Name  string  `json:"name,omitempty"`
		Start *string `json:"start,omitempty"`
		End   *string `json:"end,omitempty"`
	}

	err := json.Unmarshal(input, &temp)
	if err != nil {
		return err
	}

	dr.Name = temp.Name
	if temp.Start != nil {
		dr.startString = temp.Start
	}
	if temp.End != nil {
		dr.endString = temp.End
	}

	return nil
}

func (dr *dateTimeRange) MarshalJSON() ([]byte, error) {
	rv := map[string]interface{}{
		"name":  dr.Name,
		"start": dr.Start,
		"end":   dr.End,
	}
	if dr.Start.IsZero() && dr.startString != nil {
		rv["start"] = dr.startString
	}
	if dr.End.IsZero() && dr.endString != nil {
		rv["end"] = dr.endString
	}
	return json.Marshal(rv)
}

// A FacetRequest describes a facet or aggregation
// of the result document set you would like to be
// built.
type FacetRequest struct {
	Size           int              `json:"size"`
	Field          string           `json:"field"`
	NumericRanges  []*numericRange  `json:"numeric_ranges,omitempty"`
	DateTimeRanges []*dateTimeRange `json:"date_ranges,omitempty"`
}

func (fr *FacetRequest) Validate() error {
	nrCount := len(fr.NumericRanges)
	drCount := len(fr.DateTimeRanges)
	if nrCount > 0 && drCount > 0 {
		return fmt.Errorf("facet can only conain numeric ranges or date ranges, not both")
	}

	if nrCount > 0 {
		nrNames := map[string]interface{}{}
		for _, nr := range fr.NumericRanges {
			if _, ok := nrNames[nr.Name]; ok {
				return fmt.Errorf("numeric ranges contains duplicate name '%s'", nr.Name)
			}
			nrNames[nr.Name] = struct{}{}
			if nr.Min == nil && nr.Max == nil {
				return fmt.Errorf("numeric range query must specify either min, max or both for range name '%s'", nr.Name)
			}
		}

	} else {
		dateTimeParser, err := cache.DateTimeParserNamed(defaultDateTimeParser)
		if err != nil {
			return err
		}
		drNames := map[string]interface{}{}
		for _, dr := range fr.DateTimeRanges {
			if _, ok := drNames[dr.Name]; ok {
				return fmt.Errorf("date ranges contains duplicate name '%s'", dr.Name)
			}
			drNames[dr.Name] = struct{}{}
			start, end := dr.ParseDates(dateTimeParser)
			if start.IsZero() && end.IsZero() {
				return fmt.Errorf("date range query must specify either start, end or both for range name '%s'", dr.Name)
			}
		}
	}
	return nil
}

// NewFacetRequest creates a facet on the specified
// field that limits the number of entries to the
// specified size.
func NewFacetRequest(field string, size int) *FacetRequest {
	return &FacetRequest{
		Field: field,
		Size:  size,
	}
}

// AddDateTimeRange adds a bucket to a field
// containing date values.  Documents with a
// date value falling into this range are tabulated
// as part of this bucket/range.
func (fr *FacetRequest) AddDateTimeRange(name string, start, end time.Time) {
	if fr.DateTimeRanges == nil {
		fr.DateTimeRanges = make([]*dateTimeRange, 0, 1)
	}
	fr.DateTimeRanges = append(fr.DateTimeRanges, &dateTimeRange{Name: name, Start: start, End: end})
}

// AddDateTimeRangeString adds a bucket to a field
// containing date values.
func (fr *FacetRequest) AddDateTimeRangeString(name string, start, end *string) {
	if fr.DateTimeRanges == nil {
		fr.DateTimeRanges = make([]*dateTimeRange, 0, 1)
	}
	fr.DateTimeRanges = append(fr.DateTimeRanges,
		&dateTimeRange{Name: name, startString: start, endString: end})
}

// AddNumericRange adds a bucket to a field
// containing numeric values.  Documents with a
// numeric value falling into this range are
// tabulated as part of this bucket/range.
func (fr *FacetRequest) AddNumericRange(name string, min, max *float64) {
	if fr.NumericRanges == nil {
		fr.NumericRanges = make([]*numericRange, 0, 1)
	}
	fr.NumericRanges = append(fr.NumericRanges, &numericRange{Name: name, Min: min, Max: max})
}

// FacetsRequest groups together all the
// FacetRequest objects for a single query.
type FacetsRequest map[string]*FacetRequest

func (fr FacetsRequest) Validate() error {
	for _, v := range fr {
		err := v.Validate()
		if err != nil {
			return err
		}
	}
	return nil
}

// HighlightRequest describes how field matches
// should be highlighted.
type HighlightRequest struct {
	Style  *string  `json:"style"`
	Fields []string `json:"fields"`
}

// NewHighlight creates a default
// HighlightRequest.
func NewHighlight() *HighlightRequest {
	return &HighlightRequest{}
}

// NewHighlightWithStyle creates a HighlightRequest
// with an alternate style.
func NewHighlightWithStyle(style string) *HighlightRequest {
	return &HighlightRequest{
		Style: &style,
	}
}

func (h *HighlightRequest) AddField(field string) {
	if h.Fields == nil {
		h.Fields = make([]string, 0, 1)
	}
	h.Fields = append(h.Fields, field)
}

// A SearchRequest describes all the parameters
// needed to search the index.
// Query is required.
// Size/From describe how much and which part of the
// result set to return.
// Highlight describes optional search result
// highlighting.
// Fields describes a list of field values which
// should be retrieved for result documents, provided they
// were stored while indexing.
// Facets describe the set of facets to be computed.
// Explain triggers inclusion of additional search
// result score explanations.
// Sort describes the desired order for the results to be returned.
//
// A special field named "*" can be used to return all fields.
type SearchRequest struct {
	Query            query.Query       `json:"query"`
	Size             int               `json:"size"`
	From             int               `json:"from"`
	Highlight        *HighlightRequest `json:"highlight"`
	Fields           []string          `json:"fields"`
	Facets           FacetsRequest     `json:"facets"`
	Explain          bool              `json:"explain"`
	Sort             search.SortOrder  `json:"sort"`
	IncludeLocations bool              `json:"includeLocations"`
}

func (r *SearchRequest) Validate() error {
	if srq, ok := r.Query.(query.ValidatableQuery); ok {
		err := srq.Validate()
		if err != nil {
			return err
		}
	}

	return r.Facets.Validate()
}

// AddFacet adds a FacetRequest to this SearchRequest
func (r *SearchRequest) AddFacet(facetName string, f *FacetRequest) {
	if r.Facets == nil {
		r.Facets = make(FacetsRequest, 1)
	}
	r.Facets[facetName] = f
}

// SortBy changes the request to use the requested sort order
// this form uses the simplified syntax with an array of strings
// each string can either be a field name
// or the magic value _id and _score which refer to the doc id and search score
// any of these values can optionally be prefixed with - to reverse the order
func (r *SearchRequest) SortBy(order []string) {
	so := search.ParseSortOrderStrings(order)
	r.Sort = so
}

// SortByCustom changes the request to use the requested sort order
func (r *SearchRequest) SortByCustom(order search.SortOrder) {
	r.Sort = order
}

// UnmarshalJSON deserializes a JSON representation of
// a SearchRequest
func (r *SearchRequest) UnmarshalJSON(input []byte) error {
	var temp struct {
		Q                json.RawMessage   `json:"query"`
		Size             *int              `json:"size"`
		From             int               `json:"from"`
		Highlight        *HighlightRequest `json:"highlight"`
		Fields           []string          `json:"fields"`
		Facets           FacetsRequest     `json:"facets"`
		Explain          bool              `json:"explain"`
		Sort             []json.RawMessage `json:"sort"`
		IncludeLocations bool              `json:"includeLocations"`
	}

	err := json.Unmarshal(input, &temp)
	if err != nil {
		return err
	}

	if temp.Size == nil {
		r.Size = 10
	} else {
		r.Size = *temp.Size
	}
	if temp.Sort == nil {
		r.Sort = search.SortOrder{&search.SortScore{Desc: true}}
	} else {
		r.Sort, err = search.ParseSortOrderJSON(temp.Sort)
		if err != nil {
			return err
		}
	}
	r.From = temp.From
	r.Explain = temp.Explain
	r.Highlight = temp.Highlight
	r.Fields = temp.Fields
	r.Facets = temp.Facets
	r.IncludeLocations = temp.IncludeLocations
	r.Query, err = query.ParseQuery(temp.Q)
	if err != nil {
		return err
	}

	if r.Size < 0 {
		r.Size = 10
	}
	if r.From < 0 {
		r.From = 0
	}

	return nil

}

// NewSearchRequest creates a new SearchRequest
// for the Query, using default values for all
// other search parameters.
func NewSearchRequest(q query.Query) *SearchRequest {
	return NewSearchRequestOptions(q, 10, 0, false)
}

// NewSearchRequestOptions creates a new SearchRequest
// for the Query, with the requested size, from
// and explanation search parameters.
// By default results are ordered by score, descending.
func NewSearchRequestOptions(q query.Query, size, from int, explain bool) *SearchRequest {
	return &SearchRequest{
		Query:   q,
		Size:    size,
		From:    from,
		Explain: explain,
		Sort:    search.SortOrder{&search.SortScore{Desc: true}},
	}
}

// IndexErrMap tracks errors with the name of the index where it occurred
type IndexErrMap map[string]error

// MarshalJSON seralizes the error into a string for JSON consumption
func (iem IndexErrMap) MarshalJSON() ([]byte, error) {
	tmp := make(map[string]string, len(iem))
	for k, v := range iem {
		tmp[k] = v.Error()
	}
	return json.Marshal(tmp)
}

func (iem IndexErrMap) UnmarshalJSON(data []byte) error {
	var tmp map[string]string
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	for k, v := range tmp {
		iem[k] = fmt.Errorf("%s", v)
	}
	return nil
}

// SearchStatus is a secion in the SearchResult reporting how many
// underlying indexes were queried, how many were successful/failed
// and a map of any errors that were encountered
type SearchStatus struct {
	Total      int         `json:"total"`
	Failed     int         `json:"failed"`
	Successful int         `json:"successful"`
	Errors     IndexErrMap `json:"errors,omitempty"`
}

// Merge will merge together multiple SearchStatuses during a MultiSearch
func (ss *SearchStatus) Merge(other *SearchStatus) {
	ss.Total += other.Total
	ss.Failed += other.Failed
	ss.Successful += other.Successful
	if len(other.Errors) > 0 {
		if ss.Errors == nil {
			ss.Errors = make(map[string]error)
		}
		for otherIndex, otherError := range other.Errors {
			ss.Errors[otherIndex] = otherError
		}
	}
}

// A SearchResult describes the results of executing
// a SearchRequest.
type SearchResult struct {
	Status   *SearchStatus                  `json:"status"`
	Request  *SearchRequest                 `json:"request"`
	Hits     search.DocumentMatchCollection `json:"hits"`
	Total    uint64                         `json:"total_hits"`
	MaxScore float64                        `json:"max_score"`
	Took     time.Duration                  `json:"took"`
	Facets   search.FacetResults            `json:"facets"`
}

func (sr *SearchResult) String() string {
	rv := ""
	if sr.Total > 0 {
		if sr.Request.Size > 0 {
			rv = fmt.Sprintf("%d matches, showing %d through %d, took %s\n", sr.Total, sr.Request.From+1, sr.Request.From+len(sr.Hits), sr.Took)
			for i, hit := range sr.Hits {
				rv += fmt.Sprintf("%5d. %s (%f)\n", i+sr.Request.From+1, hit.ID, hit.Score)
				for fragmentField, fragments := range hit.Fragments {
					rv += fmt.Sprintf("\t%s\n", fragmentField)
					for _, fragment := range fragments {
						rv += fmt.Sprintf("\t\t%s\n", fragment)
					}
				}
				for otherFieldName, otherFieldValue := range hit.Fields {
					if _, ok := hit.Fragments[otherFieldName]; !ok {
						rv += fmt.Sprintf("\t%s\n", otherFieldName)
						rv += fmt.Sprintf("\t\t%v\n", otherFieldValue)
					}
				}
			}
		} else {
			rv = fmt.Sprintf("%d matches, took %s\n", sr.Total, sr.Took)
		}
	} else {
		rv = "No matches"
	}
	if len(sr.Facets) > 0 {
		rv += fmt.Sprintf("Facets:\n")
		for fn, f := range sr.Facets {
			rv += fmt.Sprintf("%s(%d)\n", fn, f.Total)
			for _, t := range f.Terms {
				rv += fmt.Sprintf("\t%s(%d)\n", t.Term, t.Count)
			}
			if f.Other != 0 {
				rv += fmt.Sprintf("\tOther(%d)\n", f.Other)
			}
		}
	}
	return rv
}

// Merge will merge together multiple SearchResults during a MultiSearch
func (sr *SearchResult) Merge(other *SearchResult) {
	sr.Status.Merge(other.Status)
	sr.Hits = append(sr.Hits, other.Hits...)
	sr.Total += other.Total
	if other.MaxScore > sr.MaxScore {
		sr.MaxScore = other.MaxScore
	}
	sr.Facets.Merge(other.Facets)
}
