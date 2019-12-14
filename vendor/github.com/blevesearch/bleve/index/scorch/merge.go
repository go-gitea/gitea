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
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
		atomic.AddUint64(&s.stats.TotFileMergeLoopBeg, 1)

		select {
		case <-s.closeCh:
			break OUTER

		default:
			// check to see if there is a new snapshot to persist
			s.rootLock.Lock()
			ourSnapshot := s.root
			ourSnapshot.AddRef()
			atomic.StoreUint64(&s.iStats.mergeSnapshotSize, uint64(ourSnapshot.Size()))
			atomic.StoreUint64(&s.iStats.mergeEpoch, ourSnapshot.epoch)
			s.rootLock.Unlock()

			if ourSnapshot.epoch != lastEpochMergePlanned {
				startTime := time.Now()

				// lets get started
				err := s.planMergeAtSnapshot(ourSnapshot, mergePlannerOptions)
				if err != nil {
					atomic.StoreUint64(&s.iStats.mergeEpoch, 0)
					if err == segment.ErrClosed {
						// index has been closed
						_ = ourSnapshot.DecRef()
						break OUTER
					}
					s.fireAsyncError(fmt.Errorf("merging err: %v", err))
					_ = ourSnapshot.DecRef()
					atomic.AddUint64(&s.stats.TotFileMergeLoopErr, 1)
					continue OUTER
				}
				lastEpochMergePlanned = ourSnapshot.epoch

				atomic.StoreUint64(&s.stats.LastMergedEpoch, ourSnapshot.epoch)

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

		atomic.AddUint64(&s.stats.TotFileMergeLoopEnd, 1)
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

		err = mergeplan.ValidateMergePlannerOptions(&mergePlannerOptions)
		if err != nil {
			return nil, err
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

	atomic.AddUint64(&s.stats.TotFileMergePlan, 1)

	// give this list to the planner
	resultMergePlan, err := mergeplan.Plan(onlyZapSnapshots, options)
	if err != nil {
		atomic.AddUint64(&s.stats.TotFileMergePlanErr, 1)
		return fmt.Errorf("merge planning err: %v", err)
	}
	if resultMergePlan == nil {
		// nothing to do
		atomic.AddUint64(&s.stats.TotFileMergePlanNone, 1)
		return nil
	}
	atomic.AddUint64(&s.stats.TotFileMergePlanOk, 1)

	atomic.AddUint64(&s.stats.TotFileMergePlanTasks, uint64(len(resultMergePlan.Tasks)))

	// process tasks in serial for now
	var notifications []chan *IndexSnapshot
	var filenames []string
	for _, task := range resultMergePlan.Tasks {
		if len(task.Segments) == 0 {
			atomic.AddUint64(&s.stats.TotFileMergePlanTasksSegmentsEmpty, 1)
			continue
		}

		atomic.AddUint64(&s.stats.TotFileMergePlanTasksSegments, uint64(len(task.Segments)))

		oldMap := make(map[uint64]*SegmentSnapshot)
		newSegmentID := atomic.AddUint64(&s.nextSegmentID, 1)
		segmentsToMerge := make([]*zap.Segment, 0, len(task.Segments))
		docsToDrop := make([]*roaring.Bitmap, 0, len(task.Segments))

		for _, planSegment := range task.Segments {
			if segSnapshot, ok := planSegment.(*SegmentSnapshot); ok {
				oldMap[segSnapshot.id] = segSnapshot
				if zapSeg, ok := segSnapshot.segment.(*zap.Segment); ok {
					if segSnapshot.LiveSize() == 0 {
						atomic.AddUint64(&s.stats.TotFileMergeSegmentsEmpty, 1)
						oldMap[segSnapshot.id] = nil
					} else {
						segmentsToMerge = append(segmentsToMerge, zapSeg)
						docsToDrop = append(docsToDrop, segSnapshot.deleted)
					}
					// track the files getting merged for unsetting the
					// removal ineligibility. This helps to unflip files
					// even with fast merger, slow persister work flows.
					path := zapSeg.Path()
					filenames = append(filenames,
						strings.TrimPrefix(path, s.path+string(os.PathSeparator)))
				}
			}
		}

		var oldNewDocNums map[uint64][]uint64
		var seg segment.Segment
		if len(segmentsToMerge) > 0 {
			filename := zapFileName(newSegmentID)
			s.markIneligibleForRemoval(filename)
			path := s.path + string(os.PathSeparator) + filename

			fileMergeZapStartTime := time.Now()

			atomic.AddUint64(&s.stats.TotFileMergeZapBeg, 1)
			newDocNums, _, err := zap.Merge(segmentsToMerge, docsToDrop, path,
				DefaultChunkFactor, s.closeCh, s)
			atomic.AddUint64(&s.stats.TotFileMergeZapEnd, 1)

			fileMergeZapTime := uint64(time.Since(fileMergeZapStartTime))
			atomic.AddUint64(&s.stats.TotFileMergeZapTime, fileMergeZapTime)
			if atomic.LoadUint64(&s.stats.MaxFileMergeZapTime) < fileMergeZapTime {
				atomic.StoreUint64(&s.stats.MaxFileMergeZapTime, fileMergeZapTime)
			}

			if err != nil {
				s.unmarkIneligibleForRemoval(filename)
				atomic.AddUint64(&s.stats.TotFileMergePlanTasksErr, 1)
				if err == segment.ErrClosed {
					return err
				}
				return fmt.Errorf("merging failed: %v", err)
			}

			seg, err = zap.Open(path)
			if err != nil {
				s.unmarkIneligibleForRemoval(filename)
				atomic.AddUint64(&s.stats.TotFileMergePlanTasksErr, 1)
				return err
			}
			err = zap.ValidateMerge(segmentsToMerge, nil, docsToDrop, seg.(*zap.Segment))
			if err != nil {
				s.unmarkIneligibleForRemoval(filename)
				return fmt.Errorf("merge validation failed: %v", err)
			}
			oldNewDocNums = make(map[uint64][]uint64)
			for i, segNewDocNums := range newDocNums {
				oldNewDocNums[task.Segments[i].Id()] = segNewDocNums
			}

			atomic.AddUint64(&s.stats.TotFileMergeSegments, uint64(len(segmentsToMerge)))
		}

		sm := &segmentMerge{
			id:            newSegmentID,
			old:           oldMap,
			oldNewDocNums: oldNewDocNums,
			new:           seg,
			notify:        make(chan *IndexSnapshot, 1),
		}
		notifications = append(notifications, sm.notify)

		// give it to the introducer
		select {
		case <-s.closeCh:
			_ = seg.Close()
			return segment.ErrClosed
		case s.merges <- sm:
			atomic.AddUint64(&s.stats.TotFileMergeIntroductions, 1)
		}

		atomic.AddUint64(&s.stats.TotFileMergePlanTasksDone, 1)
	}

	for _, notification := range notifications {
		select {
		case <-s.closeCh:
			atomic.AddUint64(&s.stats.TotFileMergeIntroductionsSkipped, 1)
			return segment.ErrClosed
		case newSnapshot := <-notification:
			atomic.AddUint64(&s.stats.TotFileMergeIntroductionsDone, 1)
			if newSnapshot != nil {
				_ = newSnapshot.DecRef()
			}
		}
	}

	// once all the newly merged segment introductions are done,
	// its safe to unflip the removal ineligibility for the replaced
	// older segments
	for _, f := range filenames {
		s.unmarkIneligibleForRemoval(f)
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
	chunkFactor uint32) (*IndexSnapshot, uint64, error) {
	atomic.AddUint64(&s.stats.TotMemMergeBeg, 1)

	memMergeZapStartTime := time.Now()

	atomic.AddUint64(&s.stats.TotMemMergeZapBeg, 1)

	newSegmentID := atomic.AddUint64(&s.nextSegmentID, 1)
	filename := zapFileName(newSegmentID)
	path := s.path + string(os.PathSeparator) + filename

	newDocNums, _, err :=
		zap.MergeSegmentBases(sbs, sbsDrops, path, chunkFactor, s.closeCh, s)

	atomic.AddUint64(&s.stats.TotMemMergeZapEnd, 1)

	memMergeZapTime := uint64(time.Since(memMergeZapStartTime))
	atomic.AddUint64(&s.stats.TotMemMergeZapTime, memMergeZapTime)
	if atomic.LoadUint64(&s.stats.MaxMemMergeZapTime) < memMergeZapTime {
		atomic.StoreUint64(&s.stats.MaxMemMergeZapTime, memMergeZapTime)
	}

	if err != nil {
		atomic.AddUint64(&s.stats.TotMemMergeErr, 1)
		return nil, 0, err
	}

	seg, err := zap.Open(path)
	if err != nil {
		atomic.AddUint64(&s.stats.TotMemMergeErr, 1)
		return nil, 0, err
	}
	err = zap.ValidateMerge(nil, sbs, sbsDrops, seg.(*zap.Segment))
	if err != nil {
		return nil, 0, fmt.Errorf("in-memory merge validation failed: %v", err)
	}

	// update persisted stats
	atomic.AddUint64(&s.stats.TotPersistedItems, seg.Count())
	atomic.AddUint64(&s.stats.TotPersistedSegments, 1)

	sm := &segmentMerge{
		id:            newSegmentID,
		old:           make(map[uint64]*SegmentSnapshot),
		oldNewDocNums: make(map[uint64][]uint64),
		new:           seg,
		notify:        make(chan *IndexSnapshot, 1),
	}

	for i, idx := range sbsIndexes {
		ss := snapshot.segment[idx]
		sm.old[ss.id] = ss
		sm.oldNewDocNums[ss.id] = newDocNums[i]
	}

	select { // send to introducer
	case <-s.closeCh:
		_ = seg.DecRef()
		return nil, 0, segment.ErrClosed
	case s.merges <- sm:
	}

	select { // wait for introduction to complete
	case <-s.closeCh:
		return nil, 0, segment.ErrClosed
	case newSnapshot := <-sm.notify:
		atomic.AddUint64(&s.stats.TotMemMergeSegments, uint64(len(sbs)))
		atomic.AddUint64(&s.stats.TotMemMergeDone, 1)
		return newSnapshot, newSegmentID, nil
	}
}

func (s *Scorch) ReportBytesWritten(bytesWritten uint64) {
	atomic.AddUint64(&s.stats.TotFileMergeWrittenBytes, bytesWritten)
}
