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

package document

import (
	"fmt"
	"reflect"

	"github.com/blevesearch/bleve/size"
)

var reflectStaticSizeDocument int

func init() {
	var d Document
	reflectStaticSizeDocument = int(reflect.TypeOf(d).Size())
}

type Document struct {
	ID              string  `json:"id"`
	Fields          []Field `json:"fields"`
	CompositeFields []*CompositeField
}

func NewDocument(id string) *Document {
	return &Document{
		ID:              id,
		Fields:          make([]Field, 0),
		CompositeFields: make([]*CompositeField, 0),
	}
}

func (d *Document) Size() int {
	sizeInBytes := reflectStaticSizeDocument + size.SizeOfPtr +
		len(d.ID)

	for _, entry := range d.Fields {
		sizeInBytes += entry.Size()
	}

	for _, entry := range d.CompositeFields {
		sizeInBytes += entry.Size()
	}

	return sizeInBytes
}

func (d *Document) AddField(f Field) *Document {
	switch f := f.(type) {
	case *CompositeField:
		d.CompositeFields = append(d.CompositeFields, f)
	default:
		d.Fields = append(d.Fields, f)
	}
	return d
}

func (d *Document) GoString() string {
	fields := ""
	for i, field := range d.Fields {
		if i != 0 {
			fields += ", "
		}
		fields += fmt.Sprintf("%#v", field)
	}
	compositeFields := ""
	for i, field := range d.CompositeFields {
		if i != 0 {
			compositeFields += ", "
		}
		compositeFields += fmt.Sprintf("%#v", field)
	}
	return fmt.Sprintf("&document.Document{ID:%s, Fields: %s, CompositeFields: %s}", d.ID, fields, compositeFields)
}

func (d *Document) NumPlainTextBytes() uint64 {
	rv := uint64(0)
	for _, field := range d.Fields {
		rv += field.NumPlainTextBytes()
	}
	for _, compositeField := range d.CompositeFields {
		for _, field := range d.Fields {
			if compositeField.includesField(field.Name()) {
				rv += field.NumPlainTextBytes()
			}
		}
	}
	return rv
}
