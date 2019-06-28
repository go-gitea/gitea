//  Copyright (c) 2017 Couchbase, Inc.
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

package searcher

import (
	"github.com/blevesearch/bleve/document"
	"github.com/blevesearch/bleve/geo"
	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/numeric"
	"github.com/blevesearch/bleve/search"
)

func NewGeoBoundingBoxSearcher(indexReader index.IndexReader, minLon, minLat,
	maxLon, maxLat float64, field string, boost float64,
	options search.SearcherOptions, checkBoundaries bool) (
	search.Searcher, error) {

	// track list of opened searchers, for cleanup on early exit
	var openedSearchers []search.Searcher
	cleanupOpenedSearchers := func() {
		for _, s := range openedSearchers {
			_ = s.Close()
		}
	}

	// do math to produce list of terms needed for this search
	onBoundaryTerms, notOnBoundaryTerms := ComputeGeoRange(0, (geo.GeoBits<<1)-1,
		minLon, minLat, maxLon, maxLat, checkBoundaries)

	var onBoundarySearcher search.Searcher
	dvReader, err := indexReader.DocValueReader([]string{field})
	if err != nil {
		return nil, err
	}

	if len(onBoundaryTerms) > 0 {
		rawOnBoundarySearcher, err := NewMultiTermSearcherBytes(indexReader,
			onBoundaryTerms, field, boost, options, false)
		if err != nil {
			return nil, err
		}
		// add filter to check points near the boundary
		onBoundarySearcher = NewFilteringSearcher(rawOnBoundarySearcher,
			buildRectFilter(dvReader, field, minLon, minLat, maxLon, maxLat))
		openedSearchers = append(openedSearchers, onBoundarySearcher)
	}

	var notOnBoundarySearcher search.Searcher
	if len(notOnBoundaryTerms) > 0 {
		var err error
		notOnBoundarySearcher, err = NewMultiTermSearcherBytes(indexReader,
			notOnBoundaryTerms, field, boost, options, false)
		if err != nil {
			cleanupOpenedSearchers()
			return nil, err
		}
		openedSearchers = append(openedSearchers, notOnBoundarySearcher)
	}

	if onBoundarySearcher != nil && notOnBoundarySearcher != nil {
		rv, err := NewDisjunctionSearcher(indexReader,
			[]search.Searcher{
				onBoundarySearcher,
				notOnBoundarySearcher,
			},
			0, options)
		if err != nil {
			cleanupOpenedSearchers()
			return nil, err
		}
		return rv, nil
	} else if onBoundarySearcher != nil {
		return onBoundarySearcher, nil
	} else if notOnBoundarySearcher != nil {
		return notOnBoundarySearcher, nil
	}

	return NewMatchNoneSearcher(indexReader)
}

var geoMaxShift = document.GeoPrecisionStep * 4
var geoDetailLevel = ((geo.GeoBits << 1) - geoMaxShift) / 2

func ComputeGeoRange(term uint64, shift uint,
	sminLon, sminLat, smaxLon, smaxLat float64,
	checkBoundaries bool) (
	onBoundary [][]byte, notOnBoundary [][]byte) {
	split := term | uint64(0x1)<<shift
	var upperMax uint64
	if shift < 63 {
		upperMax = term | ((uint64(1) << (shift + 1)) - 1)
	} else {
		upperMax = 0xffffffffffffffff
	}
	lowerMax := split - 1
	onBoundary, notOnBoundary = relateAndRecurse(term, lowerMax, shift,
		sminLon, sminLat, smaxLon, smaxLat, checkBoundaries)
	plusOnBoundary, plusNotOnBoundary := relateAndRecurse(split, upperMax, shift,
		sminLon, sminLat, smaxLon, smaxLat, checkBoundaries)
	onBoundary = append(onBoundary, plusOnBoundary...)
	notOnBoundary = append(notOnBoundary, plusNotOnBoundary...)
	return
}

func relateAndRecurse(start, end uint64, res uint,
	sminLon, sminLat, smaxLon, smaxLat float64,
	checkBoundaries bool) (
	onBoundary [][]byte, notOnBoundary [][]byte) {
	minLon := geo.MortonUnhashLon(start)
	minLat := geo.MortonUnhashLat(start)
	maxLon := geo.MortonUnhashLon(end)
	maxLat := geo.MortonUnhashLat(end)

	level := ((geo.GeoBits << 1) - res) >> 1

	within := res%document.GeoPrecisionStep == 0 &&
		geo.RectWithin(minLon, minLat, maxLon, maxLat,
			sminLon, sminLat, smaxLon, smaxLat)
	if within || (level == geoDetailLevel &&
		geo.RectIntersects(minLon, minLat, maxLon, maxLat,
			sminLon, sminLat, smaxLon, smaxLat)) {
		if !within && checkBoundaries {
			return [][]byte{
				numeric.MustNewPrefixCodedInt64(int64(start), res),
			}, nil
		}
		return nil,
			[][]byte{
				numeric.MustNewPrefixCodedInt64(int64(start), res),
			}
	} else if level < geoDetailLevel &&
		geo.RectIntersects(minLon, minLat, maxLon, maxLat,
			sminLon, sminLat, smaxLon, smaxLat) {
		return ComputeGeoRange(start, res-1, sminLon, sminLat, smaxLon, smaxLat,
			checkBoundaries)
	}
	return nil, nil
}

func buildRectFilter(dvReader index.DocValueReader, field string,
	minLon, minLat, maxLon, maxLat float64) FilterFunc {
	return func(d *search.DocumentMatch) bool {
		var lon, lat float64
		var found bool
		err := dvReader.VisitDocValues(d.IndexInternalID, func(field string, term []byte) {
			// only consider the values which are shifted 0
			prefixCoded := numeric.PrefixCoded(term)
			shift, err := prefixCoded.Shift()
			if err == nil && shift == 0 {
				var i64 int64
				i64, err = prefixCoded.Int64()
				if err == nil {
					lon = geo.MortonUnhashLon(uint64(i64))
					lat = geo.MortonUnhashLat(uint64(i64))
					found = true
				}
			}
		})
		if err == nil && found {
			return geo.BoundingBoxContains(lon, lat,
				minLon, minLat, maxLon, maxLat)
		}
		return false
	}
}
