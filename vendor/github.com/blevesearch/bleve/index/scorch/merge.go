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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/index/scorch/mergeplan"
	"github.com/blevesearch/bleve/index/scorch/segment"
)

func (s *Scorch) mergerLoop() {
	var lastEpochMergePlanned uint64
	var ctrlMsg *mergerCtrl
	mergePlannerOptions, err := s.parseMergePlannerOptions()
	if err != nil {
		s.fireAsyncError(fmt.Errorf("mergePlannerOption json parsing err: %v", err))
		s.asyncTasks.Done()
		return
	}
	ctrlMsgDflt := &mergerCtrl{ctx: context.Background(),
		options: mergePlannerOptions,
		doneCh:  nil}

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

			if ctrlMsg == nil && ourSnapshot.epoch != lastEpochMergePlanned {
				ctrlMsg = ctrlMsgDflt
			}
			if ctrlMsg != nil {
				startTime := time.Now()

				// lets get started
				err := s.planMergeAtSnapshot(ctrlMsg.ctx, ctrlMsg.options,
					ourSnapshot)
				if err != nil {
					atomic.StoreUint64(&s.iStats.mergeEpoch, 0)
					if err == segment.ErrClosed {
						// index has been closed
						_ = ourSnapshot.DecRef()

						// continue the workloop on a user triggered cancel
						if ctrlMsg.doneCh != nil {
							close(ctrlMsg.doneCh)
							ctrlMsg = nil
							continue OUTER
						}

						// exit the workloop on index closure
						ctrlMsg = nil
						break OUTER
					}
					s.fireAsyncError(fmt.Errorf("merging err: %v", err))
					_ = ourSnapshot.DecRef()
					atomic.AddUint64(&s.stats.TotFileMergeLoopErr, 1)
					continue OUTER
				}

				if ctrlMsg.doneCh != nil {
					close(ctrlMsg.doneCh)
				}
				ctrlMsg = nil

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
			case ctrlMsg = <-s.forceMergeRequestCh:
				continue OUTER
			}

			// now wait for persister (but also detect close)
			select {
			case <-s.closeCh:
				break OUTER
			case <-ew.notifyCh:
			case ctrlMsg = <-s.forceMergeRequestCh:
			}
		}

		atomic.AddUint64(&s.stats.TotFileMergeLoopEnd, 1)
	}

	s.asyncTasks.Done()
}

type mergerCtrl struct {
	ctx     context.Context
	options *mergeplan.MergePlanOptions
	doneCh  chan struct{}
}

// ForceMerge helps users trigger a merge operation on
// an online scorch index.
func (s *Scorch) ForceMerge(ctx context.Context,
	mo *mergeplan.MergePlanOptions) error {
	// check whether force merge is already under processing
	s.rootLock.Lock()
	if s.stats.TotFileMergeForceOpsStarted >
		s.stats.TotFileMergeForceOpsCompleted {
		s.rootLock.Unlock()
		return fmt.Errorf("force merge already in progress")
	}

	s.stats.TotFileMergeForceOpsStarted++
	s.rootLock.Unlock()

	if mo != nil {
		err := mergeplan.ValidateMergePlannerOptions(mo)
		if err != nil {
			return err
		}
	} else {
		// assume the default single segment merge policy
		mo = &mergeplan.SingleSegmentMergePlanOptions
	}
	msg := &mergerCtrl{options: mo,
		doneCh: make(chan struct{}),
		ctx:    ctx,
	}

	// request the merger perform a force merge
	select {
	case s.forceMergeRequestCh <- msg:
	case <-s.closeCh:
		return nil
	}

	// wait for the force merge operation completion
	select {
	case <-msg.doneCh:
		atomic.AddUint64(&s.stats.TotFileMergeForceOpsCompleted, 1)
	case <-s.closeCh:
	}

	return nil
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

type closeChWrapper struct {
	ch1     chan struct{}
	ctx     context.Context
	closeCh chan struct{}
}

func newCloseChWrapper(ch1 chan struct{},
	ctx context.Context) *closeChWrapper {
	return &closeChWrapper{ch1: ch1,
		ctx:     ctx,
		closeCh: make(chan struct{})}
}

func (w *closeChWrapper) close() {
	select {
	case <-w.closeCh:
	default:
		close(w.closeCh)
	}
}

func (w *closeChWrapper) listen() {
	select {
	case <-w.ch1:
		w.close()
	case <-w.ctx.Done():
		w.close()
	case <-w.closeCh:
	}
}

func (s *Scorch) planMergeAtSnapshot(ctx context.Context,
	options *mergeplan.MergePlanOptions, ourSnapshot *IndexSnapshot) error {
	// build list of persisted segments in this snapshot
	var onlyPersistedSnapshots []mergeplan.Segment
	for _, segmentSnapshot := range ourSnapshot.segment {
		if _, ok := segmentSnapshot.segment.(segment.PersistedSegment); ok {
			onlyPersistedSnapshots = append(onlyPersistedSnapshots, segmentSnapshot)
		}
	}

	atomic.AddUint64(&s.stats.TotFileMergePlan, 1)

	// give this list to the planner
	resultMergePlan, err := mergeplan.Plan(onlyPersistedSnapshots, options)
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
	var filenames []string

	cw := newCloseChWrapper(s.closeCh, ctx)
	defer cw.close()

	go cw.listen()

	for _, task := range resultMergePlan.Tasks {
		if len(task.Segments) == 0 {
			atomic.AddUint64(&s.stats.TotFileMergePlanTasksSegmentsEmpty, 1)
			continue
		}

		atomic.AddUint64(&s.stats.TotFileMergePlanTasksSegments, uint64(len(task.Segments)))

		oldMap := make(map[uint64]*SegmentSnapshot)
		newSegmentID := atomic.AddUint64(&s.nextSegmentID, 1)
		segmentsToMerge := make([]segment.Segment, 0, len(task.Segments))
		docsToDrop := make([]*roaring.Bitmap, 0, len(task.Segments))

		for _, planSegment := range task.Segments {
			if segSnapshot, ok := planSegment.(*SegmentSnapshot); ok {
				oldMap[segSnapshot.id] = segSnapshot
				if persistedSeg, ok := segSnapshot.segment.(segment.PersistedSegment); ok {
					if segSnapshot.LiveSize() == 0 {
						atomic.AddUint64(&s.stats.TotFileMergeSegmentsEmpty, 1)
						oldMap[segSnapshot.id] = nil
					} else {
						segmentsToMerge = append(segmentsToMerge, segSnapshot.segment)
						docsToDrop = append(docsToDrop, segSnapshot.deleted)
					}
					// track the files getting merged for unsetting the
					// removal ineligibility. This helps to unflip files
					// even with fast merger, slow persister work flows.
					path := persistedSeg.Path()
					filenames = append(filenames,
						strings.TrimPrefix(path, s.path+string(os.PathSeparator)))
				}
			}
		}

		var oldNewDocNums map[uint64][]uint64
		var seg segment.Segment
		var filename string
		if len(segmentsToMerge) > 0 {
			filename = zapFileName(newSegmentID)
			s.markIneligibleForRemoval(filename)
			path := s.path + string(os.PathSeparator) + filename

			fileMergeZapStartTime := time.Now()

			atomic.AddUint64(&s.stats.TotFileMergeZapBeg, 1)
			newDocNums, _, err := s.segPlugin.Merge(segmentsToMerge, docsToDrop, path,
				cw.closeCh, s)
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

			seg, err = s.segPlugin.Open(path)
			if err != nil {
				s.unmarkIneligibleForRemoval(filename)
				atomic.AddUint64(&s.stats.TotFileMergePlanTasksErr, 1)
				return err
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
			notifyCh:      make(chan *mergeTaskIntroStatus),
		}

		s.fireEvent(EventKindMergeTaskIntroductionStart, 0)

		// give it to the introducer
		select {
		case <-s.closeCh:
			_ = seg.Close()
			return segment.ErrClosed
		case s.merges <- sm:
			atomic.AddUint64(&s.stats.TotFileMergeIntroductions, 1)
		}

		introStartTime := time.Now()
		// it is safe to blockingly wait for the merge introduction
		// here as the introducer is bound to handle the notify channel.
		introStatus := <-sm.notifyCh
		introTime := uint64(time.Since(introStartTime))
		atomic.AddUint64(&s.stats.TotFileMergeZapIntroductionTime, introTime)
		if atomic.LoadUint64(&s.stats.MaxFileMergeZapIntroductionTime) < introTime {
			atomic.StoreUint64(&s.stats.MaxFileMergeZapIntroductionTime, introTime)
		}
		atomic.AddUint64(&s.stats.TotFileMergeIntroductionsDone, 1)
		if introStatus != nil && introStatus.indexSnapshot != nil {
			_ = introStatus.indexSnapshot.DecRef()
			if introStatus.skipped {
				// close the segment on skipping introduction.
				s.unmarkIneligibleForRemoval(filename)
				_ = seg.Close()
			}
		}

		atomic.AddUint64(&s.stats.TotFileMergePlanTasksDone, 1)

		s.fireEvent(EventKindMergeTaskIntroduction, 0)
	}

	// once all the newly merged segment introductions are done,
	// its safe to unflip the removal ineligibility for the replaced
	// older segments
	for _, f := range filenames {
		s.unmarkIneligibleForRemoval(f)
	}

	return nil
}

type mergeTaskIntroStatus struct {
	indexSnapshot *IndexSnapshot
	skipped       bool
}

type segmentMerge struct {
	id            uint64
	old           map[uint64]*SegmentSnapshot
	oldNewDocNums map[uint64][]uint64
	new           segment.Segment
	notifyCh      chan *mergeTaskIntroStatus
}

// perform a merging of the given SegmentBase instances into a new,
// persisted segment, and synchronously introduce that new segment
// into the root
func (s *Scorch) mergeSegmentBases(snapshot *IndexSnapshot,
	sbs []segment.Segment, sbsDrops []*roaring.Bitmap,
	sbsIndexes []int) (*IndexSnapshot, uint64, error) {
	atomic.AddUint64(&s.stats.TotMemMergeBeg, 1)

	memMergeZapStartTime := time.Now()

	atomic.AddUint64(&s.stats.TotMemMergeZapBeg, 1)

	newSegmentID := atomic.AddUint64(&s.nextSegmentID, 1)
	filename := zapFileName(newSegmentID)
	path := s.path + string(os.PathSeparator) + filename

	newDocNums, _, err :=
		s.segPlugin.Merge(sbs, sbsDrops, path, s.closeCh, s)

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

	seg, err := s.segPlugin.Open(path)
	if err != nil {
		atomic.AddUint64(&s.stats.TotMemMergeErr, 1)
		return nil, 0, err
	}

	// update persisted stats
	atomic.AddUint64(&s.stats.TotPersistedItems, seg.Count())
	atomic.AddUint64(&s.stats.TotPersistedSegments, 1)

	sm := &segmentMerge{
		id:            newSegmentID,
		old:           make(map[uint64]*SegmentSnapshot),
		oldNewDocNums: make(map[uint64][]uint64),
		new:           seg,
		notifyCh:      make(chan *mergeTaskIntroStatus),
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

	// blockingly wait for the introduction to complete
	var newSnapshot *IndexSnapshot
	introStatus := <-sm.notifyCh
	if introStatus != nil && introStatus.indexSnapshot != nil {
		newSnapshot = introStatus.indexSnapshot
		atomic.AddUint64(&s.stats.TotMemMergeSegments, uint64(len(sbs)))
		atomic.AddUint64(&s.stats.TotMemMergeDone, 1)
		if introStatus.skipped {
			// close the segment on skipping introduction.
			_ = newSnapshot.DecRef()
			_ = seg.Close()
			newSnapshot = nil
		}
	}

	return newSnapshot, newSegmentID, nil
}

func (s *Scorch) ReportBytesWritten(bytesWritten uint64) {
	atomic.AddUint64(&s.stats.TotFileMergeWrittenBytes, bytesWritten)
}
