//  Copyright (c) 2019 Couchbase, Inc.
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
	"github.com/blevesearch/bleve/geo"
	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/numeric"
	"github.com/blevesearch/bleve/search"
	"math"
)

func NewGeoBoundedPolygonSearcher(indexReader index.IndexReader,
	polygon []geo.Point, field string, boost float64,
	options search.SearcherOptions) (search.Searcher, error) {

	// compute the bounding box enclosing the polygon
	topLeftLon, topLeftLat, bottomRightLon, bottomRightLat, err :=
		geo.BoundingRectangleForPolygon(polygon)
	if err != nil {
		return nil, err
	}

	// build a searcher for the bounding box on the polygon
	boxSearcher, err := boxSearcher(indexReader,
		topLeftLon, topLeftLat, bottomRightLon, bottomRightLat,
		field, boost, options, true)
	if err != nil {
		return nil, err
	}

	dvReader, err := indexReader.DocValueReader([]string{field})
	if err != nil {
		return nil, err
	}

	// wrap it in a filtering searcher that checks for the polygon inclusivity
	return NewFilteringSearcher(boxSearcher,
		buildPolygonFilter(dvReader, field, polygon)), nil
}

const float64EqualityThreshold = 1e-6

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= float64EqualityThreshold
}

// buildPolygonFilter returns true if the point lies inside the
// polygon. It is based on the ray-casting technique as referred
// here: https://wrf.ecse.rpi.edu/nikola/pubdetails/pnpoly.html
func buildPolygonFilter(dvReader index.DocValueReader, field string,
	polygon []geo.Point) FilterFunc {
	return func(d *search.DocumentMatch) bool {
		var lon, lat float64
		var found bool

		err := dvReader.VisitDocValues(d.IndexInternalID, func(field string, term []byte) {
			// only consider the values which are shifted 0
			prefixCoded := numeric.PrefixCoded(term)
			shift, err := prefixCoded.Shift()
			if err == nil && shift == 0 {
				i64, err := prefixCoded.Int64()
				if err == nil {
					lon = geo.MortonUnhashLon(uint64(i64))
					lat = geo.MortonUnhashLat(uint64(i64))
					found = true
				}
			}
		})

		// Note: this approach works for points which are strictly inside
		// the polygon. ie it might fail for certain points on the polygon boundaries.
		if err == nil && found {
			nVertices := len(polygon)
			var inside bool
			// check for a direct vertex match
			if almostEqual(polygon[0].Lat, lat) &&
				almostEqual(polygon[0].Lon, lon) {
				return true
			}

			for i := 1; i < nVertices; i++ {
				if almostEqual(polygon[i].Lat, lat) &&
					almostEqual(polygon[i].Lon, lon) {
					return true
				}
				if (polygon[i].Lat > lat) != (polygon[i-1].Lat > lat) &&
					lon < (polygon[i-1].Lon-polygon[i].Lon)*(lat-polygon[i].Lat)/
						(polygon[i-1].Lat-polygon[i].Lat)+polygon[i].Lon {
					inside = !inside
				}
			}
			return inside

		}
		return false
	}
}
