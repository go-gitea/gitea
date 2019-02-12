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
	"math"
	"sort"

	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/index/scorch/segment"
	"github.com/golang/snappy"
)

type docValueIterator struct {
	field          string
	curChunkNum    uint64
	numChunks      uint64
	chunkLens      []uint64
	dvDataLoc      uint64
	curChunkHeader []MetaData
	curChunkData   []byte // compressed data cache
}

func (di *docValueIterator) sizeInBytes() uint64 {
	// curChunkNum, numChunks, dvDataLoc --> uint64
	sizeInBytes := 24

	// field
	sizeInBytes += (len(di.field) + int(segment.SizeOfString))

	// chunkLens, curChunkHeader
	sizeInBytes += len(di.chunkLens)*8 +
		len(di.curChunkHeader)*24 +
		int(segment.SizeOfSlice*2) /* overhead from slices */

	// curChunkData is mmap'ed, not included

	return uint64(sizeInBytes)
}

func (di *docValueIterator) fieldName() string {
	return di.field
}

func (di *docValueIterator) curChunkNumber() uint64 {
	return di.curChunkNum
}

func (s *SegmentBase) loadFieldDocValueIterator(field string,
	fieldDvLoc uint64) (*docValueIterator, error) {
	// get the docValue offset for the given fields
	if fieldDvLoc == fieldNotUninverted {
		return nil, fmt.Errorf("loadFieldDocValueIterator: "+
			"no docValues found for field: %s", field)
	}

	// read the number of chunks, chunk lengths
	var offset, clen uint64
	numChunks, read := binary.Uvarint(s.mem[fieldDvLoc : fieldDvLoc+binary.MaxVarintLen64])
	if read <= 0 {
		return nil, fmt.Errorf("failed to read the field "+
			"doc values for field %s", field)
	}
	offset += uint64(read)

	fdvIter := &docValueIterator{
		curChunkNum: math.MaxUint64,
		field:       field,
		chunkLens:   make([]uint64, int(numChunks)),
	}
	for i := 0; i < int(numChunks); i++ {
		clen, read = binary.Uvarint(s.mem[fieldDvLoc+offset : fieldDvLoc+offset+binary.MaxVarintLen64])
		if read <= 0 {
			return nil, fmt.Errorf("corrupted chunk length during segment load")
		}
		fdvIter.chunkLens[i] = clen
		offset += uint64(read)
	}

	fdvIter.dvDataLoc = fieldDvLoc + offset
	return fdvIter, nil
}

func (di *docValueIterator) loadDvChunk(chunkNumber,
	localDocNum uint64, s *SegmentBase) error {
	// advance to the chunk where the docValues
	// reside for the given docNum
	destChunkDataLoc := di.dvDataLoc
	for i := 0; i < int(chunkNumber); i++ {
		destChunkDataLoc += di.chunkLens[i]
	}

	curChunkSize := di.chunkLens[chunkNumber]
	// read the number of docs reside in the chunk
	numDocs, read := binary.Uvarint(s.mem[destChunkDataLoc : destChunkDataLoc+binary.MaxVarintLen64])
	if read <= 0 {
		return fmt.Errorf("failed to read the chunk")
	}
	chunkMetaLoc := destChunkDataLoc + uint64(read)

	offset := uint64(0)
	di.curChunkHeader = make([]MetaData, int(numDocs))
	for i := 0; i < int(numDocs); i++ {
		di.curChunkHeader[i].DocNum, read = binary.Uvarint(s.mem[chunkMetaLoc+offset : chunkMetaLoc+offset+binary.MaxVarintLen64])
		offset += uint64(read)
		di.curChunkHeader[i].DocDvLoc, read = binary.Uvarint(s.mem[chunkMetaLoc+offset : chunkMetaLoc+offset+binary.MaxVarintLen64])
		offset += uint64(read)
		di.curChunkHeader[i].DocDvLen, read = binary.Uvarint(s.mem[chunkMetaLoc+offset : chunkMetaLoc+offset+binary.MaxVarintLen64])
		offset += uint64(read)
	}

	compressedDataLoc := chunkMetaLoc + offset
	dataLength := destChunkDataLoc + curChunkSize - compressedDataLoc
	di.curChunkData = s.mem[compressedDataLoc : compressedDataLoc+dataLength]
	di.curChunkNum = chunkNumber
	return nil
}

func (di *docValueIterator) visitDocValues(docNum uint64,
	visitor index.DocumentFieldTermVisitor) error {
	// binary search the term locations for the docNum
	start, length := di.getDocValueLocs(docNum)
	if start == math.MaxUint64 || length == math.MaxUint64 {
		return nil
	}
	// uncompress the already loaded data
	uncompressed, err := snappy.Decode(nil, di.curChunkData)
	if err != nil {
		return err
	}

	// pick the terms for the given docNum
	uncompressed = uncompressed[start : start+length]
	for {
		i := bytes.Index(uncompressed, termSeparatorSplitSlice)
		if i < 0 {
			break
		}

		visitor(di.field, uncompressed[0:i])
		uncompressed = uncompressed[i+1:]
	}

	return nil
}

func (di *docValueIterator) getDocValueLocs(docNum uint64) (uint64, uint64) {
	i := sort.Search(len(di.curChunkHeader), func(i int) bool {
		return di.curChunkHeader[i].DocNum >= docNum
	})
	if i < len(di.curChunkHeader) && di.curChunkHeader[i].DocNum == docNum {
		return di.curChunkHeader[i].DocDvLoc, di.curChunkHeader[i].DocDvLen
	}
	return math.MaxUint64, math.MaxUint64
}

// VisitDocumentFieldTerms is an implementation of the
// DocumentFieldTermVisitable interface
func (s *SegmentBase) VisitDocumentFieldTerms(localDocNum uint64, fields []string,
	visitor index.DocumentFieldTermVisitor) error {
	fieldIDPlus1 := uint16(0)
	ok := true
	for _, field := range fields {
		if fieldIDPlus1, ok = s.fieldsMap[field]; !ok {
			continue
		}
		// find the chunkNumber where the docValues are stored
		docInChunk := localDocNum / uint64(s.chunkFactor)

		if dvIter, exists := s.fieldDvIterMap[fieldIDPlus1-1]; exists &&
			dvIter != nil {
			// check if the chunk is already loaded
			if docInChunk != dvIter.curChunkNumber() {
				err := dvIter.loadDvChunk(docInChunk, localDocNum, s)
				if err != nil {
					continue
				}
			}

			_ = dvIter.visitDocValues(localDocNum, visitor)
		}
	}
	return nil
}

// VisitableDocValueFields returns the list of fields with
// persisted doc value terms ready to be visitable using the
// VisitDocumentFieldTerms method.
func (s *Segment) VisitableDocValueFields() ([]string, error) {
	var rv []string
	for fieldID, field := range s.fieldsInv {
		if dvIter, ok := s.fieldDvIterMap[uint16(fieldID)]; ok &&
			dvIter != nil {
			rv = append(rv, field)
		}
	}
	return rv, nil
}
