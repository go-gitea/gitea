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

package geo

import (
	"reflect"
	"strings"
)

// ExtractGeoPoint takes an arbitrary interface{} and tries it's best to
// interpret it is as geo point.  Supported formats:
// Container:
// slice length 2 (GeoJSON)
//  first element lon, second element lat
// map[string]interface{}
//  exact keys lat and lon or lng
// struct
//  w/exported fields case-insensitive match on lat and lon or lng
// struct
//  satisfying Later and Loner or Lnger interfaces
//
// in all cases values must be some sort of numeric-like thing: int/uint/float
func ExtractGeoPoint(thing interface{}) (lon, lat float64, success bool) {
	var foundLon, foundLat bool

	thingVal := reflect.ValueOf(thing)
	thingTyp := thingVal.Type()

	// is it a slice
	if thingVal.IsValid() && thingVal.Kind() == reflect.Slice {
		// must be length 2
		if thingVal.Len() == 2 {
			first := thingVal.Index(0)
			if first.CanInterface() {
				firstVal := first.Interface()
				lon, foundLon = extractNumericVal(firstVal)
			}
			second := thingVal.Index(1)
			if second.CanInterface() {
				secondVal := second.Interface()
				lat, foundLat = extractNumericVal(secondVal)
			}
		}
	}

	// is it a map
	if l, ok := thing.(map[string]interface{}); ok {
		if lval, ok := l["lon"]; ok {
			lon, foundLon = extractNumericVal(lval)
		} else if lval, ok := l["lng"]; ok {
			lon, foundLon = extractNumericVal(lval)
		}
		if lval, ok := l["lat"]; ok {
			lat, foundLat = extractNumericVal(lval)
		}
	}

	// now try reflection on struct fields
	if thingVal.IsValid() && thingVal.Kind() == reflect.Struct {
		for i := 0; i < thingVal.NumField(); i++ {
			fieldName := thingTyp.Field(i).Name
			if strings.HasPrefix(strings.ToLower(fieldName), "lon") {
				if thingVal.Field(i).CanInterface() {
					fieldVal := thingVal.Field(i).Interface()
					lon, foundLon = extractNumericVal(fieldVal)
				}
			}
			if strings.HasPrefix(strings.ToLower(fieldName), "lng") {
				if thingVal.Field(i).CanInterface() {
					fieldVal := thingVal.Field(i).Interface()
					lon, foundLon = extractNumericVal(fieldVal)
				}
			}
			if strings.HasPrefix(strings.ToLower(fieldName), "lat") {
				if thingVal.Field(i).CanInterface() {
					fieldVal := thingVal.Field(i).Interface()
					lat, foundLat = extractNumericVal(fieldVal)
				}
			}
		}
	}

	// last hope, some interfaces
	// lon
	if l, ok := thing.(loner); ok {
		lon = l.Lon()
		foundLon = true
	} else if l, ok := thing.(lnger); ok {
		lon = l.Lng()
		foundLon = true
	}
	// lat
	if l, ok := thing.(later); ok {
		lat = l.Lat()
		foundLat = true
	}

	return lon, lat, foundLon && foundLat
}

// extract numeric value (if possible) and returns a float64
func extractNumericVal(v interface{}) (float64, bool) {
	val := reflect.ValueOf(v)
	typ := val.Type()
	switch typ.Kind() {
	case reflect.Float32, reflect.Float64:
		return val.Float(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(val.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(val.Uint()), true
	}

	return 0, false
}

// various support interfaces which can be used to find lat/lon
type loner interface {
	Lon() float64
}

type later interface {
	Lat() float64
}

type lnger interface {
	Lng() float64
}
