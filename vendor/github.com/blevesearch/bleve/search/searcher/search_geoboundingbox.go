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

var GeoBitsShift1 = geo.GeoBits << 1
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
type closeFunc func() error

func ComputeGeoRange(term uint64, shift uint,
	sminLon, sminLat, smaxLon, smaxLat float64, checkBoundaries bool,
	indexReader index.IndexReader, field string) (
	onBoundary [][]byte, notOnBoundary [][]byte, err error) {

	isIndexed, closeF, err := buildIsIndexedFunc(indexReader, field)
	if closeF != nil {
		defer func() {
			cerr := closeF()
			if cerr != nil {
				err = cerr
			}
		}()
	}

	grc := &geoRangeCompute{
		preallocBytesLen: 32,
		preallocBytes:    make([]byte, 32),
		sminLon:          sminLon,
		sminLat:          sminLat,
		smaxLon:          smaxLon,
		smaxLat:          smaxLat,
		checkBoundaries:  checkBoundaries,
		isIndexed:        isIndexed,
	}

	grc.computeGeoRange(term, shift)

	return grc.onBoundary, grc.notOnBoundary, nil
}

func buildIsIndexedFunc(indexReader index.IndexReader, field string) (isIndexed filterFunc, closeF closeFunc, err error) {
	if irr, ok := indexReader.(index.IndexReaderContains); ok {
		fieldDict, err := irr.FieldDictContains(field)
		if err != nil {
			return nil, nil, err
		}

		isIndexed = func(term []byte) bool {
			found, err := fieldDict.Contains(term)
			return err == nil && found
		}

		closeF = func() error {
			if fd, ok := fieldDict.(index.FieldDict); ok {
				err := fd.Close()
				if err != nil {
					return err
				}
			}
			return nil
		}
	} else if indexReader != nil {
		isIndexed = func(term []byte) bool {
				reader, err := indexReader.TermFieldReader(term, field, false, false, false)
				if err != nil || reader == nil {
					return false
				}
				if reader.Count() == 0 {
					_ = reader.Close()
					return false
				}
				_ = reader.Close()
				return true
		}

	} else {
		isIndexed = func([]byte) bool {
			return true
		}
	}
	return isIndexed, closeF, err
}

func buildRectFilter(dvReader index.DocValueReader, field string,
	minLon, minLat, maxLon, maxLat float64) FilterFunc {
	return func(d *search.DocumentMatch) bool {
		// check geo matches against all numeric type terms indexed
		var lons, lats []float64
		var found bool
		err := dvReader.VisitDocValues(d.IndexInternalID, func(field string, term []byte) {
			// only consider the values which are shifted 0
			prefixCoded := numeric.PrefixCoded(term)
			shift, err := prefixCoded.Shift()
			if err == nil && shift == 0 {
				var i64 int64
				i64, err = prefixCoded.Int64()
				if err == nil {
					lons = append(lons, geo.MortonUnhashLon(uint64(i64)))
					lats = append(lats, geo.MortonUnhashLat(uint64(i64)))
					found = true
				}
			}
		})
		if err == nil && found {
			for i := range lons {
				if geo.BoundingBoxContains(lons[i], lats[i],
					minLon, minLat, maxLon, maxLat) {
					return true
				}
			}
		}
		return false
	}
}

type geoRangeCompute struct {
	preallocBytesLen int
	preallocBytes []byte
	sminLon, sminLat, smaxLon, smaxLat float64
	checkBoundaries bool
	onBoundary, notOnBoundary [][]byte
	isIndexed func(term []byte) bool
}

func (grc *geoRangeCompute) makePrefixCoded(in int64, shift uint) (rv numeric.PrefixCoded) {
	if len(grc.preallocBytes) <= 0 {
		grc.preallocBytesLen = grc.preallocBytesLen * 2
		grc.preallocBytes = make([]byte, grc.preallocBytesLen)
	}

	rv, grc.preallocBytes, _ =
		numeric.NewPrefixCodedInt64Prealloc(in, shift, grc.preallocBytes)

	return rv
}

func (grc *geoRangeCompute) computeGeoRange(term uint64, shift uint) {
	split := term | uint64(0x1)<<shift
	var upperMax uint64
	if shift < 63 {
		upperMax = term | ((uint64(1) << (shift + 1)) - 1)
	} else {
		upperMax = 0xffffffffffffffff
	}
	lowerMax := split - 1
	grc.relateAndRecurse(term, lowerMax, shift)
	grc.relateAndRecurse(split, upperMax, shift)
}

func (grc *geoRangeCompute) relateAndRecurse(start, end uint64, res uint) {
	minLon := geo.MortonUnhashLon(start)
	minLat := geo.MortonUnhashLat(start)
	maxLon := geo.MortonUnhashLon(end)
	maxLat := geo.MortonUnhashLat(end)

	level := (GeoBitsShift1 - res) >> 1

	within := res%document.GeoPrecisionStep == 0 &&
		geo.RectWithin(minLon, minLat, maxLon, maxLat,
			grc.sminLon, grc.sminLat, grc.smaxLon, grc.smaxLat)
	if within || (level == geoDetailLevel &&
		geo.RectIntersects(minLon, minLat, maxLon, maxLat,
			grc.sminLon, grc.sminLat, grc.smaxLon, grc.smaxLat)) {
		codedTerm := grc.makePrefixCoded(int64(start), res)
		if grc.isIndexed(codedTerm) {
			if !within && grc.checkBoundaries {
				grc.onBoundary = append(grc.onBoundary, codedTerm)
			} else {
				grc.notOnBoundary = append(grc.notOnBoundary, codedTerm)
			}
		}
	} else if level < geoDetailLevel &&
		geo.RectIntersects(minLon, minLat, maxLon, maxLat,
			grc.sminLon, grc.sminLat, grc.smaxLon, grc.smaxLat) {
		grc.computeGeoRange(start, res-1)
	}
}