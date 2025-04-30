//  Copyright (c) 2020 Couchbase, Inc.
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
	"github.com/RoaringBitmap/roaring"
	"math"
	"reflect"
)

var reflectStaticSizeUnadornedPostingsIteratorBitmap int
var reflectStaticSizeUnadornedPostingsIterator1Hit int
var reflectStaticSizeUnadornedPosting int

func init() {
	var pib UnadornedPostingsIteratorBitmap
	reflectStaticSizeUnadornedPostingsIteratorBitmap = int(reflect.TypeOf(pib).Size())
	var pi1h UnadornedPostingsIterator1Hit
	reflectStaticSizeUnadornedPostingsIterator1Hit = int(reflect.TypeOf(pi1h).Size())
	var up UnadornedPosting
	reflectStaticSizeUnadornedPosting = int(reflect.TypeOf(up).Size())
}

type UnadornedPostingsIteratorBitmap struct {
	actual   roaring.IntPeekable
	actualBM *roaring.Bitmap
}

func (i *UnadornedPostingsIteratorBitmap) Next() (Posting, error) {
	return i.nextAtOrAfter(0)
}

func (i *UnadornedPostingsIteratorBitmap) Advance(docNum uint64) (Posting, error) {
	return i.nextAtOrAfter(docNum)
}

func (i *UnadornedPostingsIteratorBitmap) nextAtOrAfter(atOrAfter uint64) (Posting, error) {
	docNum, exists := i.nextDocNumAtOrAfter(atOrAfter)
	if !exists {
		return nil, nil
	}
	return UnadornedPosting(docNum), nil
}

func (i *UnadornedPostingsIteratorBitmap) nextDocNumAtOrAfter(atOrAfter uint64) (uint64, bool) {
	if i.actual == nil || !i.actual.HasNext() {
		return 0, false
	}
	i.actual.AdvanceIfNeeded(uint32(atOrAfter))

	if !i.actual.HasNext() {
		return 0, false // couldn't find anything
	}

	return uint64(i.actual.Next()), true
}

func (i *UnadornedPostingsIteratorBitmap) Size() int {
	return reflectStaticSizeUnadornedPostingsIteratorBitmap
}

func (i *UnadornedPostingsIteratorBitmap) ActualBitmap() *roaring.Bitmap {
	return i.actualBM
}

func (i *UnadornedPostingsIteratorBitmap) DocNum1Hit() (uint64, bool) {
	return 0, false
}

func (i *UnadornedPostingsIteratorBitmap) ReplaceActual(actual *roaring.Bitmap) {
	i.actualBM = actual
	i.actual = actual.Iterator()
}

func NewUnadornedPostingsIteratorFromBitmap(bm *roaring.Bitmap) PostingsIterator {
	return &UnadornedPostingsIteratorBitmap{
		actualBM: bm,
		actual:   bm.Iterator(),
	}
}

const docNum1HitFinished = math.MaxUint64

type UnadornedPostingsIterator1Hit struct {
	docNum uint64
}

func (i *UnadornedPostingsIterator1Hit) Next() (Posting, error) {
	return i.nextAtOrAfter(0)
}

func (i *UnadornedPostingsIterator1Hit) Advance(docNum uint64) (Posting, error) {
	return i.nextAtOrAfter(docNum)
}

func (i *UnadornedPostingsIterator1Hit) nextAtOrAfter(atOrAfter uint64) (Posting, error) {
	docNum, exists := i.nextDocNumAtOrAfter(atOrAfter)
	if !exists {
		return nil, nil
	}
	return UnadornedPosting(docNum), nil
}

func (i *UnadornedPostingsIterator1Hit) nextDocNumAtOrAfter(atOrAfter uint64) (uint64, bool) {
	if i.docNum == docNum1HitFinished {
		return 0, false
	}
	if i.docNum < atOrAfter {
		// advanced past our 1-hit
		i.docNum = docNum1HitFinished // consume our 1-hit docNum
		return 0, false
	}
	docNum := i.docNum
	i.docNum = docNum1HitFinished // consume our 1-hit docNum
	return docNum, true
}

func (i *UnadornedPostingsIterator1Hit) Size() int {
	return reflectStaticSizeUnadornedPostingsIterator1Hit
}

func NewUnadornedPostingsIteratorFrom1Hit(docNum1Hit uint64) PostingsIterator {
	return &UnadornedPostingsIterator1Hit{
		docNum1Hit,
	}
}

type UnadornedPosting uint64

func (p UnadornedPosting) Number() uint64 {
	return uint64(p)
}

func (p UnadornedPosting) Frequency() uint64 {
	return 0
}

func (p UnadornedPosting) Norm() float64 {
	return 0
}

func (p UnadornedPosting) Locations() []Location {
	return nil
}

func (p UnadornedPosting) Size() int {
	return reflectStaticSizeUnadornedPosting
}
