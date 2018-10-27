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
	"github.com/blevesearch/bleve/index/scorch/segment/zap"
)

func (s *IndexSnapshotTermFieldReader) Optimize(kind string, octx index.OptimizableContext) (
	index.OptimizableContext, error) {
	if kind != "conjunction" {
		return octx, nil
	}

	if octx == nil {
		octx = &OptimizeTFRConjunction{snapshot: s.snapshot}
	}

	o, ok := octx.(*OptimizeTFRConjunction)
	if !ok {
		return octx, nil
	}

	if o.snapshot != s.snapshot {
		return nil, fmt.Errorf("tried to optimize across different snapshots")
	}

	o.tfrs = append(o.tfrs, s)

	return o, nil
}

type OptimizeTFRConjunction struct {
	snapshot *IndexSnapshot

	tfrs []*IndexSnapshotTermFieldReader
}

func (o *OptimizeTFRConjunction) Finish() error {
	if len(o.tfrs) <= 1 {
		return nil
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

		for _, tfr := range o.tfrs {
			itr, ok := tfr.iterators[i].(*zap.PostingsIterator)
			if ok && itr.ActualBM != nil {
				itr.ActualBM = bm
				itr.Actual = bm.Iterator()
			}
		}
	}

	return nil
}
