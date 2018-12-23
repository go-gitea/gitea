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
	"bufio"
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"sort"

	"github.com/Smerity/govarint"
	"github.com/blevesearch/bleve/index/scorch/segment/mem"
	"github.com/couchbase/vellum"
	"github.com/golang/snappy"
)

const version uint32 = 3

const fieldNotUninverted = math.MaxUint64

// PersistSegmentBase persists SegmentBase in the zap file format.
func PersistSegmentBase(sb *SegmentBase, path string) error {
	flag := os.O_RDWR | os.O_CREATE

	f, err := os.OpenFile(path, flag, 0600)
	if err != nil {
		return err
	}

	cleanup := func() {
		_ = f.Close()
		_ = os.Remove(path)
	}

	br := bufio.NewWriter(f)

	_, err = br.Write(sb.mem)
	if err != nil {
		cleanup()
		return err
	}

	err = persistFooter(sb.numDocs, sb.storedIndexOffset, sb.fieldsIndexOffset, sb.docValueOffset,
		sb.chunkFactor, sb.memCRC, br)
	if err != nil {
		cleanup()
		return err
	}

	err = br.Flush()
	if err != nil {
		cleanup()
		return err
	}

	err = f.Sync()
	if err != nil {
		cleanup()
		return err
	}

	err = f.Close()
	if err != nil {
		cleanup()
		return err
	}

	return nil
}

// PersistSegment takes the in-memory segment and persists it to
// the specified path in the zap file format.
func PersistSegment(memSegment *mem.Segment, path string, chunkFactor uint32) error {
	flag := os.O_RDWR | os.O_CREATE

	f, err := os.OpenFile(path, flag, 0600)
	if err != nil {
		return err
	}

	cleanup := func() {
		_ = f.Close()
		_ = os.Remove(path)
	}

	// buffer the output
	br := bufio.NewWriter(f)

	// wrap it for counting (tracking offsets)
	cr := NewCountHashWriter(br)

	numDocs, storedIndexOffset, fieldsIndexOffset, docValueOffset, _, err :=
		persistBase(memSegment, cr, chunkFactor)
	if err != nil {
		cleanup()
		return err
	}

	err = persistFooter(numDocs, storedIndexOffset, fieldsIndexOffset, docValueOffset,
		chunkFactor, cr.Sum32(), cr)
	if err != nil {
		cleanup()
		return err
	}

	err = br.Flush()
	if err != nil {
		cleanup()
		return err
	}

	err = f.Sync()
	if err != nil {
		cleanup()
		return err
	}

	err = f.Close()
	if err != nil {
		cleanup()
		return err
	}

	return nil
}

func persistBase(memSegment *mem.Segment, cr *CountHashWriter, chunkFactor uint32) (
	numDocs, storedIndexOffset, fieldsIndexOffset, docValueOffset uint64,
	dictLocs []uint64, err error) {
	docValueOffset = uint64(fieldNotUninverted)

	if len(memSegment.Stored) > 0 {
		storedIndexOffset, err = persistStored(memSegment, cr)
		if err != nil {
			return 0, 0, 0, 0, nil, err
		}

		freqOffsets, locOffsets, err := persistPostingDetails(memSegment, cr, chunkFactor)
		if err != nil {
			return 0, 0, 0, 0, nil, err
		}

		postingsListLocs, err := persistPostingsLocs(memSegment, cr)
		if err != nil {
			return 0, 0, 0, 0, nil, err
		}

		postingsLocs, err := persistPostingsLists(memSegment, cr, postingsListLocs, freqOffsets, locOffsets)
		if err != nil {
			return 0, 0, 0, 0, nil, err
		}

		dictLocs, err = persistDictionary(memSegment, cr, postingsLocs)
		if err != nil {
			return 0, 0, 0, 0, nil, err
		}

		docValueOffset, err = persistFieldDocValues(memSegment, cr, chunkFactor)
		if err != nil {
			return 0, 0, 0, 0, nil, err
		}
	} else {
		dictLocs = make([]uint64, len(memSegment.FieldsInv))
	}

	fieldsIndexOffset, err = persistFields(memSegment.FieldsInv, cr, dictLocs)
	if err != nil {
		return 0, 0, 0, 0, nil, err
	}

	return uint64(len(memSegment.Stored)), storedIndexOffset, fieldsIndexOffset, docValueOffset,
		dictLocs, nil
}

func persistStored(memSegment *mem.Segment, w *CountHashWriter) (uint64, error) {
	var curr int
	var metaBuf bytes.Buffer
	var data, compressed []byte

	metaEncoder := govarint.NewU64Base128Encoder(&metaBuf)

	docNumOffsets := make(map[int]uint64, len(memSegment.Stored))

	for docNum, storedValues := range memSegment.Stored {
		if docNum != 0 {
			// reset buffer if necessary
			curr = 0
			metaBuf.Reset()
			data = data[:0]
			compressed = compressed[:0]
		}

		st := memSegment.StoredTypes[docNum]
		sp := memSegment.StoredPos[docNum]

		// encode fields in order
		for fieldID := range memSegment.FieldsInv {
			if storedFieldValues, ok := storedValues[uint16(fieldID)]; ok {
				stf := st[uint16(fieldID)]
				spf := sp[uint16(fieldID)]

				var err2 error
				curr, data, err2 = persistStoredFieldValues(fieldID,
					storedFieldValues, stf, spf, curr, metaEncoder, data)
				if err2 != nil {
					return 0, err2
				}
			}
		}

		metaEncoder.Close()
		metaBytes := metaBuf.Bytes()

		// compress the data
		compressed = snappy.Encode(compressed, data)

		// record where we're about to start writing
		docNumOffsets[docNum] = uint64(w.Count())

		// write out the meta len and compressed data len
		_, err := writeUvarints(w, uint64(len(metaBytes)), uint64(len(compressed)))
		if err != nil {
			return 0, err
		}

		// now write the meta
		_, err = w.Write(metaBytes)
		if err != nil {
			return 0, err
		}
		// now write the compressed data
		_, err = w.Write(compressed)
		if err != nil {
			return 0, err
		}
	}

	// return value is the start of the stored index
	rv := uint64(w.Count())
	// now write out the stored doc index
	for docNum := range memSegment.Stored {
		err := binary.Write(w, binary.BigEndian, docNumOffsets[docNum])
		if err != nil {
			return 0, err
		}
	}

	return rv, nil
}

func persistStoredFieldValues(fieldID int,
	storedFieldValues [][]byte, stf []byte, spf [][]uint64,
	curr int, metaEncoder *govarint.Base128Encoder, data []byte) (
	int, []byte, error) {
	for i := 0; i < len(storedFieldValues); i++ {
		// encode field
		_, err := metaEncoder.PutU64(uint64(fieldID))
		if err != nil {
			return 0, nil, err
		}
		// encode type
		_, err = metaEncoder.PutU64(uint64(stf[i]))
		if err != nil {
			return 0, nil, err
		}
		// encode start offset
		_, err = metaEncoder.PutU64(uint64(curr))
		if err != nil {
			return 0, nil, err
		}
		// end len
		_, err = metaEncoder.PutU64(uint64(len(storedFieldValues[i])))
		if err != nil {
			return 0, nil, err
		}
		// encode number of array pos
		_, err = metaEncoder.PutU64(uint64(len(spf[i])))
		if err != nil {
			return 0, nil, err
		}
		// encode all array positions
		for _, pos := range spf[i] {
			_, err = metaEncoder.PutU64(pos)
			if err != nil {
				return 0, nil, err
			}
		}

		data = append(data, storedFieldValues[i]...)
		curr += len(storedFieldValues[i])
	}

	return curr, data, nil
}

func persistPostingDetails(memSegment *mem.Segment, w *CountHashWriter, chunkFactor uint32) ([]uint64, []uint64, error) {
	var freqOffsets, locOfffsets []uint64
	tfEncoder := newChunkedIntCoder(uint64(chunkFactor), uint64(len(memSegment.Stored)-1))
	for postingID := range memSegment.Postings {
		if postingID != 0 {
			tfEncoder.Reset()
		}
		freqs := memSegment.Freqs[postingID]
		norms := memSegment.Norms[postingID]
		postingsListItr := memSegment.Postings[postingID].Iterator()
		var offset int
		for postingsListItr.HasNext() {

			docNum := uint64(postingsListItr.Next())

			// put freq
			err := tfEncoder.Add(docNum, freqs[offset])
			if err != nil {
				return nil, nil, err
			}

			// put norm
			norm := norms[offset]
			normBits := math.Float32bits(norm)
			err = tfEncoder.Add(docNum, uint64(normBits))
			if err != nil {
				return nil, nil, err
			}

			offset++
		}

		// record where this postings freq info starts
		freqOffsets = append(freqOffsets, uint64(w.Count()))

		tfEncoder.Close()
		_, err := tfEncoder.Write(w)
		if err != nil {
			return nil, nil, err
		}

	}

	// now do it again for the locations
	locEncoder := newChunkedIntCoder(uint64(chunkFactor), uint64(len(memSegment.Stored)-1))
	for postingID := range memSegment.Postings {
		if postingID != 0 {
			locEncoder.Reset()
		}
		freqs := memSegment.Freqs[postingID]
		locfields := memSegment.Locfields[postingID]
		locpos := memSegment.Locpos[postingID]
		locstarts := memSegment.Locstarts[postingID]
		locends := memSegment.Locends[postingID]
		locarraypos := memSegment.Locarraypos[postingID]
		postingsListItr := memSegment.Postings[postingID].Iterator()
		var offset int
		var locOffset int
		for postingsListItr.HasNext() {
			docNum := uint64(postingsListItr.Next())
			for i := 0; i < int(freqs[offset]); i++ {
				if len(locfields) > 0 {
					// put field
					err := locEncoder.Add(docNum, uint64(locfields[locOffset]))
					if err != nil {
						return nil, nil, err
					}

					// put pos
					err = locEncoder.Add(docNum, locpos[locOffset])
					if err != nil {
						return nil, nil, err
					}

					// put start
					err = locEncoder.Add(docNum, locstarts[locOffset])
					if err != nil {
						return nil, nil, err
					}

					// put end
					err = locEncoder.Add(docNum, locends[locOffset])
					if err != nil {
						return nil, nil, err
					}

					// put the number of array positions to follow
					num := len(locarraypos[locOffset])
					err = locEncoder.Add(docNum, uint64(num))
					if err != nil {
						return nil, nil, err
					}

					// put each array position
					for _, pos := range locarraypos[locOffset] {
						err = locEncoder.Add(docNum, pos)
						if err != nil {
							return nil, nil, err
						}
					}
				}
				locOffset++
			}
			offset++
		}

		// record where this postings loc info starts
		locOfffsets = append(locOfffsets, uint64(w.Count()))
		locEncoder.Close()
		_, err := locEncoder.Write(w)
		if err != nil {
			return nil, nil, err
		}
	}
	return freqOffsets, locOfffsets, nil
}

func persistPostingsLocs(memSegment *mem.Segment, w *CountHashWriter) (rv []uint64, err error) {
	rv = make([]uint64, 0, len(memSegment.PostingsLocs))
	var reuseBuf bytes.Buffer
	reuseBufVarint := make([]byte, binary.MaxVarintLen64)
	for postingID := range memSegment.PostingsLocs {
		// record where we start this posting loc
		rv = append(rv, uint64(w.Count()))
		// write out the length and bitmap
		_, err = writeRoaringWithLen(memSegment.PostingsLocs[postingID], w, &reuseBuf, reuseBufVarint)
		if err != nil {
			return nil, err
		}
	}
	return rv, nil
}

func persistPostingsLists(memSegment *mem.Segment, w *CountHashWriter,
	postingsListLocs, freqOffsets, locOffsets []uint64) (rv []uint64, err error) {
	rv = make([]uint64, 0, len(memSegment.Postings))
	var reuseBuf bytes.Buffer
	reuseBufVarint := make([]byte, binary.MaxVarintLen64)
	for postingID := range memSegment.Postings {
		// record where we start this posting list
		rv = append(rv, uint64(w.Count()))

		// write out the term info, loc info, and loc posting list offset
		_, err = writeUvarints(w, freqOffsets[postingID],
			locOffsets[postingID], postingsListLocs[postingID])
		if err != nil {
			return nil, err
		}

		// write out the length and bitmap
		_, err = writeRoaringWithLen(memSegment.Postings[postingID], w, &reuseBuf, reuseBufVarint)
		if err != nil {
			return nil, err
		}
	}
	return rv, nil
}

func persistDictionary(memSegment *mem.Segment, w *CountHashWriter, postingsLocs []uint64) ([]uint64, error) {
	rv := make([]uint64, 0, len(memSegment.DictKeys))

	varintBuf := make([]byte, binary.MaxVarintLen64)

	var buffer bytes.Buffer
	for fieldID, fieldTerms := range memSegment.DictKeys {
		if fieldID != 0 {
			buffer.Reset()
		}

		// start a new vellum for this field
		builder, err := vellum.New(&buffer, nil)
		if err != nil {
			return nil, err
		}

		dict := memSegment.Dicts[fieldID]
		// now walk the dictionary in order of fieldTerms (already sorted)
		for _, fieldTerm := range fieldTerms {
			postingID := dict[fieldTerm] - 1
			postingsAddr := postingsLocs[postingID]
			err = builder.Insert([]byte(fieldTerm), postingsAddr)
			if err != nil {
				return nil, err
			}
		}
		err = builder.Close()
		if err != nil {
			return nil, err
		}

		// record where this dictionary starts
		rv = append(rv, uint64(w.Count()))

		vellumData := buffer.Bytes()

		// write out the length of the vellum data
		n := binary.PutUvarint(varintBuf, uint64(len(vellumData)))
		_, err = w.Write(varintBuf[:n])
		if err != nil {
			return nil, err
		}

		// write this vellum to disk
		_, err = w.Write(vellumData)
		if err != nil {
			return nil, err
		}
	}

	return rv, nil
}

type docIDRange []uint64

func (a docIDRange) Len() int           { return len(a) }
func (a docIDRange) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a docIDRange) Less(i, j int) bool { return a[i] < a[j] }

func persistDocValues(memSegment *mem.Segment, w *CountHashWriter,
	chunkFactor uint32) (map[uint16]uint64, error) {
	fieldChunkOffsets := make(map[uint16]uint64, len(memSegment.FieldsInv))
	fdvEncoder := newChunkedContentCoder(uint64(chunkFactor), uint64(len(memSegment.Stored)-1))

	for fieldID := range memSegment.DocValueFields {
		field := memSegment.FieldsInv[fieldID]
		docTermMap := make(map[uint64][]byte, 0)
		dict, err := memSegment.Dictionary(field)
		if err != nil {
			return nil, err
		}

		dictItr := dict.Iterator()
		next, err := dictItr.Next()
		for err == nil && next != nil {
			postings, err1 := dict.PostingsList(next.Term, nil)
			if err1 != nil {
				return nil, err
			}

			postingsItr := postings.Iterator()
			nextPosting, err2 := postingsItr.Next()
			for err2 == nil && nextPosting != nil {
				docNum := nextPosting.Number()
				docTermMap[docNum] = append(docTermMap[docNum], []byte(next.Term)...)
				docTermMap[docNum] = append(docTermMap[docNum], termSeparator)
				nextPosting, err2 = postingsItr.Next()
			}
			if err2 != nil {
				return nil, err2
			}

			next, err = dictItr.Next()
		}

		if err != nil {
			return nil, err
		}
		// sort wrt to docIDs
		var docNumbers docIDRange
		for k := range docTermMap {
			docNumbers = append(docNumbers, k)
		}
		sort.Sort(docNumbers)

		for _, docNum := range docNumbers {
			err = fdvEncoder.Add(docNum, docTermMap[docNum])
			if err != nil {
				return nil, err
			}
		}

		fieldChunkOffsets[fieldID] = uint64(w.Count())
		err = fdvEncoder.Close()
		if err != nil {
			return nil, err
		}
		// persist the doc value details for this field
		_, err = fdvEncoder.Write(w)
		if err != nil {
			return nil, err
		}
		// reseting encoder for the next field
		fdvEncoder.Reset()
	}

	return fieldChunkOffsets, nil
}

func persistFieldDocValues(memSegment *mem.Segment, w *CountHashWriter,
	chunkFactor uint32) (uint64, error) {
	fieldDvOffsets, err := persistDocValues(memSegment, w, chunkFactor)
	if err != nil {
		return 0, err
	}

	fieldDocValuesOffset := uint64(w.Count())
	buf := make([]byte, binary.MaxVarintLen64)
	offset := uint64(0)
	ok := true
	for fieldID := range memSegment.FieldsInv {
		// if the field isn't configured for docValue, then mark
		// the offset accordingly
		if offset, ok = fieldDvOffsets[uint16(fieldID)]; !ok {
			offset = fieldNotUninverted
		}
		n := binary.PutUvarint(buf, uint64(offset))
		_, err := w.Write(buf[:n])
		if err != nil {
			return 0, err
		}
	}

	return fieldDocValuesOffset, nil
}

func NewSegmentBase(memSegment *mem.Segment, chunkFactor uint32) (*SegmentBase, error) {
	var br bytes.Buffer

	cr := NewCountHashWriter(&br)

	numDocs, storedIndexOffset, fieldsIndexOffset, docValueOffset, dictLocs, err :=
		persistBase(memSegment, cr, chunkFactor)
	if err != nil {
		return nil, err
	}

	return InitSegmentBase(br.Bytes(), cr.Sum32(), chunkFactor,
		memSegment.FieldsMap, memSegment.FieldsInv, numDocs,
		storedIndexOffset, fieldsIndexOffset, docValueOffset, dictLocs)
}

func InitSegmentBase(mem []byte, memCRC uint32, chunkFactor uint32,
	fieldsMap map[string]uint16, fieldsInv []string, numDocs uint64,
	storedIndexOffset uint64, fieldsIndexOffset uint64, docValueOffset uint64,
	dictLocs []uint64) (*SegmentBase, error) {
	sb := &SegmentBase{
		mem:               mem,
		memCRC:            memCRC,
		chunkFactor:       chunkFactor,
		fieldsMap:         fieldsMap,
		fieldsInv:         fieldsInv,
		numDocs:           numDocs,
		storedIndexOffset: storedIndexOffset,
		fieldsIndexOffset: fieldsIndexOffset,
		docValueOffset:    docValueOffset,
		dictLocs:          dictLocs,
		fieldDvIterMap:    make(map[uint16]*docValueIterator),
	}

	err := sb.loadDvIterators()
	if err != nil {
		return nil, err
	}

	return sb, nil
}
