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

	"github.com/blevesearch/bleve/analysis"
)

const DefaultBooleanIndexingOptions = StoreField | IndexField

type BooleanField struct {
	name              string
	arrayPositions    []uint64
	options           IndexingOptions
	value             []byte
	numPlainTextBytes uint64
}

func (b *BooleanField) Name() string {
	return b.name
}

func (b *BooleanField) ArrayPositions() []uint64 {
	return b.arrayPositions
}

func (b *BooleanField) Options() IndexingOptions {
	return b.options
}

func (b *BooleanField) Analyze() (int, analysis.TokenFrequencies) {
	tokens := make(analysis.TokenStream, 0)
	tokens = append(tokens, &analysis.Token{
		Start:    0,
		End:      len(b.value),
		Term:     b.value,
		Position: 1,
		Type:     analysis.Boolean,
	})

	fieldLength := len(tokens)
	tokenFreqs := analysis.TokenFrequency(tokens, b.arrayPositions, b.options.IncludeTermVectors())
	return fieldLength, tokenFreqs
}

func (b *BooleanField) Value() []byte {
	return b.value
}

func (b *BooleanField) Boolean() (bool, error) {
	if len(b.value) == 1 {
		return b.value[0] == 'T', nil
	}
	return false, fmt.Errorf("boolean field has %d bytes", len(b.value))
}

func (b *BooleanField) GoString() string {
	return fmt.Sprintf("&document.BooleanField{Name:%s, Options: %s, Value: %s}", b.name, b.options, b.value)
}

func (b *BooleanField) NumPlainTextBytes() uint64 {
	return b.numPlainTextBytes
}

func NewBooleanFieldFromBytes(name string, arrayPositions []uint64, value []byte) *BooleanField {
	return &BooleanField{
		name:              name,
		arrayPositions:    arrayPositions,
		value:             value,
		options:           DefaultNumericIndexingOptions,
		numPlainTextBytes: uint64(len(value)),
	}
}

func NewBooleanField(name string, arrayPositions []uint64, b bool) *BooleanField {
	return NewBooleanFieldWithIndexingOptions(name, arrayPositions, b, DefaultNumericIndexingOptions)
}

func NewBooleanFieldWithIndexingOptions(name string, arrayPositions []uint64, b bool, options IndexingOptions) *BooleanField {
	numPlainTextBytes := 5
	v := []byte("F")
	if b {
		numPlainTextBytes = 4
		v = []byte("T")
	}
	return &BooleanField{
		name:              name,
		arrayPositions:    arrayPositions,
		value:             v,
		options:           options,
		numPlainTextBytes: uint64(numPlainTextBytes),
	}
}
