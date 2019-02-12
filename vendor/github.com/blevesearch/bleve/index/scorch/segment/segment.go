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

package segment

import (
	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/index"
)

// Overhead from go data structures when deployed on a 64-bit system.
const SizeOfMap uint64 = 8
const SizeOfPointer uint64 = 8
const SizeOfSlice uint64 = 24
const SizeOfString uint64 = 16

// DocumentFieldValueVisitor defines a callback to be visited for each
// stored field value.  The return value determines if the visitor
// should keep going.  Returning true continues visiting, false stops.
type DocumentFieldValueVisitor func(field string, typ byte, value []byte, pos []uint64) bool

type Segment interface {
	Dictionary(field string) (TermDictionary, error)

	VisitDocument(num uint64, visitor DocumentFieldValueVisitor) error
	Count() uint64

	DocNumbers([]string) (*roaring.Bitmap, error)

	Fields() []string

	Close() error

	SizeInBytes() uint64

	AddRef()
	DecRef() error
}

type TermDictionary interface {
	PostingsList(term string, except *roaring.Bitmap) (PostingsList, error)

	Iterator() DictionaryIterator
	PrefixIterator(prefix string) DictionaryIterator
	RangeIterator(start, end string) DictionaryIterator
}

type DictionaryIterator interface {
	Next() (*index.DictEntry, error)
}

type PostingsList interface {
	Iterator() PostingsIterator

	Count() uint64

	// NOTE deferred for future work

	// And(other PostingsList) PostingsList
	// Or(other PostingsList) PostingsList
}

type PostingsIterator interface {
	// The caller is responsible for copying whatever it needs from
	// the returned Posting instance before calling Next(), as some
	// implementations may return a shared instance to reduce memory
	// allocations.
	Next() (Posting, error)
}

type Posting interface {
	Number() uint64

	Frequency() uint64
	Norm() float64

	Locations() []Location
}

type Location interface {
	Field() string
	Start() uint64
	End() uint64
	Pos() uint64
	ArrayPositions() []uint64
}

// DocumentFieldTermVisitable is implemented by various scorch segment
// implementations with persistence for the un inverting of the
// postings or other indexed values.
type DocumentFieldTermVisitable interface {
	VisitDocumentFieldTerms(localDocNum uint64, fields []string,
		visitor index.DocumentFieldTermVisitor) error

	// VisitableDocValueFields implementation should return
	// the list of fields which are document value persisted and
	// therefore visitable by the above VisitDocumentFieldTerms method.
	VisitableDocValueFields() ([]string, error)
}
