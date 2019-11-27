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

type filterFunc func(key []byte) bool

var GeoBitsShift1 = (geo.GeoBits << 1)
var GeoBitsShift1Minus1 = GeoBitsShift1 - 1

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
	onBoundaryTerms, notOnBoundaryTerms, err := ComputeGeoRange(0, GeoBitsShift1Minus1,
		minLon, minLat, maxLon, maxLat, checkBoundaries, indexReader, field)
	if err != nil {
		return nil, err
	}

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
	sminLon, sminLat, smaxLon, smaxLat float64, checkBoundaries bool,
	indexReader index.IndexReader, field string) (
	onBoundary [][]byte, notOnBoundary [][]byte, err error) {
	preallocBytesLen := 32
	preallocBytes := make([]byte, preallocBytesLen)

	makePrefixCoded := func(in int64, shift uint) (rv numeric.PrefixCoded) {
		if len(preallocBytes) <= 0 {
			preallocBytesLen = preallocBytesLen * 2
			preallocBytes = make([]byte, preallocBytesLen)
		}

		rv, preallocBytes, err =
			numeric.NewPrefixCodedInt64Prealloc(in, shift, preallocBytes)

		return rv
	}

	var fieldDict index.FieldDictContains
	var isIndexed filterFunc
	if irr, ok := indexReader.(index.IndexReaderContains); ok {
		fieldDict, err = irr.FieldDictContains(field)
		if err != nil {
			return nil, nil, err
		}

		isIndexed = func(term []byte) bool {
			found, err := fieldDict.Contains(term)
			return err == nil && found
		}
	}

	defer func() {
		if fieldDict != nil {
			if fd, ok := fieldDict.(index.FieldDict); ok {
				cerr := fd.Close()
				if cerr != nil {
					err = cerr
				}
			}
		}
	}()

	if isIndexed == nil {
		isIndexed = func(term []byte) bool {
			if indexReader != nil {
				reader, err := indexReader.TermFieldReader(term, field, false, false, false)
				if err != nil || reader == nil {
					return false
				}
				if reader.Count() == 0 {
					_ = reader.Close()
					return false
				}
				_ = reader.Close()
			}
			return true
		}
	}

	var computeGeoRange func(term uint64, shift uint) // declare for recursion

	relateAndRecurse := func(start, end uint64, res, level uint) {
		minLon := geo.MortonUnhashLon(start)
		minLat := geo.MortonUnhashLat(start)
		maxLon := geo.MortonUnhashLon(end)
		maxLat := geo.MortonUnhashLat(end)

		within := res%document.GeoPrecisionStep == 0 &&
			geo.RectWithin(minLon, minLat, maxLon, maxLat,
				sminLon, sminLat, smaxLon, smaxLat)
		if within || (level == geoDetailLevel &&
			geo.RectIntersects(minLon, minLat, maxLon, maxLat,
				sminLon, sminLat, smaxLon, smaxLat)) {
			codedTerm := makePrefixCoded(int64(start), res)
			if isIndexed(codedTerm) {
				if !within && checkBoundaries {
					onBoundary = append(onBoundary, codedTerm)
				} else {
					notOnBoundary = append(notOnBoundary, codedTerm)
				}
			}
		} else if level < geoDetailLevel &&
			geo.RectIntersects(minLon, minLat, maxLon, maxLat,
				sminLon, sminLat, smaxLon, smaxLat) {
			computeGeoRange(start, res-1)
		}
	}

	computeGeoRange = func(term uint64, shift uint) {
		if err != nil {
			return
		}

		split := term | uint64(0x1)<<shift
		var upperMax uint64
		if shift < 63 {
			upperMax = term | ((uint64(1) << (shift + 1)) - 1)
		} else {
			upperMax = 0xffffffffffffffff
		}

		lowerMax := split - 1

		level := (GeoBitsShift1 - shift) >> 1

		relateAndRecurse(term, lowerMax, shift, level)
		relateAndRecurse(split, upperMax, shift, level)
	}

	computeGeoRange(term, shift)

	if err != nil {
		return nil, nil, err
	}

	return onBoundary, notOnBoundary, err
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
