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
	"github.com/blevesearch/bleve/numeric"
)

const DefaultNumericIndexingOptions = StoreField | IndexField

const DefaultPrecisionStep uint = 4

type NumericField struct {
	name              string
	arrayPositions    []uint64
	options           IndexingOptions
	value             numeric.PrefixCoded
	numPlainTextBytes uint64
}

func (n *NumericField) Name() string {
	return n.name
}

func (n *NumericField) ArrayPositions() []uint64 {
	return n.arrayPositions
}

func (n *NumericField) Options() IndexingOptions {
	return n.options
}

func (n *NumericField) Analyze() (int, analysis.TokenFrequencies) {
	tokens := make(analysis.TokenStream, 0)
	tokens = append(tokens, &analysis.Token{
		Start:    0,
		End:      len(n.value),
		Term:     n.value,
		Position: 1,
		Type:     analysis.Numeric,
	})

	original, err := n.value.Int64()
	if err == nil {

		shift := DefaultPrecisionStep
		for shift < 64 {
			shiftEncoded, err := numeric.NewPrefixCodedInt64(original, shift)
			if err != nil {
				break
			}
			token := analysis.Token{
				Start:    0,
				End:      len(shiftEncoded),
				Term:     shiftEncoded,
				Position: 1,
				Type:     analysis.Numeric,
			}
			tokens = append(tokens, &token)
			shift += DefaultPrecisionStep
		}
	}

	fieldLength := len(tokens)
	tokenFreqs := analysis.TokenFrequency(tokens, n.arrayPositions, n.options.IncludeTermVectors())
	return fieldLength, tokenFreqs
}

func (n *NumericField) Value() []byte {
	return n.value
}

func (n *NumericField) Number() (float64, error) {
	i64, err := n.value.Int64()
	if err != nil {
		return 0.0, err
	}
	return numeric.Int64ToFloat64(i64), nil
}

func (n *NumericField) GoString() string {
	return fmt.Sprintf("&document.NumericField{Name:%s, Options: %s, Value: %s}", n.name, n.options, n.value)
}

func (n *NumericField) NumPlainTextBytes() uint64 {
	return n.numPlainTextBytes
}

func NewNumericFieldFromBytes(name string, arrayPositions []uint64, value []byte) *NumericField {
	return &NumericField{
		name:              name,
		arrayPositions:    arrayPositions,
		value:             value,
		options:           DefaultNumericIndexingOptions,
		numPlainTextBytes: uint64(len(value)),
	}
}

func NewNumericField(name string, arrayPositions []uint64, number float64) *NumericField {
	return NewNumericFieldWithIndexingOptions(name, arrayPositions, number, DefaultNumericIndexingOptions)
}

func NewNumericFieldWithIndexingOptions(name string, arrayPositions []uint64, number float64, options IndexingOptions) *NumericField {
	numberInt64 := numeric.Float64ToInt64(number)
	prefixCoded := numeric.MustNewPrefixCodedInt64(numberInt64, 0)
	return &NumericField{
		name:           name,
		arrayPositions: arrayPositions,
		value:          prefixCoded,
		options:        options,
		// not correct, just a place holder until we revisit how fields are
		// represented and can fix this better
		numPlainTextBytes: uint64(8),
	}
}
