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
	"fmt"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/index"
	"github.com/couchbase/vellum"
)

var ErrClosed = fmt.Errorf("index closed")

// DocumentFieldValueVisitor defines a callback to be visited for each
// stored field value.  The return value determines if the visitor
// should keep going.  Returning true continues visiting, false stops.
type DocumentFieldValueVisitor func(field string, typ byte, value []byte, pos []uint64) bool

type Segment interface {
	Dictionary(field string) (TermDictionary, error)

	VisitDocument(num uint64, visitor DocumentFieldValueVisitor) error

	DocID(num uint64) ([]byte, error)

	Count() uint64

	DocNumbers([]string) (*roaring.Bitmap, error)

	Fields() []string

	Close() error

	Size() int

	AddRef()
	DecRef() error
}

type UnpersistedSegment interface {
	Segment
	Persist(path string) error
}

type PersistedSegment interface {
	Segment
	Path() string
}

type TermDictionary interface {
	PostingsList(term []byte, except *roaring.Bitmap, prealloc PostingsList) (PostingsList, error)

	Iterator() DictionaryIterator
	PrefixIterator(prefix string) DictionaryIterator
	RangeIterator(start, end string) DictionaryIterator
	AutomatonIterator(a vellum.Automaton,
		startKeyInclusive, endKeyExclusive []byte) DictionaryIterator
	OnlyIterator(onlyTerms [][]byte, includeCount bool) DictionaryIterator

	Contains(key []byte) (bool, error)
}

type DictionaryIterator interface {
	Next() (*index.DictEntry, error)
}

type PostingsList interface {
	Iterator(includeFreq, includeNorm, includeLocations bool, prealloc PostingsIterator) PostingsIterator

	Size() int

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

	// Advance will return the posting with the specified doc number
	// or if there is no such posting, the next posting.
	// Callers MUST NOT attempt to pass a docNum that is less than or
	// equal to the currently visited posting doc Num.
	Advance(docNum uint64) (Posting, error)

	Size() int
}

type OptimizablePostingsIterator interface {
	ActualBitmap() *roaring.Bitmap
	DocNum1Hit() (uint64, bool)
	ReplaceActual(*roaring.Bitmap)
}

type Posting interface {
	Number() uint64

	Frequency() uint64
	Norm() float64

	Locations() []Location

	Size() int
}

type Location interface {
	Field() string
	Start() uint64
	End() uint64
	Pos() uint64
	ArrayPositions() []uint64
	Size() int
}

// DocumentFieldTermVisitable is implemented by various scorch segment
// implementations with persistence for the un inverting of the
// postings or other indexed values.
type DocumentFieldTermVisitable interface {
	VisitDocumentFieldTerms(localDocNum uint64, fields []string,
		visitor index.DocumentFieldTermVisitor, optional DocVisitState) (DocVisitState, error)

	// VisitableDocValueFields implementation should return
	// the list of fields which are document value persisted and
	// therefore visitable by the above VisitDocumentFieldTerms method.
	VisitableDocValueFields() ([]string, error)
}

type DocVisitState interface {
}

type StatsReporter interface {
	ReportBytesWritten(bytesWritten uint64)
}
