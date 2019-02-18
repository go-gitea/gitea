//  Copyright (c) 2018 Couchbase, Inc.
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

package scorch

import (
	"fmt"

	"github.com/RoaringBitmap/roaring"

	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/index/scorch/segment"
	"github.com/blevesearch/bleve/index/scorch/segment/zap"
)

var OptimizeConjunction = true
var OptimizeConjunctionUnadorned = true
var OptimizeDisjunctionUnadorned = true

func (s *IndexSnapshotTermFieldReader) Optimize(kind string,
	octx index.OptimizableContext) (index.OptimizableContext, error) {
	if OptimizeConjunction && kind == "conjunction" {
		return s.optimizeConjunction(octx)
	}

	if OptimizeConjunctionUnadorned && kind == "conjunction:unadorned" {
		return s.optimizeConjunctionUnadorned(octx)
	}

	if OptimizeDisjunctionUnadorned && kind == "disjunction:unadorned" {
		return s.optimizeDisjunctionUnadorned(octx)
	}

	return octx, nil
}

var OptimizeDisjunctionUnadornedMinChildCardinality = uint64(256)

// ----------------------------------------------------------------

func (s *IndexSnapshotTermFieldReader) optimizeConjunction(
	octx index.OptimizableContext) (index.OptimizableContext, error) {
	if octx == nil {
		octx = &OptimizeTFRConjunction{snapshot: s.snapshot}
	}

	o, ok := octx.(*OptimizeTFRConjunction)
	if !ok {
		return octx, nil
	}

	if o.snapshot != s.snapshot {
		return nil, fmt.Errorf("tried to optimize conjunction across different snapshots")
	}

	o.tfrs = append(o.tfrs, s)

	return o, nil
}

type OptimizeTFRConjunction struct {
	snapshot *IndexSnapshot

	tfrs []*IndexSnapshotTermFieldReader
}

func (o *OptimizeTFRConjunction) Finish() (index.Optimized, error) {
	if len(o.tfrs) <= 1 {
		return nil, nil
	}

	for i := range o.snapshot.segment {
		itr0, ok := o.tfrs[0].iterators[i].(*zap.PostingsIterator)
		if !ok || itr0.ActualBM == nil {
			continue
		}

		itr1, ok := o.tfrs[1].iterators[i].(*zap.PostingsIterator)
		if !ok || itr1.ActualBM == nil {
			continue
		}

		bm := roaring.And(itr0.ActualBM, itr1.ActualBM)

		for _, tfr := range o.tfrs[2:] {
			itr, ok := tfr.iterators[i].(*zap.PostingsIterator)
			if !ok || itr.ActualBM == nil {
				continue
			}

			bm.And(itr.ActualBM)
		}

		// in this conjunction optimization, the postings iterators
		// will all share the same AND'ed together actual bitmap.  The
		// regular conjunction searcher machinery will still be used,
		// but the underlying bitmap will be smaller.
		for _, tfr := range o.tfrs {
			itr, ok := tfr.iterators[i].(*zap.PostingsIterator)
			if ok && itr.ActualBM != nil {
				itr.ActualBM = bm
				itr.Actual = bm.Iterator()
			}
		}
	}

	return nil, nil
}

// ----------------------------------------------------------------

// An "unadorned" conjunction optimization is appropriate when
// additional or subsidiary information like freq-norm's and
// term-vectors are not required, and instead only the internal-id's
// are needed.
func (s *IndexSnapshotTermFieldReader) optimizeConjunctionUnadorned(
	octx index.OptimizableContext) (index.OptimizableContext, error) {
	if octx == nil {
		octx = &OptimizeTFRConjunctionUnadorned{snapshot: s.snapshot}
	}

	o, ok := octx.(*OptimizeTFRConjunctionUnadorned)
	if !ok {
		return nil, nil
	}

	if o.snapshot != s.snapshot {
		return nil, fmt.Errorf("tried to optimize unadorned conjunction across different snapshots")
	}

	o.tfrs = append(o.tfrs, s)

	return o, nil
}

type OptimizeTFRConjunctionUnadorned struct {
	snapshot *IndexSnapshot

	tfrs []*IndexSnapshotTermFieldReader
}

var OptimizeTFRConjunctionUnadornedTerm = []byte("<conjunction:unadorned>")
var OptimizeTFRConjunctionUnadornedField = "*"

// Finish of an unadorned conjunction optimization will compute a
// termFieldReader with an "actual" bitmap that represents the
// constituent bitmaps AND'ed together.  This termFieldReader cannot
// provide any freq-norm or termVector associated information.
func (o *OptimizeTFRConjunctionUnadorned) Finish() (rv index.Optimized, err error) {
	if len(o.tfrs) <= 1 {
		return nil, nil
	}

	// We use an artificial term and field because the optimized
	// termFieldReader can represent multiple terms and fields.
	oTFR := &IndexSnapshotTermFieldReader{
		term:               OptimizeTFRConjunctionUnadornedTerm,
		field:              OptimizeTFRConjunctionUnadornedField,
		snapshot:           o.snapshot,
		iterators:          make([]segment.PostingsIterator, len(o.snapshot.segment)),
		segmentOffset:      0,
		includeFreq:        false,
		includeNorm:        false,
		includeTermVectors: false,
	}

	var actualBMs []*roaring.Bitmap // Collected from regular posting lists.

OUTER:
	for i := range o.snapshot.segment {
		actualBMs = actualBMs[:0]

		var docNum1HitLast uint64
		var docNum1HitLastOk bool

		for _, tfr := range o.tfrs {
			if _, ok := tfr.iterators[i].(*segment.EmptyPostingsIterator); ok {
				// An empty postings iterator means the entire AND is empty.
				oTFR.iterators[i] = segment.AnEmptyPostingsIterator
				continue OUTER
			}

			itr, ok := tfr.iterators[i].(*zap.PostingsIterator)
			if !ok {
				// We optimize zap postings iterators only.
				return nil, nil
			}

			// If the postings iterator is "1-hit" optimized, then we
			// can perform several optimizations up-front here.
			docNum1Hit, ok := itr.DocNum1Hit()
			if ok {
				if docNum1Hit == zap.DocNum1HitFinished {
					// An empty docNum here means the entire AND is empty.
					oTFR.iterators[i] = segment.AnEmptyPostingsIterator
					continue OUTER
				}

				if docNum1HitLastOk && docNum1HitLast != docNum1Hit {
					// The docNum1Hit doesn't match the previous
					// docNum1HitLast, so the entire AND is empty.
					oTFR.iterators[i] = segment.AnEmptyPostingsIterator
					continue OUTER
				}

				docNum1HitLast = docNum1Hit
				docNum1HitLastOk = true

				continue
			}

			if itr.ActualBM == nil {
				// An empty actual bitmap means the entire AND is empty.
				oTFR.iterators[i] = segment.AnEmptyPostingsIterator
				continue OUTER
			}

			// Collect the actual bitmap for more processing later.
			actualBMs = append(actualBMs, itr.ActualBM)
		}

		if docNum1HitLastOk {
			// We reach here if all the 1-hit optimized posting
			// iterators had the same 1-hit docNum, so we can check if
			// our collected actual bitmaps also have that docNum.
			for _, bm := range actualBMs {
				if !bm.Contains(uint32(docNum1HitLast)) {
					// The docNum1Hit isn't in one of our actual
					// bitmaps, so the entire AND is empty.
					oTFR.iterators[i] = segment.AnEmptyPostingsIterator
					continue OUTER
				}
			}

			// The actual bitmaps and docNum1Hits all contain or have
			// the same 1-hit docNum, so that's our AND'ed result.
			oTFR.iterators[i], err = zap.PostingsIteratorFrom1Hit(
				docNum1HitLast, zap.NormBits1Hit, false, false)
			if err != nil {
				return nil, nil
			}

			continue OUTER
		}

		if len(actualBMs) == 0 {
			// If we've collected no actual bitmaps at this point,
			// then the entire AND is empty.
			oTFR.iterators[i] = segment.AnEmptyPostingsIterator
			continue OUTER
		}

		if len(actualBMs) == 1 {
			// If we've only 1 actual bitmap, then that's our result.
			oTFR.iterators[i], err = zap.PostingsIteratorFromBitmap(
				actualBMs[0], false, false)
			if err != nil {
				return nil, nil
			}

			continue OUTER
		}

		// Else, AND together our collected bitmaps as our result.
		bm := roaring.And(actualBMs[0], actualBMs[1])

		for _, actualBM := range actualBMs[2:] {
			bm.And(actualBM)
		}

		oTFR.iterators[i], err = zap.PostingsIteratorFromBitmap(
			bm, false, false)
		if err != nil {
			return nil, nil
		}
	}

	return oTFR, nil
}

// ----------------------------------------------------------------

// An "unadorned" disjunction optimization is appropriate when
// additional or subsidiary information like freq-norm's and
// term-vectors are not required, and instead only the internal-id's
// are needed.
func (s *IndexSnapshotTermFieldReader) optimizeDisjunctionUnadorned(
	octx index.OptimizableContext) (index.OptimizableContext, error) {
	if octx == nil {
		octx = &OptimizeTFRDisjunctionUnadorned{snapshot: s.snapshot}
	}

	o, ok := octx.(*OptimizeTFRDisjunctionUnadorned)
	if !ok {
		return nil, nil
	}

	if o.snapshot != s.snapshot {
		return nil, fmt.Errorf("tried to optimize unadorned disjunction across different snapshots")
	}

	o.tfrs = append(o.tfrs, s)

	return o, nil
}

type OptimizeTFRDisjunctionUnadorned struct {
	snapshot *IndexSnapshot

	tfrs []*IndexSnapshotTermFieldReader
}

var OptimizeTFRDisjunctionUnadornedTerm = []byte("<disjunction:unadorned>")
var OptimizeTFRDisjunctionUnadornedField = "*"

// Finish of an unadorned disjunction optimization will compute a
// termFieldReader with an "actual" bitmap that represents the
// constituent bitmaps OR'ed together.  This termFieldReader cannot
// provide any freq-norm or termVector associated information.
func (o *OptimizeTFRDisjunctionUnadorned) Finish() (rv index.Optimized, err error) {
	if len(o.tfrs) <= 1 {
		return nil, nil
	}

	for i := range o.snapshot.segment {
		var cMax uint64

		for _, tfr := range o.tfrs {
			itr, ok := tfr.iterators[i].(*zap.PostingsIterator)
			if !ok {
				return nil, nil
			}

			if itr.ActualBM != nil {
				c := itr.ActualBM.GetCardinality()
				if cMax < c {
					cMax = c
				}
			}
		}

		// Heuristic to skip the optimization if all the constituent
		// bitmaps are too small, where the processing & resource
		// overhead to create the OR'ed bitmap outweighs the benefit.
		if cMax < OptimizeDisjunctionUnadornedMinChildCardinality {
			return nil, nil
		}
	}

	// We use an artificial term and field because the optimized
	// termFieldReader can represent multiple terms and fields.
	oTFR := &IndexSnapshotTermFieldReader{
		term:               OptimizeTFRDisjunctionUnadornedTerm,
		field:              OptimizeTFRDisjunctionUnadornedField,
		snapshot:           o.snapshot,
		iterators:          make([]segment.PostingsIterator, len(o.snapshot.segment)),
		segmentOffset:      0,
		includeFreq:        false,
		includeNorm:        false,
		includeTermVectors: false,
	}

	var docNums []uint32            // Collected docNum's from 1-hit posting lists.
	var actualBMs []*roaring.Bitmap // Collected from regular posting lists.

	for i := range o.snapshot.segment {
		docNums = docNums[:0]
		actualBMs = actualBMs[:0]

		for _, tfr := range o.tfrs {
			itr, ok := tfr.iterators[i].(*zap.PostingsIterator)
			if !ok {
				return nil, nil
			}

			docNum, ok := itr.DocNum1Hit()
			if ok {
				docNums = append(docNums, uint32(docNum))
				continue
			}

			if itr.ActualBM != nil {
				actualBMs = append(actualBMs, itr.ActualBM)
			}
		}

		var bm *roaring.Bitmap
		if len(actualBMs) > 2 {
			bm = roaring.HeapOr(actualBMs...)
		} else if len(actualBMs) == 2 {
			bm = roaring.Or(actualBMs[0], actualBMs[1])
		} else if len(actualBMs) == 1 {
			bm = actualBMs[0].Clone()
		}

		if bm == nil {
			bm = roaring.New()
		}

		bm.AddMany(docNums)

		oTFR.iterators[i], err = zap.PostingsIteratorFromBitmap(bm, false, false)
		if err != nil {
			return nil, nil
		}
	}

	return oTFR, nil
}
