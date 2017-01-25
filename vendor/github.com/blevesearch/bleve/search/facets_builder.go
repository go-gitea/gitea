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

package search

import (
	"sort"

	"github.com/blevesearch/bleve/index"
)

type FacetBuilder interface {
	Update(index.FieldTerms)
	Result() *FacetResult
	Field() string
}

type FacetsBuilder struct {
	indexReader index.IndexReader
	facets      map[string]FacetBuilder
	fields      []string
}

func NewFacetsBuilder(indexReader index.IndexReader) *FacetsBuilder {
	return &FacetsBuilder{
		indexReader: indexReader,
		facets:      make(map[string]FacetBuilder, 0),
	}
}

func (fb *FacetsBuilder) Add(name string, facetBuilder FacetBuilder) {
	fb.facets[name] = facetBuilder
}

func (fb *FacetsBuilder) Update(docMatch *DocumentMatch) error {
	if fb.fields == nil {
		for _, facetBuilder := range fb.facets {
			fb.fields = append(fb.fields, facetBuilder.Field())
		}
	}

	if len(fb.fields) > 0 {
		// find out which fields haven't been loaded yet
		fieldsToLoad := docMatch.CachedFieldTerms.FieldsNotYetCached(fb.fields)
		// look them up
		fieldTerms, err := fb.indexReader.DocumentFieldTerms(docMatch.IndexInternalID, fieldsToLoad)
		if err != nil {
			return err
		}
		// cache these as well
		if docMatch.CachedFieldTerms == nil {
			docMatch.CachedFieldTerms = make(map[string][]string)
		}
		docMatch.CachedFieldTerms.Merge(fieldTerms)
	}
	for _, facetBuilder := range fb.facets {
		facetBuilder.Update(docMatch.CachedFieldTerms)
	}
	return nil
}

type TermFacet struct {
	Term  string `json:"term"`
	Count int    `json:"count"`
}

type TermFacets []*TermFacet

func (tf TermFacets) Add(termFacet *TermFacet) TermFacets {
	for _, existingTerm := range tf {
		if termFacet.Term == existingTerm.Term {
			existingTerm.Count += termFacet.Count
			return tf
		}
	}
	// if we got here it wasn't already in the existing terms
	tf = append(tf, termFacet)
	return tf
}

func (tf TermFacets) Len() int      { return len(tf) }
func (tf TermFacets) Swap(i, j int) { tf[i], tf[j] = tf[j], tf[i] }
func (tf TermFacets) Less(i, j int) bool {
	if tf[i].Count == tf[j].Count {
		return tf[i].Term < tf[j].Term
	}
	return tf[i].Count > tf[j].Count
}

type NumericRangeFacet struct {
	Name  string   `json:"name"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
	Count int      `json:"count"`
}

func (nrf *NumericRangeFacet) Same(other *NumericRangeFacet) bool {
	if nrf.Min == nil && other.Min != nil {
		return false
	}
	if nrf.Min != nil && other.Min == nil {
		return false
	}
	if nrf.Min != nil && other.Min != nil && *nrf.Min != *other.Min {
		return false
	}
	if nrf.Max == nil && other.Max != nil {
		return false
	}
	if nrf.Max != nil && other.Max == nil {
		return false
	}
	if nrf.Max != nil && other.Max != nil && *nrf.Max != *other.Max {
		return false
	}

	return true
}

type NumericRangeFacets []*NumericRangeFacet

func (nrf NumericRangeFacets) Add(numericRangeFacet *NumericRangeFacet) NumericRangeFacets {
	for _, existingNr := range nrf {
		if numericRangeFacet.Same(existingNr) {
			existingNr.Count += numericRangeFacet.Count
			return nrf
		}
	}
	// if we got here it wasn't already in the existing terms
	nrf = append(nrf, numericRangeFacet)
	return nrf
}

func (nrf NumericRangeFacets) Len() int      { return len(nrf) }
func (nrf NumericRangeFacets) Swap(i, j int) { nrf[i], nrf[j] = nrf[j], nrf[i] }
func (nrf NumericRangeFacets) Less(i, j int) bool {
	if nrf[i].Count == nrf[j].Count {
		return nrf[i].Name < nrf[j].Name
	}
	return nrf[i].Count > nrf[j].Count
}

type DateRangeFacet struct {
	Name  string  `json:"name"`
	Start *string `json:"start,omitempty"`
	End   *string `json:"end,omitempty"`
	Count int     `json:"count"`
}

func (drf *DateRangeFacet) Same(other *DateRangeFacet) bool {
	if drf.Start == nil && other.Start != nil {
		return false
	}
	if drf.Start != nil && other.Start == nil {
		return false
	}
	if drf.Start != nil && other.Start != nil && *drf.Start != *other.Start {
		return false
	}
	if drf.End == nil && other.End != nil {
		return false
	}
	if drf.End != nil && other.End == nil {
		return false
	}
	if drf.End != nil && other.End != nil && *drf.End != *other.End {
		return false
	}

	return true
}

type DateRangeFacets []*DateRangeFacet

func (drf DateRangeFacets) Add(dateRangeFacet *DateRangeFacet) DateRangeFacets {
	for _, existingDr := range drf {
		if dateRangeFacet.Same(existingDr) {
			existingDr.Count += dateRangeFacet.Count
			return drf
		}
	}
	// if we got here it wasn't already in the existing terms
	drf = append(drf, dateRangeFacet)
	return drf
}

func (drf DateRangeFacets) Len() int      { return len(drf) }
func (drf DateRangeFacets) Swap(i, j int) { drf[i], drf[j] = drf[j], drf[i] }
func (drf DateRangeFacets) Less(i, j int) bool {
	if drf[i].Count == drf[j].Count {
		return drf[i].Name < drf[j].Name
	}
	return drf[i].Count > drf[j].Count
}

type FacetResult struct {
	Field         string             `json:"field"`
	Total         int                `json:"total"`
	Missing       int                `json:"missing"`
	Other         int                `json:"other"`
	Terms         TermFacets         `json:"terms,omitempty"`
	NumericRanges NumericRangeFacets `json:"numeric_ranges,omitempty"`
	DateRanges    DateRangeFacets    `json:"date_ranges,omitempty"`
}

func (fr *FacetResult) Merge(other *FacetResult) {
	fr.Total += other.Total
	fr.Missing += other.Missing
	fr.Other += other.Other
	if fr.Terms != nil && other.Terms != nil {
		for _, term := range other.Terms {
			fr.Terms = fr.Terms.Add(term)
		}
	}
	if fr.NumericRanges != nil && other.NumericRanges != nil {
		for _, nr := range other.NumericRanges {
			fr.NumericRanges = fr.NumericRanges.Add(nr)
		}
	}
	if fr.DateRanges != nil && other.DateRanges != nil {
		for _, dr := range other.DateRanges {
			fr.DateRanges = fr.DateRanges.Add(dr)
		}
	}
}

func (fr *FacetResult) Fixup(size int) {
	if fr.Terms != nil {
		sort.Sort(fr.Terms)
		if len(fr.Terms) > size {
			moveToOther := fr.Terms[size:]
			for _, mto := range moveToOther {
				fr.Other += mto.Count
			}
			fr.Terms = fr.Terms[0:size]
		}
	} else if fr.NumericRanges != nil {
		sort.Sort(fr.NumericRanges)
		if len(fr.NumericRanges) > size {
			moveToOther := fr.NumericRanges[size:]
			for _, mto := range moveToOther {
				fr.Other += mto.Count
			}
			fr.NumericRanges = fr.NumericRanges[0:size]
		}
	} else if fr.DateRanges != nil {
		sort.Sort(fr.DateRanges)
		if len(fr.DateRanges) > size {
			moveToOther := fr.DateRanges[size:]
			for _, mto := range moveToOther {
				fr.Other += mto.Count
			}
			fr.DateRanges = fr.DateRanges[0:size]
		}
	}
}

type FacetResults map[string]*FacetResult

func (fr FacetResults) Merge(other FacetResults) {
	for name, oFacetResult := range other {
		facetResult, ok := fr[name]
		if ok {
			facetResult.Merge(oFacetResult)
		} else {
			fr[name] = oFacetResult
		}
	}
}

func (fr FacetResults) Fixup(name string, size int) {
	facetResult, ok := fr[name]
	if ok {
		facetResult.Fixup(size)
	}
}

func (fb *FacetsBuilder) Results() FacetResults {
	fr := make(FacetResults)
	for facetName, facetBuilder := range fb.facets {
		facetResult := facetBuilder.Result()
		fr[facetName] = facetResult
	}
	return fr
}
