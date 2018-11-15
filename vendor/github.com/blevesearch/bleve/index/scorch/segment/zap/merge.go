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
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/RoaringBitmap/roaring"
	"github.com/Smerity/govarint"
	"github.com/couchbase/vellum"
	"github.com/golang/snappy"
)

const docDropped = math.MaxUint64 // sentinel docNum to represent a deleted doc

// Merge takes a slice of zap segments and bit masks describing which
// documents may be dropped, and creates a new segment containing the
// remaining data.  This new segment is built at the specified path,
// with the provided chunkFactor.
func Merge(segments []*Segment, drops []*roaring.Bitmap, path string,
	chunkFactor uint32) ([][]uint64, error) {
	flag := os.O_RDWR | os.O_CREATE

	f, err := os.OpenFile(path, flag, 0600)
	if err != nil {
		return nil, err
	}

	cleanup := func() {
		_ = f.Close()
		_ = os.Remove(path)
	}

	segmentBases := make([]*SegmentBase, len(segments))
	for segmenti, segment := range segments {
		segmentBases[segmenti] = &segment.SegmentBase
	}

	// buffer the output
	br := bufio.NewWriter(f)

	// wrap it for counting (tracking offsets)
	cr := NewCountHashWriter(br)

	newDocNums, numDocs, storedIndexOffset, fieldsIndexOffset, docValueOffset, _, _, _, err :=
		MergeToWriter(segmentBases, drops, chunkFactor, cr)
	if err != nil {
		cleanup()
		return nil, err
	}

	err = persistFooter(numDocs, storedIndexOffset, fieldsIndexOffset,
		docValueOffset, chunkFactor, cr.Sum32(), cr)
	if err != nil {
		cleanup()
		return nil, err
	}

	err = br.Flush()
	if err != nil {
		cleanup()
		return nil, err
	}

	err = f.Sync()
	if err != nil {
		cleanup()
		return nil, err
	}

	err = f.Close()
	if err != nil {
		cleanup()
		return nil, err
	}

	return newDocNums, nil
}

func MergeToWriter(segments []*SegmentBase, drops []*roaring.Bitmap,
	chunkFactor uint32, cr *CountHashWriter) (
	newDocNums [][]uint64,
	numDocs, storedIndexOffset, fieldsIndexOffset, docValueOffset uint64,
	dictLocs []uint64, fieldsInv []string, fieldsMap map[string]uint16,
	err error) {
	docValueOffset = uint64(fieldNotUninverted)

	var fieldsSame bool
	fieldsSame, fieldsInv = mergeFields(segments)
	fieldsMap = mapFields(fieldsInv)

	numDocs = computeNewDocCount(segments, drops)
	if numDocs > 0 {
		storedIndexOffset, newDocNums, err = mergeStoredAndRemap(segments, drops,
			fieldsMap, fieldsInv, fieldsSame, numDocs, cr)
		if err != nil {
			return nil, 0, 0, 0, 0, nil, nil, nil, err
		}

		dictLocs, docValueOffset, err = persistMergedRest(segments, drops, fieldsInv, fieldsMap,
			newDocNums, numDocs, chunkFactor, cr)
		if err != nil {
			return nil, 0, 0, 0, 0, nil, nil, nil, err
		}
	} else {
		dictLocs = make([]uint64, len(fieldsInv))
	}

	fieldsIndexOffset, err = persistFields(fieldsInv, cr, dictLocs)
	if err != nil {
		return nil, 0, 0, 0, 0, nil, nil, nil, err
	}

	return newDocNums, numDocs, storedIndexOffset, fieldsIndexOffset, docValueOffset, dictLocs, fieldsInv, fieldsMap, nil
}

// mapFields takes the fieldsInv list and returns a map of fieldName
// to fieldID+1
func mapFields(fields []string) map[string]uint16 {
	rv := make(map[string]uint16, len(fields))
	for i, fieldName := range fields {
		rv[fieldName] = uint16(i) + 1
	}
	return rv
}

// computeNewDocCount determines how many documents will be in the newly
// merged segment when obsoleted docs are dropped
func computeNewDocCount(segments []*SegmentBase, drops []*roaring.Bitmap) uint64 {
	var newDocCount uint64
	for segI, segment := range segments {
		newDocCount += segment.numDocs
		if drops[segI] != nil {
			newDocCount -= drops[segI].GetCardinality()
		}
	}
	return newDocCount
}

func persistMergedRest(segments []*SegmentBase, dropsIn []*roaring.Bitmap,
	fieldsInv []string, fieldsMap map[string]uint16, newDocNumsIn [][]uint64,
	newSegDocCount uint64, chunkFactor uint32,
	w *CountHashWriter) ([]uint64, uint64, error) {

	var bufReuse bytes.Buffer
	var bufMaxVarintLen64 []byte = make([]byte, binary.MaxVarintLen64)
	var bufLoc []uint64

	var postings *PostingsList
	var postItr *PostingsIterator

	rv := make([]uint64, len(fieldsInv))
	fieldDvLocs := make([]uint64, len(fieldsInv))

	tfEncoder := newChunkedIntCoder(uint64(chunkFactor), newSegDocCount-1)
	locEncoder := newChunkedIntCoder(uint64(chunkFactor), newSegDocCount-1)

	// docTermMap is keyed by docNum, where the array impl provides
	// better memory usage behavior than a sparse-friendlier hashmap
	// for when docs have much structural similarity (i.e., every doc
	// has a given field)
	var docTermMap [][]byte

	var vellumBuf bytes.Buffer

	// for each field
	for fieldID, fieldName := range fieldsInv {
		if fieldID != 0 {
			vellumBuf.Reset()
		}
		newVellum, err := vellum.New(&vellumBuf, nil)
		if err != nil {
			return nil, 0, err
		}

		// collect FST iterators from all active segments for this field
		var newDocNums [][]uint64
		var drops []*roaring.Bitmap
		var dicts []*Dictionary
		var itrs []vellum.Iterator

		for segmentI, segment := range segments {
			dict, err2 := segment.dictionary(fieldName)
			if err2 != nil {
				return nil, 0, err2
			}
			if dict != nil && dict.fst != nil {
				itr, err2 := dict.fst.Iterator(nil, nil)
				if err2 != nil && err2 != vellum.ErrIteratorDone {
					return nil, 0, err2
				}
				if itr != nil {
					newDocNums = append(newDocNums, newDocNumsIn[segmentI])
					drops = append(drops, dropsIn[segmentI])
					dicts = append(dicts, dict)
					itrs = append(itrs, itr)
				}
			}
		}

		if uint64(cap(docTermMap)) < newSegDocCount {
			docTermMap = make([][]byte, newSegDocCount)
		} else {
			docTermMap = docTermMap[0:newSegDocCount]
			for docNum := range docTermMap { // reset the docTermMap
				docTermMap[docNum] = docTermMap[docNum][:0]
			}
		}

		var prevTerm []byte

		newRoaring := roaring.NewBitmap()
		newRoaringLocs := roaring.NewBitmap()

		finishTerm := func(term []byte) error {
			if term == nil {
				return nil
			}

			tfEncoder.Close()
			locEncoder.Close()

			if newRoaring.GetCardinality() > 0 {
				// this field/term actually has hits in the new segment, lets write it down
				freqOffset := uint64(w.Count())
				_, err := tfEncoder.Write(w)
				if err != nil {
					return err
				}
				locOffset := uint64(w.Count())
				_, err = locEncoder.Write(w)
				if err != nil {
					return err
				}
				postingLocOffset := uint64(w.Count())
				_, err = writeRoaringWithLen(newRoaringLocs, w, &bufReuse, bufMaxVarintLen64)
				if err != nil {
					return err
				}
				postingOffset := uint64(w.Count())

				// write out the start of the term info
				n := binary.PutUvarint(bufMaxVarintLen64, freqOffset)
				_, err = w.Write(bufMaxVarintLen64[:n])
				if err != nil {
					return err
				}
				// write out the start of the loc info
				n = binary.PutUvarint(bufMaxVarintLen64, locOffset)
				_, err = w.Write(bufMaxVarintLen64[:n])
				if err != nil {
					return err
				}
				// write out the start of the posting locs
				n = binary.PutUvarint(bufMaxVarintLen64, postingLocOffset)
				_, err = w.Write(bufMaxVarintLen64[:n])
				if err != nil {
					return err
				}
				_, err = writeRoaringWithLen(newRoaring, w, &bufReuse, bufMaxVarintLen64)
				if err != nil {
					return err
				}

				err = newVellum.Insert(term, postingOffset)
				if err != nil {
					return err
				}
			}

			newRoaring = roaring.NewBitmap()
			newRoaringLocs = roaring.NewBitmap()

			tfEncoder.Reset()
			locEncoder.Reset()

			return nil
		}

		enumerator, err := newEnumerator(itrs)

		for err == nil {
			term, itrI, postingsOffset := enumerator.Current()

			if !bytes.Equal(prevTerm, term) {
				// if the term changed, write out the info collected
				// for the previous term
				err2 := finishTerm(prevTerm)
				if err2 != nil {
					return nil, 0, err2
				}
			}

			var err2 error
			postings, err2 = dicts[itrI].postingsListFromOffset(
				postingsOffset, drops[itrI], postings)
			if err2 != nil {
				return nil, 0, err2
			}

			newDocNumsI := newDocNums[itrI]

			postItr = postings.iterator(postItr)
			next, err2 := postItr.Next()
			for next != nil && err2 == nil {
				hitNewDocNum := newDocNumsI[next.Number()]
				if hitNewDocNum == docDropped {
					return nil, 0, fmt.Errorf("see hit with dropped doc num")
				}
				newRoaring.Add(uint32(hitNewDocNum))
				// encode norm bits
				norm := next.Norm()
				normBits := math.Float32bits(float32(norm))
				err = tfEncoder.Add(hitNewDocNum, next.Frequency(), uint64(normBits))
				if err != nil {
					return nil, 0, err
				}
				locs := next.Locations()
				if len(locs) > 0 {
					newRoaringLocs.Add(uint32(hitNewDocNum))
					for _, loc := range locs {
						if cap(bufLoc) < 5+len(loc.ArrayPositions()) {
							bufLoc = make([]uint64, 0, 5+len(loc.ArrayPositions()))
						}
						args := bufLoc[0:5]
						args[0] = uint64(fieldsMap[loc.Field()] - 1)
						args[1] = loc.Pos()
						args[2] = loc.Start()
						args[3] = loc.End()
						args[4] = uint64(len(loc.ArrayPositions()))
						args = append(args, loc.ArrayPositions()...)
						err = locEncoder.Add(hitNewDocNum, args...)
						if err != nil {
							return nil, 0, err
						}
					}
				}

				docTermMap[hitNewDocNum] =
					append(append(docTermMap[hitNewDocNum], term...), termSeparator)

				next, err2 = postItr.Next()
			}
			if err2 != nil {
				return nil, 0, err2
			}

			prevTerm = prevTerm[:0] // copy to prevTerm in case Next() reuses term mem
			prevTerm = append(prevTerm, term...)

			err = enumerator.Next()
		}
		if err != nil && err != vellum.ErrIteratorDone {
			return nil, 0, err
		}

		err = finishTerm(prevTerm)
		if err != nil {
			return nil, 0, err
		}

		dictOffset := uint64(w.Count())

		err = newVellum.Close()
		if err != nil {
			return nil, 0, err
		}
		vellumData := vellumBuf.Bytes()

		// write out the length of the vellum data
		n := binary.PutUvarint(bufMaxVarintLen64, uint64(len(vellumData)))
		_, err = w.Write(bufMaxVarintLen64[:n])
		if err != nil {
			return nil, 0, err
		}

		// write this vellum to disk
		_, err = w.Write(vellumData)
		if err != nil {
			return nil, 0, err
		}

		rv[fieldID] = dictOffset

		// update the field doc values
		fdvEncoder := newChunkedContentCoder(uint64(chunkFactor), newSegDocCount-1)
		for docNum, docTerms := range docTermMap {
			if len(docTerms) > 0 {
				err = fdvEncoder.Add(uint64(docNum), docTerms)
				if err != nil {
					return nil, 0, err
				}
			}
		}
		err = fdvEncoder.Close()
		if err != nil {
			return nil, 0, err
		}

		// get the field doc value offset
		fieldDvLocs[fieldID] = uint64(w.Count())

		// persist the doc value details for this field
		_, err = fdvEncoder.Write(w)
		if err != nil {
			return nil, 0, err
		}
	}

	fieldDvLocsOffset := uint64(w.Count())

	buf := bufMaxVarintLen64
	for _, offset := range fieldDvLocs {
		n := binary.PutUvarint(buf, uint64(offset))
		_, err := w.Write(buf[:n])
		if err != nil {
			return nil, 0, err
		}
	}

	return rv, fieldDvLocsOffset, nil
}

func mergeStoredAndRemap(segments []*SegmentBase, drops []*roaring.Bitmap,
	fieldsMap map[string]uint16, fieldsInv []string, fieldsSame bool, newSegDocCount uint64,
	w *CountHashWriter) (uint64, [][]uint64, error) {
	var rv [][]uint64 // The remapped or newDocNums for each segment.

	var newDocNum uint64

	var curr int
	var metaBuf bytes.Buffer
	var data, compressed []byte

	metaEncoder := govarint.NewU64Base128Encoder(&metaBuf)

	vals := make([][][]byte, len(fieldsInv))
	typs := make([][]byte, len(fieldsInv))
	poss := make([][][]uint64, len(fieldsInv))

	docNumOffsets := make([]uint64, newSegDocCount)

	// for each segment
	for segI, segment := range segments {
		segNewDocNums := make([]uint64, segment.numDocs)

		dropsI := drops[segI]

		// optimize when the field mapping is the same across all
		// segments and there are no deletions, via byte-copying
		// of stored docs bytes directly to the writer
		if fieldsSame && (dropsI == nil || dropsI.GetCardinality() == 0) {
			err := segment.copyStoredDocs(newDocNum, docNumOffsets, w)
			if err != nil {
				return 0, nil, err
			}

			for i := uint64(0); i < segment.numDocs; i++ {
				segNewDocNums[i] = newDocNum
				newDocNum++
			}
			rv = append(rv, segNewDocNums)

			continue
		}

		// for each doc num
		for docNum := uint64(0); docNum < segment.numDocs; docNum++ {
			// TODO: roaring's API limits docNums to 32-bits?
			if dropsI != nil && dropsI.Contains(uint32(docNum)) {
				segNewDocNums[docNum] = docDropped
				continue
			}

			segNewDocNums[docNum] = newDocNum

			curr = 0
			metaBuf.Reset()
			data = data[:0]
			compressed = compressed[:0]

			// collect all the data
			for i := 0; i < len(fieldsInv); i++ {
				vals[i] = vals[i][:0]
				typs[i] = typs[i][:0]
				poss[i] = poss[i][:0]
			}
			err := segment.VisitDocument(docNum, func(field string, typ byte, value []byte, pos []uint64) bool {
				fieldID := int(fieldsMap[field]) - 1
				vals[fieldID] = append(vals[fieldID], value)
				typs[fieldID] = append(typs[fieldID], typ)
				poss[fieldID] = append(poss[fieldID], pos)
				return true
			})
			if err != nil {
				return 0, nil, err
			}

			// now walk the fields in order
			for fieldID := range fieldsInv {
				storedFieldValues := vals[int(fieldID)]

				stf := typs[int(fieldID)]
				spf := poss[int(fieldID)]

				var err2 error
				curr, data, err2 = persistStoredFieldValues(fieldID,
					storedFieldValues, stf, spf, curr, metaEncoder, data)
				if err2 != nil {
					return 0, nil, err2
				}
			}

			metaEncoder.Close()
			metaBytes := metaBuf.Bytes()

			compressed = snappy.Encode(compressed, data)

			// record where we're about to start writing
			docNumOffsets[newDocNum] = uint64(w.Count())

			// write out the meta len and compressed data len
			_, err = writeUvarints(w, uint64(len(metaBytes)), uint64(len(compressed)))
			if err != nil {
				return 0, nil, err
			}
			// now write the meta
			_, err = w.Write(metaBytes)
			if err != nil {
				return 0, nil, err
			}
			// now write the compressed data
			_, err = w.Write(compressed)
			if err != nil {
				return 0, nil, err
			}

			newDocNum++
		}

		rv = append(rv, segNewDocNums)
	}

	// return value is the start of the stored index
	storedIndexOffset := uint64(w.Count())

	// now write out the stored doc index
	for _, docNumOffset := range docNumOffsets {
		err := binary.Write(w, binary.BigEndian, docNumOffset)
		if err != nil {
			return 0, nil, err
		}
	}

	return storedIndexOffset, rv, nil
}

// copyStoredDocs writes out a segment's stored doc info, optimized by
// using a single Write() call for the entire set of bytes.  The
// newDocNumOffsets is filled with the new offsets for each doc.
func (s *SegmentBase) copyStoredDocs(newDocNum uint64, newDocNumOffsets []uint64,
	w *CountHashWriter) error {
	if s.numDocs <= 0 {
		return nil
	}

	indexOffset0, storedOffset0, _, _, _ :=
		s.getDocStoredOffsets(0) // the segment's first doc

	indexOffsetN, storedOffsetN, readN, metaLenN, dataLenN :=
		s.getDocStoredOffsets(s.numDocs - 1) // the segment's last doc

	storedOffset0New := uint64(w.Count())

	storedBytes := s.mem[storedOffset0 : storedOffsetN+readN+metaLenN+dataLenN]
	_, err := w.Write(storedBytes)
	if err != nil {
		return err
	}

	// remap the storedOffset's for the docs into new offsets relative
	// to storedOffset0New, filling the given docNumOffsetsOut array
	for indexOffset := indexOffset0; indexOffset <= indexOffsetN; indexOffset += 8 {
		storedOffset := binary.BigEndian.Uint64(s.mem[indexOffset : indexOffset+8])
		storedOffsetNew := storedOffset - storedOffset0 + storedOffset0New
		newDocNumOffsets[newDocNum] = storedOffsetNew
		newDocNum += 1
	}

	return nil
}

// mergeFields builds a unified list of fields used across all the
// input segments, and computes whether the fields are the same across
// segments (which depends on fields to be sorted in the same way
// across segments)
func mergeFields(segments []*SegmentBase) (bool, []string) {
	fieldsSame := true

	var segment0Fields []string
	if len(segments) > 0 {
		segment0Fields = segments[0].Fields()
	}

	fieldsExist := map[string]struct{}{}
	for _, segment := range segments {
		fields := segment.Fields()
		for fieldi, field := range fields {
			fieldsExist[field] = struct{}{}
			if len(segment0Fields) != len(fields) || segment0Fields[fieldi] != field {
				fieldsSame = false
			}
		}
	}

	rv := make([]string, 0, len(fieldsExist))
	// ensure _id stays first
	rv = append(rv, "_id")
	for k := range fieldsExist {
		if k != "_id" {
			rv = append(rv, k)
		}
	}

	sort.Strings(rv[1:]) // leave _id as first

	return fieldsSame, rv
}
