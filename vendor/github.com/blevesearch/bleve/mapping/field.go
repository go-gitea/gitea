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

package mapping

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/blevesearch/bleve/analysis"
	"github.com/blevesearch/bleve/document"
	"github.com/blevesearch/bleve/geo"
)

// control the default behavior for dynamic fields (those not explicitly mapped)
var (
	IndexDynamic = true
	StoreDynamic = true
)

// A FieldMapping describes how a specific item
// should be put into the index.
type FieldMapping struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`

	// Analyzer specifies the name of the analyzer to use for this field.  If
	// Analyzer is empty, traverse the DocumentMapping tree toward the root and
	// pick the first non-empty DefaultAnalyzer found. If there is none, use
	// the IndexMapping.DefaultAnalyzer.
	Analyzer string `json:"analyzer,omitempty"`

	// Store indicates whether to store field values in the index. Stored
	// values can be retrieved from search results using SearchRequest.Fields.
	Store bool `json:"store,omitempty"`
	Index bool `json:"index,omitempty"`

	// IncludeTermVectors, if true, makes terms occurrences to be recorded for
	// this field. It includes the term position within the terms sequence and
	// the term offsets in the source document field. Term vectors are required
	// to perform phrase queries or terms highlighting in source documents.
	IncludeTermVectors bool   `json:"include_term_vectors,omitempty"`
	IncludeInAll       bool   `json:"include_in_all,omitempty"`
	DateFormat         string `json:"date_format,omitempty"`
}

// NewTextFieldMapping returns a default field mapping for text
func NewTextFieldMapping() *FieldMapping {
	return &FieldMapping{
		Type:               "text",
		Store:              true,
		Index:              true,
		IncludeTermVectors: true,
		IncludeInAll:       true,
	}
}

func newTextFieldMappingDynamic(im *IndexMappingImpl) *FieldMapping {
	rv := NewTextFieldMapping()
	rv.Store = im.StoreDynamic
	rv.Index = im.IndexDynamic
	return rv
}

// NewNumericFieldMapping returns a default field mapping for numbers
func NewNumericFieldMapping() *FieldMapping {
	return &FieldMapping{
		Type:         "number",
		Store:        true,
		Index:        true,
		IncludeInAll: true,
	}
}

func newNumericFieldMappingDynamic(im *IndexMappingImpl) *FieldMapping {
	rv := NewNumericFieldMapping()
	rv.Store = im.StoreDynamic
	rv.Index = im.IndexDynamic
	return rv
}

// NewDateTimeFieldMapping returns a default field mapping for dates
func NewDateTimeFieldMapping() *FieldMapping {
	return &FieldMapping{
		Type:         "datetime",
		Store:        true,
		Index:        true,
		IncludeInAll: true,
	}
}

func newDateTimeFieldMappingDynamic(im *IndexMappingImpl) *FieldMapping {
	rv := NewDateTimeFieldMapping()
	rv.Store = im.StoreDynamic
	rv.Index = im.IndexDynamic
	return rv
}

// NewBooleanFieldMapping returns a default field mapping for booleans
func NewBooleanFieldMapping() *FieldMapping {
	return &FieldMapping{
		Type:         "boolean",
		Store:        true,
		Index:        true,
		IncludeInAll: true,
	}
}

func newBooleanFieldMappingDynamic(im *IndexMappingImpl) *FieldMapping {
	rv := NewBooleanFieldMapping()
	rv.Store = im.StoreDynamic
	rv.Index = im.IndexDynamic
	return rv
}

// NewGeoPointFieldMapping returns a default field mapping for geo points
func NewGeoPointFieldMapping() *FieldMapping {
	return &FieldMapping{
		Type:         "geopoint",
		Store:        true,
		Index:        true,
		IncludeInAll: true,
	}
}

// Options returns the indexing options for this field.
func (fm *FieldMapping) Options() document.IndexingOptions {
	var rv document.IndexingOptions
	if fm.Store {
		rv |= document.StoreField
	}
	if fm.Index {
		rv |= document.IndexField
	}
	if fm.IncludeTermVectors {
		rv |= document.IncludeTermVectors
	}
	return rv
}

func (fm *FieldMapping) processString(propertyValueString string, pathString string, path []string, indexes []uint64, context *walkContext) {
	fieldName := getFieldName(pathString, path, fm)
	options := fm.Options()
	if fm.Type == "text" {
		analyzer := fm.analyzerForField(path, context)
		field := document.NewTextFieldCustom(fieldName, indexes, []byte(propertyValueString), options, analyzer)
		context.doc.AddField(field)

		if !fm.IncludeInAll {
			context.excludedFromAll = append(context.excludedFromAll, fieldName)
		}
	} else if fm.Type == "datetime" {
		dateTimeFormat := context.im.DefaultDateTimeParser
		if fm.DateFormat != "" {
			dateTimeFormat = fm.DateFormat
		}
		dateTimeParser := context.im.DateTimeParserNamed(dateTimeFormat)
		if dateTimeParser != nil {
			parsedDateTime, err := dateTimeParser.ParseDateTime(propertyValueString)
			if err == nil {
				fm.processTime(parsedDateTime, pathString, path, indexes, context)
			}
		}
	}
}

func (fm *FieldMapping) processFloat64(propertyValFloat float64, pathString string, path []string, indexes []uint64, context *walkContext) {
	fieldName := getFieldName(pathString, path, fm)
	if fm.Type == "number" {
		options := fm.Options()
		field := document.NewNumericFieldWithIndexingOptions(fieldName, indexes, propertyValFloat, options)
		context.doc.AddField(field)

		if !fm.IncludeInAll {
			context.excludedFromAll = append(context.excludedFromAll, fieldName)
		}
	}
}

func (fm *FieldMapping) processTime(propertyValueTime time.Time, pathString string, path []string, indexes []uint64, context *walkContext) {
	fieldName := getFieldName(pathString, path, fm)
	if fm.Type == "datetime" {
		options := fm.Options()
		field, err := document.NewDateTimeFieldWithIndexingOptions(fieldName, indexes, propertyValueTime, options)
		if err == nil {
			context.doc.AddField(field)
		} else {
			logger.Printf("could not build date %v", err)
		}

		if !fm.IncludeInAll {
			context.excludedFromAll = append(context.excludedFromAll, fieldName)
		}
	}
}

func (fm *FieldMapping) processBoolean(propertyValueBool bool, pathString string, path []string, indexes []uint64, context *walkContext) {
	fieldName := getFieldName(pathString, path, fm)
	if fm.Type == "boolean" {
		options := fm.Options()
		field := document.NewBooleanFieldWithIndexingOptions(fieldName, indexes, propertyValueBool, options)
		context.doc.AddField(field)

		if !fm.IncludeInAll {
			context.excludedFromAll = append(context.excludedFromAll, fieldName)
		}
	}
}

func (fm *FieldMapping) processGeoPoint(propertyMightBeGeoPoint interface{}, pathString string, path []string, indexes []uint64, context *walkContext) {
	lon, lat, found := geo.ExtractGeoPoint(propertyMightBeGeoPoint)
	if found {
		fieldName := getFieldName(pathString, path, fm)
		options := fm.Options()
		field := document.NewGeoPointFieldWithIndexingOptions(fieldName, indexes, lon, lat, options)
		context.doc.AddField(field)

		if !fm.IncludeInAll {
			context.excludedFromAll = append(context.excludedFromAll, fieldName)
		}
	}
}

func (fm *FieldMapping) analyzerForField(path []string, context *walkContext) *analysis.Analyzer {
	analyzerName := fm.Analyzer
	if analyzerName == "" {
		analyzerName = context.dm.defaultAnalyzerName(path)
		if analyzerName == "" {
			analyzerName = context.im.DefaultAnalyzer
		}
	}
	return context.im.AnalyzerNamed(analyzerName)
}

func getFieldName(pathString string, path []string, fieldMapping *FieldMapping) string {
	fieldName := pathString
	if fieldMapping.Name != "" {
		parentName := ""
		if len(path) > 1 {
			parentName = encodePath(path[:len(path)-1]) + pathSeparator
		}
		fieldName = parentName + fieldMapping.Name
	}
	return fieldName
}

// UnmarshalJSON offers custom unmarshaling with optional strict validation
func (fm *FieldMapping) UnmarshalJSON(data []byte) error {

	var tmp map[string]json.RawMessage
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	var invalidKeys []string
	for k, v := range tmp {
		switch k {
		case "name":
			err := json.Unmarshal(v, &fm.Name)
			if err != nil {
				return err
			}
		case "type":
			err := json.Unmarshal(v, &fm.Type)
			if err != nil {
				return err
			}
		case "analyzer":
			err := json.Unmarshal(v, &fm.Analyzer)
			if err != nil {
				return err
			}
		case "store":
			err := json.Unmarshal(v, &fm.Store)
			if err != nil {
				return err
			}
		case "index":
			err := json.Unmarshal(v, &fm.Index)
			if err != nil {
				return err
			}
		case "include_term_vectors":
			err := json.Unmarshal(v, &fm.IncludeTermVectors)
			if err != nil {
				return err
			}
		case "include_in_all":
			err := json.Unmarshal(v, &fm.IncludeInAll)
			if err != nil {
				return err
			}
		case "date_format":
			err := json.Unmarshal(v, &fm.DateFormat)
			if err != nil {
				return err
			}
		default:
			invalidKeys = append(invalidKeys, k)
		}
	}

	if MappingJSONStrict && len(invalidKeys) > 0 {
		return fmt.Errorf("field mapping contains invalid keys: %v", invalidKeys)
	}

	return nil
}
