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

package zap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/RoaringBitmap/roaring"
	"github.com/Smerity/govarint"
	"github.com/blevesearch/bleve/index/scorch/segment"
	"github.com/couchbase/vellum"
	mmap "github.com/edsrzf/mmap-go"
	"github.com/golang/snappy"
)

// Open returns a zap impl of a segment
func Open(path string) (segment.Segment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	mm, err := mmap.Map(f, mmap.RDONLY, 0)
	if err != nil {
		// mmap failed, try to close the file
		_ = f.Close()
		return nil, err
	}

	rv := &Segment{
		SegmentBase: SegmentBase{
			mem:            mm[0 : len(mm)-FooterSize],
			fieldsMap:      make(map[string]uint16),
			fieldDvIterMap: make(map[uint16]*docValueIterator),
		},
		f:    f,
		mm:   mm,
		path: path,
		refs: 1,
	}

	err = rv.loadConfig()
	if err != nil {
		_ = rv.Close()
		return nil, err
	}

	err = rv.loadFields()
	if err != nil {
		_ = rv.Close()
		return nil, err
	}

	err = rv.loadDvIterators()
	if err != nil {
		_ = rv.Close()
		return nil, err
	}

	return rv, nil
}

// SegmentBase is a memory only, read-only implementation of the
// segment.Segment interface, using zap's data representation.
type SegmentBase struct {
	mem               []byte
	memCRC            uint32
	chunkFactor       uint32
	fieldsMap         map[string]uint16 // fieldName -> fieldID+1
	fieldsInv         []string          // fieldID -> fieldName
	numDocs           uint64
	storedIndexOffset uint64
	fieldsIndexOffset uint64
	docValueOffset    uint64
	dictLocs          []uint64
	fieldDvIterMap    map[uint16]*docValueIterator // naive chunk cache per field
}

func (sb *SegmentBase) AddRef()             {}
func (sb *SegmentBase) DecRef() (err error) { return nil }
func (sb *SegmentBase) Close() (err error)  { return nil }

// Segment implements a persisted segment.Segment interface, by
// embedding an mmap()'ed SegmentBase.
type Segment struct {
	SegmentBase

	f       *os.File
	mm      mmap.MMap
	path    string
	version uint32
	crc     uint32

	m    sync.Mutex // Protects the fields that follow.
	refs int64
}

func (s *Segment) SizeInBytes() uint64 {
	// 8 /* size of file pointer */
	// 4 /* size of version -> uint32 */
	// 4 /* size of crc -> uint32 */
	sizeOfUints := 16

	sizeInBytes := (len(s.path) + int(segment.SizeOfString)) + sizeOfUints

	// mutex, refs -> int64
	sizeInBytes += 16

	// do not include the mmap'ed part
	return uint64(sizeInBytes) + s.SegmentBase.SizeInBytes() - uint64(len(s.mem))
}

func (s *SegmentBase) SizeInBytes() uint64 {
	// 4 /* size of memCRC -> uint32 */
	// 4 /* size of chunkFactor -> uint32 */
	// 8 /* size of numDocs -> uint64 */
	// 8 /* size of storedIndexOffset -> uint64 */
	// 8 /* size of fieldsIndexOffset -> uint64 */
	// 8 /* size of docValueOffset -> uint64 */
	sizeInBytes := 40

	sizeInBytes += len(s.mem) + int(segment.SizeOfSlice)

	// fieldsMap
	for k, _ := range s.fieldsMap {
		sizeInBytes += (len(k) + int(segment.SizeOfString)) + 2 /* size of uint16 */
	}
	sizeInBytes += int(segment.SizeOfMap) /* overhead from map */

	// fieldsInv, dictLocs
	for _, entry := range s.fieldsInv {
		sizeInBytes += (len(entry) + int(segment.SizeOfString))
	}
	sizeInBytes += len(s.dictLocs) * 8          /* size of uint64 */
	sizeInBytes += int(segment.SizeOfSlice) * 3 /* overhead from slices */

	// fieldDvIterMap
	sizeInBytes += len(s.fieldDvIterMap) *
		int(segment.SizeOfPointer+2 /* size of uint16 */)
	for _, entry := range s.fieldDvIterMap {
		if entry != nil {
			sizeInBytes += int(entry.sizeInBytes())
		}
	}
	sizeInBytes += int(segment.SizeOfMap)

	return uint64(sizeInBytes)
}

func (s *Segment) AddRef() {
	s.m.Lock()
	s.refs++
	s.m.Unlock()
}

func (s *Segment) DecRef() (err error) {
	s.m.Lock()
	s.refs--
	if s.refs == 0 {
		err = s.closeActual()
	}
	s.m.Unlock()
	return err
}

func (s *Segment) loadConfig() error {
	crcOffset := len(s.mm) - 4
	s.crc = binary.BigEndian.Uint32(s.mm[crcOffset : crcOffset+4])

	verOffset := crcOffset - 4
	s.version = binary.BigEndian.Uint32(s.mm[verOffset : verOffset+4])
	if s.version != version {
		return fmt.Errorf("unsupported version %d", s.version)
	}

	chunkOffset := verOffset - 4
	s.chunkFactor = binary.BigEndian.Uint32(s.mm[chunkOffset : chunkOffset+4])

	docValueOffset := chunkOffset - 8
	s.docValueOffset = binary.BigEndian.Uint64(s.mm[docValueOffset : docValueOffset+8])

	fieldsIndexOffset := docValueOffset - 8
	s.fieldsIndexOffset = binary.BigEndian.Uint64(s.mm[fieldsIndexOffset : fieldsIndexOffset+8])

	storedIndexOffset := fieldsIndexOffset - 8
	s.storedIndexOffset = binary.BigEndian.Uint64(s.mm[storedIndexOffset : storedIndexOffset+8])

	numDocsOffset := storedIndexOffset - 8
	s.numDocs = binary.BigEndian.Uint64(s.mm[numDocsOffset : numDocsOffset+8])
	return nil
}

func (s *SegmentBase) loadFields() error {
	// NOTE for now we assume the fields index immediately preceeds
	// the footer, and if this changes, need to adjust accordingly (or
	// store explicit length), where s.mem was sliced from s.mm in Open().
	fieldsIndexEnd := uint64(len(s.mem))

	// iterate through fields index
	var fieldID uint64
	for s.fieldsIndexOffset+(8*fieldID) < fieldsIndexEnd {
		addr := binary.BigEndian.Uint64(s.mem[s.fieldsIndexOffset+(8*fieldID) : s.fieldsIndexOffset+(8*fieldID)+8])

		dictLoc, read := binary.Uvarint(s.mem[addr:fieldsIndexEnd])
		n := uint64(read)
		s.dictLocs = append(s.dictLocs, dictLoc)

		var nameLen uint64
		nameLen, read = binary.Uvarint(s.mem[addr+n : fieldsIndexEnd])
		n += uint64(read)

		name := string(s.mem[addr+n : addr+n+nameLen])
		s.fieldsInv = append(s.fieldsInv, name)
		s.fieldsMap[name] = uint16(fieldID + 1)

		fieldID++
	}
	return nil
}

// Dictionary returns the term dictionary for the specified field
func (s *SegmentBase) Dictionary(field string) (segment.TermDictionary, error) {
	dict, err := s.dictionary(field)
	if err == nil && dict == nil {
		return &segment.EmptyDictionary{}, nil
	}
	return dict, err
}

func (sb *SegmentBase) dictionary(field string) (rv *Dictionary, err error) {
	fieldIDPlus1 := sb.fieldsMap[field]
	if fieldIDPlus1 > 0 {
		rv = &Dictionary{
			sb:      sb,
			field:   field,
			fieldID: fieldIDPlus1 - 1,
		}

		dictStart := sb.dictLocs[rv.fieldID]
		if dictStart > 0 {
			// read the length of the vellum data
			vellumLen, read := binary.Uvarint(sb.mem[dictStart : dictStart+binary.MaxVarintLen64])
			fstBytes := sb.mem[dictStart+uint64(read) : dictStart+uint64(read)+vellumLen]
			if fstBytes != nil {
				rv.fst, err = vellum.Load(fstBytes)
				if err != nil {
					return nil, fmt.Errorf("dictionary field %s vellum err: %v", field, err)
				}
			}
		}
	}

	return rv, nil
}

// VisitDocument invokes the DocFieldValueVistor for each stored field
// for the specified doc number
func (s *SegmentBase) VisitDocument(num uint64, visitor segment.DocumentFieldValueVisitor) error {
	// first make sure this is a valid number in this segment
	if num < s.numDocs {
		meta, compressed := s.getDocStoredMetaAndCompressed(num)
		uncompressed, err := snappy.Decode(nil, compressed)
		if err != nil {
			return err
		}
		// now decode meta and process
		reader := bytes.NewReader(meta)
		decoder := govarint.NewU64Base128Decoder(reader)

		keepGoing := true
		for keepGoing {
			field, err := decoder.GetU64()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			typ, err := decoder.GetU64()
			if err != nil {
				return err
			}
			offset, err := decoder.GetU64()
			if err != nil {
				return err
			}
			l, err := decoder.GetU64()
			if err != nil {
				return err
			}
			numap, err := decoder.GetU64()
			if err != nil {
				return err
			}
			var arrayPos []uint64
			if numap > 0 {
				arrayPos = make([]uint64, numap)
				for i := 0; i < int(numap); i++ {
					ap, err := decoder.GetU64()
					if err != nil {
						return err
					}
					arrayPos[i] = ap
				}
			}

			value := uncompressed[offset : offset+l]
			keepGoing = visitor(s.fieldsInv[field], byte(typ), value, arrayPos)
		}
	}
	return nil
}

// Count returns the number of documents in this segment.
func (s *SegmentBase) Count() uint64 {
	return s.numDocs
}

// DocNumbers returns a bitset corresponding to the doc numbers of all the
// provided _id strings
func (s *SegmentBase) DocNumbers(ids []string) (*roaring.Bitmap, error) {
	rv := roaring.New()

	if len(s.fieldsMap) > 0 {
		idDict, err := s.dictionary("_id")
		if err != nil {
			return nil, err
		}

		var postings *PostingsList
		for _, id := range ids {
			postings, err = idDict.postingsList([]byte(id), nil, postings)
			if err != nil {
				return nil, err
			}
			if postings.postings != nil {
				rv.Or(postings.postings)
			}
		}
	}

	return rv, nil
}

// Fields returns the field names used in this segment
func (s *SegmentBase) Fields() []string {
	return s.fieldsInv
}

// Path returns the path of this segment on disk
func (s *Segment) Path() string {
	return s.path
}

// Close releases all resources associated with this segment
func (s *Segment) Close() (err error) {
	return s.DecRef()
}

func (s *Segment) closeActual() (err error) {
	if s.mm != nil {
		err = s.mm.Unmap()
	}
	// try to close file even if unmap failed
	if s.f != nil {
		err2 := s.f.Close()
		if err == nil {
			// try to return first error
			err = err2
		}
	}
	return
}

// some helpers i started adding for the command-line utility

// Data returns the underlying mmaped data slice
func (s *Segment) Data() []byte {
	return s.mm
}

// CRC returns the CRC value stored in the file footer
func (s *Segment) CRC() uint32 {
	return s.crc
}

// Version returns the file version in the file footer
func (s *Segment) Version() uint32 {
	return s.version
}

// ChunkFactor returns the chunk factor in the file footer
func (s *Segment) ChunkFactor() uint32 {
	return s.chunkFactor
}

// FieldsIndexOffset returns the fields index offset in the file footer
func (s *Segment) FieldsIndexOffset() uint64 {
	return s.fieldsIndexOffset
}

// StoredIndexOffset returns the stored value index offset in the file footer
func (s *Segment) StoredIndexOffset() uint64 {
	return s.storedIndexOffset
}

// DocValueOffset returns the docValue offset in the file footer
func (s *Segment) DocValueOffset() uint64 {
	return s.docValueOffset
}

// NumDocs returns the number of documents in the file footer
func (s *Segment) NumDocs() uint64 {
	return s.numDocs
}

// DictAddr is a helper function to compute the file offset where the
// dictionary is stored for the specified field.
func (s *Segment) DictAddr(field string) (uint64, error) {
	fieldIDPlus1, ok := s.fieldsMap[field]
	if !ok {
		return 0, fmt.Errorf("no such field '%s'", field)
	}

	return s.dictLocs[fieldIDPlus1-1], nil
}

func (s *SegmentBase) loadDvIterators() error {
	if s.docValueOffset == fieldNotUninverted {
		return nil
	}

	var read uint64
	for fieldID, field := range s.fieldsInv {
		fieldLoc, n := binary.Uvarint(s.mem[s.docValueOffset+read : s.docValueOffset+read+binary.MaxVarintLen64])
		if n <= 0 {
			return fmt.Errorf("loadDvIterators: failed to read the docvalue offsets for field %d", fieldID)
		}
		s.fieldDvIterMap[uint16(fieldID)], _ = s.loadFieldDocValueIterator(field, fieldLoc)
		read += uint64(n)
	}
	return nil
}
