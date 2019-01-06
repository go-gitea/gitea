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

package index

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/blevesearch/bleve/document"
	"github.com/blevesearch/bleve/index/store"
)

var ErrorUnknownStorageType = fmt.Errorf("unknown storage type")

type Index interface {
	Open() error
	Close() error

	Update(doc *document.Document) error
	Delete(id string) error
	Batch(batch *Batch) error

	SetInternal(key, val []byte) error
	DeleteInternal(key []byte) error

	// Reader returns a low-level accessor on the index data. Close it to
	// release associated resources.
	Reader() (IndexReader, error)

	Stats() json.Marshaler
	StatsMap() map[string]interface{}

	Analyze(d *document.Document) *AnalysisResult

	Advanced() (store.KVStore, error)
}

type DocumentFieldTermVisitor func(field string, term []byte)

type IndexReader interface {
	TermFieldReader(term []byte, field string, includeFreq, includeNorm, includeTermVectors bool) (TermFieldReader, error)

	// DocIDReader returns an iterator over all doc ids
	// The caller must close returned instance to release associated resources.
	DocIDReaderAll() (DocIDReader, error)

	DocIDReaderOnly(ids []string) (DocIDReader, error)

	FieldDict(field string) (FieldDict, error)

	// FieldDictRange is currently defined to include the start and end terms
	FieldDictRange(field string, startTerm []byte, endTerm []byte) (FieldDict, error)
	FieldDictPrefix(field string, termPrefix []byte) (FieldDict, error)

	Document(id string) (*document.Document, error)
	DocumentVisitFieldTerms(id IndexInternalID, fields []string, visitor DocumentFieldTermVisitor) error

	Fields() ([]string, error)

	GetInternal(key []byte) ([]byte, error)

	DocCount() (uint64, error)

	ExternalID(id IndexInternalID) (string, error)
	InternalID(id string) (IndexInternalID, error)

	DumpAll() chan interface{}
	DumpDoc(id string) chan interface{}
	DumpFields() chan interface{}

	Close() error
}

// FieldTerms contains the terms used by a document, keyed by field
type FieldTerms map[string][]string

// FieldsNotYetCached returns a list of fields not yet cached out of a larger list of fields
func (f FieldTerms) FieldsNotYetCached(fields []string) []string {
	rv := make([]string, 0, len(fields))
	for _, field := range fields {
		if _, ok := f[field]; !ok {
			rv = append(rv, field)
		}
	}
	return rv
}

// Merge will combine two FieldTerms
// it assumes that the terms lists are complete (thus do not need to be merged)
// field terms from the other list always replace the ones in the receiver
func (f FieldTerms) Merge(other FieldTerms) {
	for field, terms := range other {
		f[field] = terms
	}
}

type TermFieldVector struct {
	Field          string
	ArrayPositions []uint64
	Pos            uint64
	Start          uint64
	End            uint64
}

// IndexInternalID is an opaque document identifier interal to the index impl
type IndexInternalID []byte

func (id IndexInternalID) Equals(other IndexInternalID) bool {
	return id.Compare(other) == 0
}

func (id IndexInternalID) Compare(other IndexInternalID) int {
	return bytes.Compare(id, other)
}

type TermFieldDoc struct {
	Term    string
	ID      IndexInternalID
	Freq    uint64
	Norm    float64
	Vectors []*TermFieldVector
}

// Reset allows an already allocated TermFieldDoc to be reused
func (tfd *TermFieldDoc) Reset() *TermFieldDoc {
	// remember the []byte used for the ID
	id := tfd.ID
	// idiom to copy over from empty TermFieldDoc (0 allocations)
	*tfd = TermFieldDoc{}
	// reuse the []byte already allocated (and reset len to 0)
	tfd.ID = id[:0]
	return tfd
}

// TermFieldReader is the interface exposing the enumeration of documents
// containing a given term in a given field. Documents are returned in byte
// lexicographic order over their identifiers.
type TermFieldReader interface {
	// Next returns the next document containing the term in this field, or nil
	// when it reaches the end of the enumeration.  The preAlloced TermFieldDoc
	// is optional, and when non-nil, will be used instead of allocating memory.
	Next(preAlloced *TermFieldDoc) (*TermFieldDoc, error)

	// Advance resets the enumeration at specified document or its immediate
	// follower.
	Advance(ID IndexInternalID, preAlloced *TermFieldDoc) (*TermFieldDoc, error)

	// Count returns the number of documents contains the term in this field.
	Count() uint64
	Close() error
}

type DictEntry struct {
	Term  string
	Count uint64
}

type FieldDict interface {
	Next() (*DictEntry, error)
	Close() error
}

// DocIDReader is the interface exposing enumeration of documents identifiers.
// Close the reader to release associated resources.
type DocIDReader interface {
	// Next returns the next document internal identifier in the natural
	// index order, nil when the end of the sequence is reached.
	Next() (IndexInternalID, error)

	// Advance resets the iteration to the first internal identifier greater than
	// or equal to ID. If ID is smaller than the start of the range, the iteration
	// will start there instead. If ID is greater than or equal to the end of
	// the range, Next() call will return io.EOF.
	Advance(ID IndexInternalID) (IndexInternalID, error)
	Close() error
}

type Batch struct {
	IndexOps    map[string]*document.Document
	InternalOps map[string][]byte
}

func NewBatch() *Batch {
	return &Batch{
		IndexOps:    make(map[string]*document.Document),
		InternalOps: make(map[string][]byte),
	}
}

func (b *Batch) Update(doc *document.Document) {
	b.IndexOps[doc.ID] = doc
}

func (b *Batch) Delete(id string) {
	b.IndexOps[id] = nil
}

func (b *Batch) SetInternal(key, val []byte) {
	b.InternalOps[string(key)] = val
}

func (b *Batch) DeleteInternal(key []byte) {
	b.InternalOps[string(key)] = nil
}

func (b *Batch) String() string {
	rv := fmt.Sprintf("Batch (%d ops, %d internal ops)\n", len(b.IndexOps), len(b.InternalOps))
	for k, v := range b.IndexOps {
		if v != nil {
			rv += fmt.Sprintf("\tINDEX - '%s'\n", k)
		} else {
			rv += fmt.Sprintf("\tDELETE - '%s'\n", k)
		}
	}
	for k, v := range b.InternalOps {
		if v != nil {
			rv += fmt.Sprintf("\tSET INTERNAL - '%s'\n", k)
		} else {
			rv += fmt.Sprintf("\tDELETE INTERNAL - '%s'\n", k)
		}
	}
	return rv
}

func (b *Batch) Reset() {
	b.IndexOps = make(map[string]*document.Document)
	b.InternalOps = make(map[string][]byte)
}
