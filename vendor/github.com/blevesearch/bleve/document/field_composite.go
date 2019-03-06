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
	"reflect"

	"github.com/blevesearch/bleve/analysis"
	"github.com/blevesearch/bleve/size"
)

var reflectStaticSizeCompositeField int

func init() {
	var cf CompositeField
	reflectStaticSizeCompositeField = int(reflect.TypeOf(cf).Size())
}

const DefaultCompositeIndexingOptions = IndexField

type CompositeField struct {
	name                 string
	includedFields       map[string]bool
	excludedFields       map[string]bool
	defaultInclude       bool
	options              IndexingOptions
	totalLength          int
	compositeFrequencies analysis.TokenFrequencies
}

func NewCompositeField(name string, defaultInclude bool, include []string, exclude []string) *CompositeField {
	return NewCompositeFieldWithIndexingOptions(name, defaultInclude, include, exclude, DefaultCompositeIndexingOptions)
}

func NewCompositeFieldWithIndexingOptions(name string, defaultInclude bool, include []string, exclude []string, options IndexingOptions) *CompositeField {
	rv := &CompositeField{
		name:                 name,
		options:              options,
		defaultInclude:       defaultInclude,
		includedFields:       make(map[string]bool, len(include)),
		excludedFields:       make(map[string]bool, len(exclude)),
		compositeFrequencies: make(analysis.TokenFrequencies),
	}

	for _, i := range include {
		rv.includedFields[i] = true
	}
	for _, e := range exclude {
		rv.excludedFields[e] = true
	}

	return rv
}

func (c *CompositeField) Size() int {
	sizeInBytes := reflectStaticSizeCompositeField + size.SizeOfPtr +
		len(c.name)

	for k, _ := range c.includedFields {
		sizeInBytes += size.SizeOfString + len(k) + size.SizeOfBool
	}

	for k, _ := range c.excludedFields {
		sizeInBytes += size.SizeOfString + len(k) + size.SizeOfBool
	}

	return sizeInBytes
}

func (c *CompositeField) Name() string {
	return c.name
}

func (c *CompositeField) ArrayPositions() []uint64 {
	return []uint64{}
}

func (c *CompositeField) Options() IndexingOptions {
	return c.options
}

func (c *CompositeField) Analyze() (int, analysis.TokenFrequencies) {
	return c.totalLength, c.compositeFrequencies
}

func (c *CompositeField) Value() []byte {
	return []byte{}
}

func (c *CompositeField) NumPlainTextBytes() uint64 {
	return 0
}

func (c *CompositeField) includesField(field string) bool {
	shouldInclude := c.defaultInclude
	_, fieldShouldBeIncluded := c.includedFields[field]
	if fieldShouldBeIncluded {
		shouldInclude = true
	}
	_, fieldShouldBeExcluded := c.excludedFields[field]
	if fieldShouldBeExcluded {
		shouldInclude = false
	}
	return shouldInclude
}

func (c *CompositeField) Compose(field string, length int, freq analysis.TokenFrequencies) {
	if c.includesField(field) {
		c.totalLength += length
		c.compositeFrequencies.MergeAll(field, freq)
	}
}
