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

	"github.com/RoaringBitmap/roaring"
	"github.com/Smerity/govarint"
	"github.com/blevesearch/bleve/index/scorch/segment"
)

// PostingsList is an in-memory represenation of a postings list
type PostingsList struct {
	sb             *SegmentBase
	postingsOffset uint64
	freqOffset     uint64
	locOffset      uint64
	locBitmap      *roaring.Bitmap
	postings       *roaring.Bitmap
	except         *roaring.Bitmap
}

// Iterator returns an iterator for this postings list
func (p *PostingsList) Iterator() segment.PostingsIterator {
	return p.iterator(nil)
}

func (p *PostingsList) iterator(rv *PostingsIterator) *PostingsIterator {
	if rv == nil {
		rv = &PostingsIterator{}
	} else {
		*rv = PostingsIterator{} // clear the struct
	}
	rv.postings = p

	if p.postings != nil {
		// prepare the freq chunk details
		var n uint64
		var read int
		var numFreqChunks uint64
		numFreqChunks, read = binary.Uvarint(p.sb.mem[p.freqOffset+n : p.freqOffset+n+binary.MaxVarintLen64])
		n += uint64(read)
		rv.freqChunkLens = make([]uint64, int(numFreqChunks))
		for i := 0; i < int(numFreqChunks); i++ {
			rv.freqChunkLens[i], read = binary.Uvarint(p.sb.mem[p.freqOffset+n : p.freqOffset+n+binary.MaxVarintLen64])
			n += uint64(read)
		}
		rv.freqChunkStart = p.freqOffset + n

		// prepare the loc chunk details
		n = 0
		var numLocChunks uint64
		numLocChunks, read = binary.Uvarint(p.sb.mem[p.locOffset+n : p.locOffset+n+binary.MaxVarintLen64])
		n += uint64(read)
		rv.locChunkLens = make([]uint64, int(numLocChunks))
		for i := 0; i < int(numLocChunks); i++ {
			rv.locChunkLens[i], read = binary.Uvarint(p.sb.mem[p.locOffset+n : p.locOffset+n+binary.MaxVarintLen64])
			n += uint64(read)
		}
		rv.locChunkStart = p.locOffset + n
		rv.locBitmap = p.locBitmap

		rv.all = p.postings.Iterator()
		if p.except != nil {
			allExcept := roaring.AndNot(p.postings, p.except)
			rv.actual = allExcept.Iterator()
		} else {
			rv.actual = p.postings.Iterator()
		}
	}

	return rv
}

// Count returns the number of items on this postings list
func (p *PostingsList) Count() uint64 {
	if p.postings != nil {
		n := p.postings.GetCardinality()
		if p.except != nil {
			e := p.except.GetCardinality()
			if e > n {
				e = n
			}
			return n - e
		}
		return n
	}
	return 0
}

func (rv *PostingsList) read(postingsOffset uint64, d *Dictionary) error {
	rv.postingsOffset = postingsOffset

	// read the location of the freq/norm details
	var n uint64
	var read int

	rv.freqOffset, read = binary.Uvarint(d.sb.mem[postingsOffset+n : postingsOffset+binary.MaxVarintLen64])
	n += uint64(read)

	rv.locOffset, read = binary.Uvarint(d.sb.mem[postingsOffset+n : postingsOffset+n+binary.MaxVarintLen64])
	n += uint64(read)

	var locBitmapOffset uint64
	locBitmapOffset, read = binary.Uvarint(d.sb.mem[postingsOffset+n : postingsOffset+n+binary.MaxVarintLen64])
	n += uint64(read)

	var locBitmapLen uint64
	locBitmapLen, read = binary.Uvarint(d.sb.mem[locBitmapOffset : locBitmapOffset+binary.MaxVarintLen64])

	locRoaringBytes := d.sb.mem[locBitmapOffset+uint64(read) : locBitmapOffset+uint64(read)+locBitmapLen]

	rv.locBitmap = roaring.NewBitmap()
	_, err := rv.locBitmap.FromBuffer(locRoaringBytes)
	if err != nil {
		return fmt.Errorf("error loading roaring bitmap of locations with hits: %v", err)
	}

	var postingsLen uint64
	postingsLen, read = binary.Uvarint(d.sb.mem[postingsOffset+n : postingsOffset+n+binary.MaxVarintLen64])
	n += uint64(read)

	roaringBytes := d.sb.mem[postingsOffset+n : postingsOffset+n+postingsLen]

	rv.postings = roaring.NewBitmap()
	_, err = rv.postings.FromBuffer(roaringBytes)
	if err != nil {
		return fmt.Errorf("error loading roaring bitmap: %v", err)
	}

	return nil
}

// PostingsIterator provides a way to iterate through the postings list
type PostingsIterator struct {
	postings  *PostingsList
	all       roaring.IntIterable
	offset    int
	locoffset int
	actual    roaring.IntIterable

	currChunk         uint32
	currChunkFreqNorm []byte
	currChunkLoc      []byte
	freqNormDecoder   *govarint.Base128Decoder
	locDecoder        *govarint.Base128Decoder

	freqChunkLens  []uint64
	freqChunkStart uint64

	locChunkLens  []uint64
	locChunkStart uint64

	locBitmap *roaring.Bitmap

	next Posting
}

func (i *PostingsIterator) loadChunk(chunk int) error {
	if chunk >= len(i.freqChunkLens) || chunk >= len(i.locChunkLens) {
		return fmt.Errorf("tried to load chunk that doesn't exist %d/(%d %d)", chunk, len(i.freqChunkLens), len(i.locChunkLens))
	}
	// load correct chunk bytes
	start := i.freqChunkStart
	for j := 0; j < chunk; j++ {
		start += i.freqChunkLens[j]
	}
	end := start + i.freqChunkLens[chunk]
	i.currChunkFreqNorm = i.postings.sb.mem[start:end]
	i.freqNormDecoder = govarint.NewU64Base128Decoder(bytes.NewReader(i.currChunkFreqNorm))

	start = i.locChunkStart
	for j := 0; j < chunk; j++ {
		start += i.locChunkLens[j]
	}
	end = start + i.locChunkLens[chunk]
	i.currChunkLoc = i.postings.sb.mem[start:end]
	i.locDecoder = govarint.NewU64Base128Decoder(bytes.NewReader(i.currChunkLoc))
	i.currChunk = uint32(chunk)
	return nil
}

func (i *PostingsIterator) readFreqNorm() (uint64, uint64, error) {
	freq, err := i.freqNormDecoder.GetU64()
	if err != nil {
		return 0, 0, fmt.Errorf("error reading frequency: %v", err)
	}
	normBits, err := i.freqNormDecoder.GetU64()
	if err != nil {
		return 0, 0, fmt.Errorf("error reading norm: %v", err)
	}
	return freq, normBits, err
}

// readLocation processes all the integers on the stream representing a single
// location.  if you care about it, pass in a non-nil location struct, and we
// will fill it.  if you don't care about it, pass in nil and we safely consume
// the contents.
func (i *PostingsIterator) readLocation(l *Location) error {
	// read off field
	fieldID, err := i.locDecoder.GetU64()
	if err != nil {
		return fmt.Errorf("error reading location field: %v", err)
	}
	// read off pos
	pos, err := i.locDecoder.GetU64()
	if err != nil {
		return fmt.Errorf("error reading location pos: %v", err)
	}
	// read off start
	start, err := i.locDecoder.GetU64()
	if err != nil {
		return fmt.Errorf("error reading location start: %v", err)
	}
	// read off end
	end, err := i.locDecoder.GetU64()
	if err != nil {
		return fmt.Errorf("error reading location end: %v", err)
	}
	// read off num array pos
	numArrayPos, err := i.locDecoder.GetU64()
	if err != nil {
		return fmt.Errorf("error reading location num array pos: %v", err)
	}

	// group these together for less branching
	if l != nil {
		l.field = i.postings.sb.fieldsInv[fieldID]
		l.pos = pos
		l.start = start
		l.end = end
		if numArrayPos > 0 {
			l.ap = make([]uint64, int(numArrayPos))
		}
	}

	// read off array positions
	for k := 0; k < int(numArrayPos); k++ {
		ap, err := i.locDecoder.GetU64()
		if err != nil {
			return fmt.Errorf("error reading array position: %v", err)
		}
		if l != nil {
			l.ap[k] = ap
		}
	}

	return nil
}

// Next returns the next posting on the postings list, or nil at the end
func (i *PostingsIterator) Next() (segment.Posting, error) {
	if i.actual == nil || !i.actual.HasNext() {
		return nil, nil
	}
	n := i.actual.Next()
	nChunk := n / i.postings.sb.chunkFactor
	allN := i.all.Next()
	allNChunk := allN / i.postings.sb.chunkFactor

	// n is the next actual hit (excluding some postings)
	// allN is the next hit in the full postings
	// if they don't match, adjust offsets to factor in item we're skipping over
	// incr the all iterator, and check again
	for allN != n {

		// in different chunks, reset offsets
		if allNChunk != nChunk {
			i.locoffset = 0
			i.offset = 0
		} else {

			if i.currChunk != nChunk || i.currChunkFreqNorm == nil {
				err := i.loadChunk(int(nChunk))
				if err != nil {
					return nil, fmt.Errorf("error loading chunk: %v", err)
				}
			}

			// read off freq/offsets even though we don't care about them
			freq, _, err := i.readFreqNorm()
			if err != nil {
				return nil, err
			}
			if i.locBitmap.Contains(allN) {
				for j := 0; j < int(freq); j++ {
					err := i.readLocation(nil)
					if err != nil {
						return nil, err
					}
				}
			}

			// in same chunk, need to account for offsets
			i.offset++
		}

		allN = i.all.Next()
	}

	if i.currChunk != nChunk || i.currChunkFreqNorm == nil {
		err := i.loadChunk(int(nChunk))
		if err != nil {
			return nil, fmt.Errorf("error loading chunk: %v", err)
		}
	}

	i.next = Posting{} // clear the struct.
	rv := &i.next
	rv.iterator = i
	rv.docNum = uint64(n)

	var err error
	var normBits uint64
	rv.freq, normBits, err = i.readFreqNorm()
	if err != nil {
		return nil, err
	}
	rv.norm = math.Float32frombits(uint32(normBits))
	if i.locBitmap.Contains(n) {
		// read off 'freq' locations
		rv.locs = make([]segment.Location, rv.freq)
		locs := make([]Location, rv.freq)
		for j := 0; j < int(rv.freq); j++ {
			err := i.readLocation(&locs[j])
			if err != nil {
				return nil, err
			}
			rv.locs[j] = &locs[j]
		}
	}

	return rv, nil
}

// Posting is a single entry in a postings list
type Posting struct {
	iterator *PostingsIterator
	docNum   uint64

	freq uint64
	norm float32
	locs []segment.Location
}

// Number returns the document number of this posting in this segment
func (p *Posting) Number() uint64 {
	return p.docNum
}

// Frequency returns the frequence of occurance of this term in this doc/field
func (p *Posting) Frequency() uint64 {
	return p.freq
}

// Norm returns the normalization factor for this posting
func (p *Posting) Norm() float64 {
	return float64(p.norm)
}

// Locations returns the location information for each occurance
func (p *Posting) Locations() []segment.Location {
	return p.locs
}

// Location represents the location of a single occurance
type Location struct {
	field string
	pos   uint64
	start uint64
	end   uint64
	ap    []uint64
}

// Field returns the name of the field (useful in composite fields to know
// which original field the value came from)
func (l *Location) Field() string {
	return l.field
}

// Start returns the start byte offset of this occurance
func (l *Location) Start() uint64 {
	return l.start
}

// End returns the end byte offset of this occurance
func (l *Location) End() uint64 {
	return l.end
}

// Pos returns the 1-based phrase position of this occurance
func (l *Location) Pos() uint64 {
	return l.pos
}

// ArrayPositions returns the array position vector associated with this occurance
func (l *Location) ArrayPositions() []uint64 {
	return l.ap
}
