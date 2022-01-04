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
	"reflect"
)

var reflectStaticSizeTermFieldDoc int
var reflectStaticSizeTermFieldVector int

func init() {
	var tfd TermFieldDoc
	reflectStaticSizeTermFieldDoc = int(reflect.TypeOf(tfd).Size())
	var tfv TermFieldVector
	reflectStaticSizeTermFieldVector = int(reflect.TypeOf(tfv).Size())
}

type Index interface {
	Open() error
	Close() error

	Update(doc Document) error
	Delete(id string) error
	Batch(batch *Batch) error

	SetInternal(key, val []byte) error
	DeleteInternal(key []byte) error

	// Reader returns a low-level accessor on the index data. Close it to
	// release associated resources.
	Reader() (IndexReader, error)

	StatsMap() map[string]interface{}
}

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

	Document(id string) (Document, error)

	DocValueReader(fields []string) (DocValueReader, error)

	Fields() ([]string, error)

	GetInternal(key []byte) ([]byte, error)

	DocCount() (uint64, error)

	ExternalID(id IndexInternalID) (string, error)
	InternalID(id string) (IndexInternalID, error)

	Close() error
}

type IndexReaderRegexp interface {
	FieldDictRegexp(field string, regex string) (FieldDict, error)
}

type IndexReaderFuzzy interface {
	FieldDictFuzzy(field string, term string, fuzziness int, prefix string) (FieldDict, error)
}

type IndexReaderContains interface {
	FieldDictContains(field string) (FieldDictContains, error)
}

type TermFieldVector struct {
	Field          string
	ArrayPositions []uint64
	Pos            uint64
	Start          uint64
	End            uint64
}

func (tfv *TermFieldVector) Size() int {
	return reflectStaticSizeTermFieldVector + sizeOfPtr +
		len(tfv.Field) + len(tfv.ArrayPositions)*sizeOfUint64
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

func (tfd *TermFieldDoc) Size() int {
	sizeInBytes := reflectStaticSizeTermFieldDoc + sizeOfPtr +
		len(tfd.Term) + len(tfd.ID)

	for _, entry := range tfd.Vectors {
		sizeInBytes += entry.Size()
	}

	return sizeInBytes
}

// Reset allows an already allocated TermFieldDoc to be reused
func (tfd *TermFieldDoc) Reset() *TermFieldDoc {
	// remember the []byte used for the ID
	id := tfd.ID
	vectors := tfd.Vectors
	// idiom to copy over from empty TermFieldDoc (0 allocations)
	*tfd = TermFieldDoc{}
	// reuse the []byte already allocated (and reset len to 0)
	tfd.ID = id[:0]
	tfd.Vectors = vectors[:0]
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

	Size() int
}

type DictEntry struct {
	Term  string
	Count uint64
}

type FieldDict interface {
	Next() (*DictEntry, error)
	Close() error
}

type FieldDictContains interface {
	Contains(key []byte) (bool, error)
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

	Size() int

	Close() error
}

type DocValueVisitor func(field string, term []byte)

type DocValueReader interface {
	VisitDocValues(id IndexInternalID, visitor DocValueVisitor) error
}

// IndexBuilder is an interface supported by some index schemes
// to allow direct write-only index building
type IndexBuilder interface {
	Index(doc Document) error
	Close() error
}
