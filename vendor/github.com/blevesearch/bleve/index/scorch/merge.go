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

package scorch

import (
	"bytes"
	"encoding/json"

	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/index/scorch/mergeplan"
	"github.com/blevesearch/bleve/index/scorch/segment"
	"github.com/blevesearch/bleve/index/scorch/segment/zap"
)

func (s *Scorch) mergerLoop() {
	var lastEpochMergePlanned uint64
	mergePlannerOptions, err := s.parseMergePlannerOptions()
	if err != nil {
		s.fireAsyncError(fmt.Errorf("mergePlannerOption json parsing err: %v", err))
		s.asyncTasks.Done()
		return
	}

OUTER:
	for {
		select {
		case <-s.closeCh:
			break OUTER

		default:
			// check to see if there is a new snapshot to persist
			s.rootLock.RLock()
			ourSnapshot := s.root
			ourSnapshot.AddRef()
			s.rootLock.RUnlock()

			if ourSnapshot.epoch != lastEpochMergePlanned {
				startTime := time.Now()

				// lets get started
				err := s.planMergeAtSnapshot(ourSnapshot, mergePlannerOptions)
				if err != nil {
					s.fireAsyncError(fmt.Errorf("merging err: %v", err))
					_ = ourSnapshot.DecRef()
					continue OUTER
				}
				lastEpochMergePlanned = ourSnapshot.epoch

				s.fireEvent(EventKindMergerProgress, time.Since(startTime))
			}
			_ = ourSnapshot.DecRef()

			// tell the persister we're waiting for changes
			// first make a epochWatcher chan
			ew := &epochWatcher{
				epoch:    lastEpochMergePlanned,
				notifyCh: make(notificationChan, 1),
			}

			// give it to the persister
			select {
			case <-s.closeCh:
				break OUTER
			case s.persisterNotifier <- ew:
			}

			// now wait for persister (but also detect close)
			select {
			case <-s.closeCh:
				break OUTER
			case <-ew.notifyCh:
			}
		}
	}
	s.asyncTasks.Done()
}

func (s *Scorch) parseMergePlannerOptions() (*mergeplan.MergePlanOptions,
	error) {
	mergePlannerOptions := mergeplan.DefaultMergePlanOptions
	if v, ok := s.config["scorchMergePlanOptions"]; ok {
		b, err := json.Marshal(v)
		if err != nil {
			return &mergePlannerOptions, err
		}

		err = json.Unmarshal(b, &mergePlannerOptions)
		if err != nil {
			return &mergePlannerOptions, err
		}
	}
	return &mergePlannerOptions, nil
}

func (s *Scorch) planMergeAtSnapshot(ourSnapshot *IndexSnapshot,
	options *mergeplan.MergePlanOptions) error {
	// build list of zap segments in this snapshot
	var onlyZapSnapshots []mergeplan.Segment
	for _, segmentSnapshot := range ourSnapshot.segment {
		if _, ok := segmentSnapshot.segment.(*zap.Segment); ok {
			onlyZapSnapshots = append(onlyZapSnapshots, segmentSnapshot)
		}
	}

	// give this list to the planner
	resultMergePlan, err := mergeplan.Plan(onlyZapSnapshots, options)
	if err != nil {
		return fmt.Errorf("merge planning err: %v", err)
	}
	if resultMergePlan == nil {
		// nothing to do
		return nil
	}

	// process tasks in serial for now
	var notifications []chan *IndexSnapshot
	for _, task := range resultMergePlan.Tasks {
		if len(task.Segments) == 0 {
			continue
		}

		oldMap := make(map[uint64]*SegmentSnapshot)
		newSegmentID := atomic.AddUint64(&s.nextSegmentID, 1)
		segmentsToMerge := make([]*zap.Segment, 0, len(task.Segments))
		docsToDrop := make([]*roaring.Bitmap, 0, len(task.Segments))
		for _, planSegment := range task.Segments {
			if segSnapshot, ok := planSegment.(*SegmentSnapshot); ok {
				oldMap[segSnapshot.id] = segSnapshot
				if zapSeg, ok := segSnapshot.segment.(*zap.Segment); ok {
					if segSnapshot.LiveSize() == 0 {
						oldMap[segSnapshot.id] = nil
					} else {
						segmentsToMerge = append(segmentsToMerge, zapSeg)
						docsToDrop = append(docsToDrop, segSnapshot.deleted)
					}
				}
			}
		}

		var oldNewDocNums map[uint64][]uint64
		var segment segment.Segment
		if len(segmentsToMerge) > 0 {
			filename := zapFileName(newSegmentID)
			s.markIneligibleForRemoval(filename)
			path := s.path + string(os.PathSeparator) + filename
			newDocNums, err := zap.Merge(segmentsToMerge, docsToDrop, path, 1024)
			if err != nil {
				s.unmarkIneligibleForRemoval(filename)
				return fmt.Errorf("merging failed: %v", err)
			}
			segment, err = zap.Open(path)
			if err != nil {
				s.unmarkIneligibleForRemoval(filename)
				return err
			}
			oldNewDocNums = make(map[uint64][]uint64)
			for i, segNewDocNums := range newDocNums {
				oldNewDocNums[task.Segments[i].Id()] = segNewDocNums
			}
		}

		sm := &segmentMerge{
			id:            newSegmentID,
			old:           oldMap,
			oldNewDocNums: oldNewDocNums,
			new:           segment,
			notify:        make(chan *IndexSnapshot, 1),
		}
		notifications = append(notifications, sm.notify)

		// give it to the introducer
		select {
		case <-s.closeCh:
			_ = segment.Close()
			return nil
		case s.merges <- sm:
		}
	}
	for _, notification := range notifications {
		select {
		case <-s.closeCh:
			return nil
		case newSnapshot := <-notification:
			if newSnapshot != nil {
				_ = newSnapshot.DecRef()
			}
		}
	}
	return nil
}

type segmentMerge struct {
	id            uint64
	old           map[uint64]*SegmentSnapshot
	oldNewDocNums map[uint64][]uint64
	new           segment.Segment
	notify        chan *IndexSnapshot
}

// perform a merging of the given SegmentBase instances into a new,
// persisted segment, and synchronously introduce that new segment
// into the root
func (s *Scorch) mergeSegmentBases(snapshot *IndexSnapshot,
	sbs []*zap.SegmentBase, sbsDrops []*roaring.Bitmap, sbsIndexes []int,
	chunkFactor uint32) (uint64, *IndexSnapshot, uint64, error) {
	var br bytes.Buffer

	cr := zap.NewCountHashWriter(&br)

	newDocNums, numDocs, storedIndexOffset, fieldsIndexOffset,
		docValueOffset, dictLocs, fieldsInv, fieldsMap, err :=
		zap.MergeToWriter(sbs, sbsDrops, chunkFactor, cr)
	if err != nil {
		return 0, nil, 0, err
	}

	sb, err := zap.InitSegmentBase(br.Bytes(), cr.Sum32(), chunkFactor,
		fieldsMap, fieldsInv, numDocs, storedIndexOffset, fieldsIndexOffset,
		docValueOffset, dictLocs)
	if err != nil {
		return 0, nil, 0, err
	}

	newSegmentID := atomic.AddUint64(&s.nextSegmentID, 1)

	filename := zapFileName(newSegmentID)
	path := s.path + string(os.PathSeparator) + filename
	err = zap.PersistSegmentBase(sb, path)
	if err != nil {
		return 0, nil, 0, err
	}

	segment, err := zap.Open(path)
	if err != nil {
		return 0, nil, 0, err
	}

	sm := &segmentMerge{
		id:            newSegmentID,
		old:           make(map[uint64]*SegmentSnapshot),
		oldNewDocNums: make(map[uint64][]uint64),
		new:           segment,
		notify:        make(chan *IndexSnapshot, 1),
	}

	for i, idx := range sbsIndexes {
		ss := snapshot.segment[idx]
		sm.old[ss.id] = ss
		sm.oldNewDocNums[ss.id] = newDocNums[i]
	}

	select { // send to introducer
	case <-s.closeCh:
		_ = segment.DecRef()
		return 0, nil, 0, nil // TODO: return ErrInterruptedClosed?
	case s.merges <- sm:
	}

	select { // wait for introduction to complete
	case <-s.closeCh:
		return 0, nil, 0, nil // TODO: return ErrInterruptedClosed?
	case newSnapshot := <-sm.notify:
		return numDocs, newSnapshot, newSegmentID, nil
	}
}
