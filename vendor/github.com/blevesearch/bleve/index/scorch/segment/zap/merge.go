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

	"github.com/RoaringBitmap/roaring"
	"github.com/Smerity/govarint"
	"github.com/couchbase/vellum"
	"github.com/golang/snappy"
)

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

	// buffer the output
	br := bufio.NewWriter(f)

	// wrap it for counting (tracking offsets)
	cr := NewCountHashWriter(br)

	fieldsInv := mergeFields(segments)
	fieldsMap := mapFields(fieldsInv)

	var newDocNums [][]uint64
	var storedIndexOffset uint64
	fieldDvLocsOffset := uint64(fieldNotUninverted)
	var dictLocs []uint64

	newSegDocCount := computeNewDocCount(segments, drops)
	if newSegDocCount > 0 {
		storedIndexOffset, newDocNums, err = mergeStoredAndRemap(segments, drops,
			fieldsMap, fieldsInv, newSegDocCount, cr)
		if err != nil {
			cleanup()
			return nil, err
		}

		dictLocs, fieldDvLocsOffset, err = persistMergedRest(segments, drops, fieldsInv, fieldsMap,
			newDocNums, newSegDocCount, chunkFactor, cr)
		if err != nil {
			cleanup()
			return nil, err
		}
	} else {
		dictLocs = make([]uint64, len(fieldsInv))
	}

	fieldsIndexOffset, err := persistFields(fieldsInv, cr, dictLocs)
	if err != nil {
		cleanup()
		return nil, err
	}

	err = persistFooter(newSegDocCount, storedIndexOffset,
		fieldsIndexOffset, fieldDvLocsOffset, chunkFactor, cr.Sum32(), cr)
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

// mapFields takes the fieldsInv list and builds the map
func mapFields(fields []string) map[string]uint16 {
	rv := make(map[string]uint16, len(fields))
	for i, fieldName := range fields {
		rv[fieldName] = uint16(i)
	}
	return rv
}

// computeNewDocCount determines how many documents will be in the newly
// merged segment when obsoleted docs are dropped
func computeNewDocCount(segments []*Segment, drops []*roaring.Bitmap) uint64 {
	var newDocCount uint64
	for segI, segment := range segments {
		newDocCount += segment.NumDocs()
		if drops[segI] != nil {
			newDocCount -= drops[segI].GetCardinality()
		}
	}
	return newDocCount
}

func persistMergedRest(segments []*Segment, drops []*roaring.Bitmap,
	fieldsInv []string, fieldsMap map[string]uint16, newDocNums [][]uint64,
	newSegDocCount uint64, chunkFactor uint32,
	w *CountHashWriter) ([]uint64, uint64, error) {

	var bufReuse bytes.Buffer
	var bufMaxVarintLen64 []byte = make([]byte, binary.MaxVarintLen64)
	var bufLoc []uint64

	rv := make([]uint64, len(fieldsInv))
	fieldDvLocs := make([]uint64, len(fieldsInv))
	fieldDvLocsOffset := uint64(fieldNotUninverted)

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

		// collect FST iterators from all segments for this field
		var dicts []*Dictionary
		var itrs []vellum.Iterator
		for _, segment := range segments {
			dict, err2 := segment.dictionary(fieldName)
			if err2 != nil {
				return nil, 0, err2
			}
			dicts = append(dicts, dict)

			if dict != nil && dict.fst != nil {
				itr, err2 := dict.fst.Iterator(nil, nil)
				if err2 != nil && err2 != vellum.ErrIteratorDone {
					return nil, 0, err2
				}
				if itr != nil {
					itrs = append(itrs, itr)
				}
			}
		}

		// create merging iterator
		mergeItr, err := vellum.NewMergeIterator(itrs, func(postingOffsets []uint64) uint64 {
			// we don't actually use the merged value
			return 0
		})

		tfEncoder := newChunkedIntCoder(uint64(chunkFactor), newSegDocCount-1)
		locEncoder := newChunkedIntCoder(uint64(chunkFactor), newSegDocCount-1)

		if uint64(cap(docTermMap)) < newSegDocCount {
			docTermMap = make([][]byte, newSegDocCount)
		} else {
			docTermMap = docTermMap[0:newSegDocCount]
			for docNum := range docTermMap { // reset the docTermMap
				docTermMap[docNum] = docTermMap[docNum][:0]
			}
		}

		for err == nil {
			term, _ := mergeItr.Current()

			newRoaring := roaring.NewBitmap()
			newRoaringLocs := roaring.NewBitmap()

			tfEncoder.Reset()
			locEncoder.Reset()

			// now go back and get posting list for this term
			// but pass in the deleted docs for that segment
			for dictI, dict := range dicts {
				if dict == nil {
					continue
				}
				postings, err2 := dict.postingsList(term, drops[dictI])
				if err2 != nil {
					return nil, 0, err2
				}

				postItr := postings.Iterator()
				next, err2 := postItr.Next()
				for next != nil && err2 == nil {
					hitNewDocNum := newDocNums[dictI][next.Number()]
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
							args[0] = uint64(fieldsMap[loc.Field()])
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
			}

			tfEncoder.Close()
			locEncoder.Close()

			if newRoaring.GetCardinality() > 0 {
				// this field/term actually has hits in the new segment, lets write it down
				freqOffset := uint64(w.Count())
				_, err = tfEncoder.Write(w)
				if err != nil {
					return nil, 0, err
				}
				locOffset := uint64(w.Count())
				_, err = locEncoder.Write(w)
				if err != nil {
					return nil, 0, err
				}
				postingLocOffset := uint64(w.Count())
				_, err = writeRoaringWithLen(newRoaringLocs, w, &bufReuse, bufMaxVarintLen64)
				if err != nil {
					return nil, 0, err
				}
				postingOffset := uint64(w.Count())
				// write out the start of the term info
				buf := bufMaxVarintLen64
				n := binary.PutUvarint(buf, freqOffset)
				_, err = w.Write(buf[:n])
				if err != nil {
					return nil, 0, err
				}

				// write out the start of the loc info
				n = binary.PutUvarint(buf, locOffset)
				_, err = w.Write(buf[:n])
				if err != nil {
					return nil, 0, err
				}

				// write out the start of the loc posting list
				n = binary.PutUvarint(buf, postingLocOffset)
				_, err = w.Write(buf[:n])
				if err != nil {
					return nil, 0, err
				}
				_, err = writeRoaringWithLen(newRoaring, w, &bufReuse, bufMaxVarintLen64)
				if err != nil {
					return nil, 0, err
				}

				err = newVellum.Insert(term, postingOffset)
				if err != nil {
					return nil, 0, err
				}
			}

			err = mergeItr.Next()
		}
		if err != nil && err != vellum.ErrIteratorDone {
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

	fieldDvLocsOffset = uint64(w.Count())

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

const docDropped = math.MaxUint64

func mergeStoredAndRemap(segments []*Segment, drops []*roaring.Bitmap,
	fieldsMap map[string]uint16, fieldsInv []string, newSegDocCount uint64,
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

		// for each doc num
		for docNum := uint64(0); docNum < segment.numDocs; docNum++ {
			// TODO: roaring's API limits docNums to 32-bits?
			if drops[segI] != nil && drops[segI].Contains(uint32(docNum)) {
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
				fieldID := int(fieldsMap[field])
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

				// has stored values for this field
				num := len(storedFieldValues)

				// process each value
				for i := 0; i < num; i++ {
					// encode field
					_, err2 := metaEncoder.PutU64(uint64(fieldID))
					if err2 != nil {
						return 0, nil, err2
					}
					// encode type
					_, err2 = metaEncoder.PutU64(uint64(typs[int(fieldID)][i]))
					if err2 != nil {
						return 0, nil, err2
					}
					// encode start offset
					_, err2 = metaEncoder.PutU64(uint64(curr))
					if err2 != nil {
						return 0, nil, err2
					}
					// end len
					_, err2 = metaEncoder.PutU64(uint64(len(storedFieldValues[i])))
					if err2 != nil {
						return 0, nil, err2
					}
					// encode number of array pos
					_, err2 = metaEncoder.PutU64(uint64(len(poss[int(fieldID)][i])))
					if err2 != nil {
						return 0, nil, err2
					}
					// encode all array positions
					for j := 0; j < len(poss[int(fieldID)][i]); j++ {
						_, err2 = metaEncoder.PutU64(poss[int(fieldID)][i][j])
						if err2 != nil {
							return 0, nil, err2
						}
					}
					// append data
					data = append(data, storedFieldValues[i]...)
					// update curr
					curr += len(storedFieldValues[i])
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
	offset := uint64(w.Count())

	// now write out the stored doc index
	for docNum := range docNumOffsets {
		err := binary.Write(w, binary.BigEndian, docNumOffsets[docNum])
		if err != nil {
			return 0, nil, err
		}
	}

	return offset, rv, nil
}

// mergeFields builds a unified list of fields used across all the input segments
func mergeFields(segments []*Segment) []string {
	fieldsMap := map[string]struct{}{}
	for _, segment := range segments {
		fields := segment.Fields()
		for _, field := range fields {
			fieldsMap[field] = struct{}{}
		}
	}

	rv := make([]string, 0, len(fieldsMap))
	// ensure _id stays first
	rv = append(rv, "_id")
	for k := range fieldsMap {
		if k != "_id" {
			rv = append(rv, k)
		}
	}
	return rv
}
