//  Copyright (c) 2015 Couchbase, Inc.
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

package index

import "time"

type Document interface {
	ID() string
	Size() int

	VisitFields(visitor FieldVisitor)
	VisitComposite(visitor CompositeFieldVisitor)
	HasComposite() bool

	NumPlainTextBytes() uint64

	AddIDField()
}

type FieldVisitor func(Field)

type Field interface {
	Name() string
	Value() []byte
	ArrayPositions() []uint64

	EncodedFieldType() byte

	Analyze()

	Options() FieldIndexingOptions

	AnalyzedLength() int
	AnalyzedTokenFrequencies() TokenFrequencies

	NumPlainTextBytes() uint64
}

type CompositeFieldVisitor func(field CompositeField)

type CompositeField interface {
	Field

	Compose(field string, length int, freq TokenFrequencies)
}

type TextField interface {
	Text() string
}

type NumericField interface {
	Number() (float64, error)
}

type DateTimeField interface {
	DateTime() (time.Time, error)
}

type BooleanField interface {
	Boolean() (bool, error)
}

type GeoPointField interface {
	Lon() (float64, error)
	Lat() (float64, error)
}
