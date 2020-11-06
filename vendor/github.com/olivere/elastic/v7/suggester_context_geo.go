// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// -- SuggesterGeoMapping --

// SuggesterGeoMapping provides a mapping for a geolocation context in a suggester.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/suggester-context.html#_geo_location_mapping.
type SuggesterGeoMapping struct {
	name             string
	defaultLocations []*GeoPoint
	precision        []string
	neighbors        *bool
	fieldName        string
}

// NewSuggesterGeoMapping creates a new SuggesterGeoMapping.
func NewSuggesterGeoMapping(name string) *SuggesterGeoMapping {
	return &SuggesterGeoMapping{
		name: name,
	}
}

func (q *SuggesterGeoMapping) DefaultLocations(locations ...*GeoPoint) *SuggesterGeoMapping {
	q.defaultLocations = append(q.defaultLocations, locations...)
	return q
}

func (q *SuggesterGeoMapping) Precision(precision ...string) *SuggesterGeoMapping {
	q.precision = append(q.precision, precision...)
	return q
}

func (q *SuggesterGeoMapping) Neighbors(neighbors bool) *SuggesterGeoMapping {
	q.neighbors = &neighbors
	return q
}

func (q *SuggesterGeoMapping) FieldName(fieldName string) *SuggesterGeoMapping {
	q.fieldName = fieldName
	return q
}

// Source returns a map that will be used to serialize the context query as JSON.
func (q *SuggesterGeoMapping) Source() (interface{}, error) {
	source := make(map[string]interface{})

	x := make(map[string]interface{})
	source[q.name] = x

	x["type"] = "geo"

	if len(q.precision) > 0 {
		x["precision"] = q.precision
	}
	if q.neighbors != nil {
		x["neighbors"] = *q.neighbors
	}

	switch len(q.defaultLocations) {
	case 0:
	case 1:
		x["default"] = q.defaultLocations[0].Source()
	default:
		var arr []interface{}
		for _, p := range q.defaultLocations {
			arr = append(arr, p.Source())
		}
		x["default"] = arr
	}

	if q.fieldName != "" {
		x["path"] = q.fieldName
	}
	return source, nil
}

// -- SuggesterGeoQuery --

// SuggesterGeoQuery provides querying a geolocation context in a suggester.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/suggester-context.html#_geo_location_query
type SuggesterGeoQuery struct {
	name       string
	location   *GeoPoint
	precision  string
	neighbours []string
	boost      *int
}

// NewSuggesterGeoQuery creates a new SuggesterGeoQuery.
func NewSuggesterGeoQuery(name string, location *GeoPoint) *SuggesterGeoQuery {
	return &SuggesterGeoQuery{
		name:       name,
		location:   location,
		neighbours: make([]string, 0),
	}
}

func (q *SuggesterGeoQuery) Precision(precision string) *SuggesterGeoQuery {
	q.precision = precision
	return q
}

func (q *SuggesterGeoQuery) Neighbours(neighbours ...string) *SuggesterGeoQuery {
	q.neighbours = append(q.neighbours, neighbours...)
	return q
}

func (q *SuggesterGeoQuery) Boost(boost int) *SuggesterGeoQuery {
	q.boost = &boost
	return q
}

// Source returns a map that will be used to serialize the context query as JSON.
func (q *SuggesterGeoQuery) Source() (interface{}, error) {
	source := make(map[string]interface{})

	x := make(map[string]interface{})
	source[q.name] = x

	if q.location != nil {
		x["context"] = q.location.Source()
	}
	if q.precision != "" {
		x["precision"] = q.precision
	}
	if q.boost != nil {
		x["boost"] = q.boost
	}
	switch len(q.neighbours) {
	case 0:
	case 1:
		x["neighbours"] = q.neighbours[0]
	default:
		x["neighbours"] = q.neighbours
	}

	return source, nil
}

type SuggesterGeoIndex struct {
	name      string
	locations []*GeoPoint
}

// NewSuggesterGeoQuery creates a new SuggesterGeoQuery.
func NewSuggesterGeoIndex(name string) *SuggesterGeoIndex {
	return &SuggesterGeoIndex{
		name: name,
	}
}

func (q *SuggesterGeoIndex) Locations(locations ...*GeoPoint) *SuggesterGeoIndex {
	q.locations = append(q.locations, locations...)
	return q
}

// Source returns a map that will be used to serialize the context query as JSON.
func (q *SuggesterGeoIndex) Source() (interface{}, error) {
	source := make(map[string]interface{})

	switch len(q.locations) {
	case 0:
		source[q.name] = make([]string, 0)
	case 1:
		source[q.name] = q.locations[0].Source()
	default:
		var arr []interface{}
		for _, p := range q.locations {
			arr = append(arr, p.Source())
		}
		source[q.name] = arr
	}

	return source, nil
}
