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
	"fmt"
	"sync/atomic"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/index/scorch/segment"
)

type segmentIntroduction struct {
	id        uint64
	data      segment.Segment
	obsoletes map[uint64]*roaring.Bitmap
	ids       []string
	internal  map[string][]byte

	applied           chan error
	persisted         chan error
	persistedCallback index.BatchCallback
}

type persistIntroduction struct {
	persisted map[uint64]segment.Segment
	applied   notificationChan
}

type epochWatcher struct {
	epoch    uint64
	notifyCh notificationChan
}

func (s *Scorch) introducerLoop() {
	var epochWatchers []*epochWatcher
OUTER:
	for {
		atomic.AddUint64(&s.stats.TotIntroduceLoop, 1)

		select {
		case <-s.closeCh:
			break OUTER

		case epochWatcher := <-s.introducerNotifier:
			epochWatchers = append(epochWatchers, epochWatcher)

		case nextMerge := <-s.merges:
			s.introduceMerge(nextMerge)

		case next := <-s.introductions:
			err := s.introduceSegment(next)
			if err != nil {
				continue OUTER
			}

		case persist := <-s.persists:
			s.introducePersist(persist)

		}

		var epochCurr uint64
		s.rootLock.RLock()
		if s.root != nil {
			epochCurr = s.root.epoch
		}
		s.rootLock.RUnlock()
		var epochWatchersNext []*epochWatcher
		for _, w := range epochWatchers {
			if w.epoch < epochCurr {
				close(w.notifyCh)
			} else {
				epochWatchersNext = append(epochWatchersNext, w)
			}
		}
		epochWatchers = epochWatchersNext
	}

	s.asyncTasks.Done()
}

func (s *Scorch) introduceSegment(next *segmentIntroduction) error {
	atomic.AddUint64(&s.stats.TotIntroduceSegmentBeg, 1)
	defer atomic.AddUint64(&s.stats.TotIntroduceSegmentEnd, 1)

	s.rootLock.RLock()
	root := s.root
	root.AddRef()
	s.rootLock.RUnlock()

	defer func() { _ = root.DecRef() }()

	nsegs := len(root.segment)

	// prepare new index snapshot
	newSnapshot := &IndexSnapshot{
		parent:   s,
		segment:  make([]*SegmentSnapshot, 0, nsegs+1),
		offsets:  make([]uint64, 0, nsegs+1),
		internal: make(map[string][]byte, len(root.internal)),
		refs:     1,
		creator:  "introduceSegment",
	}

	// iterate through current segments
	var running uint64
	var docsToPersistCount, memSegments, fileSegments uint64
	for i := range root.segment {
		// see if optimistic work included this segment
		delta, ok := next.obsoletes[root.segment[i].id]
		if !ok {
			var err error
			delta, err = root.segment[i].segment.DocNumbers(next.ids)
			if err != nil {
				next.applied <- fmt.Errorf("error computing doc numbers: %v", err)
				close(next.applied)
				_ = newSnapshot.DecRef()
				return err
			}
		}

		newss := &SegmentSnapshot{
			id:         root.segment[i].id,
			segment:    root.segment[i].segment,
			cachedDocs: root.segment[i].cachedDocs,
			creator:    root.segment[i].creator,
		}

		// apply new obsoletions
		if root.segment[i].deleted == nil {
			newss.deleted = delta
		} else {
			newss.deleted = roaring.Or(root.segment[i].deleted, delta)
		}
		if newss.deleted.IsEmpty() {
			newss.deleted = nil
		}

		// check for live size before copying
		if newss.LiveSize() > 0 {
			newSnapshot.segment = append(newSnapshot.segment, newss)
			root.segment[i].segment.AddRef()
			newSnapshot.offsets = append(newSnapshot.offsets, running)
			running += newss.segment.Count()
		}

		if isMemorySegment(root.segment[i]) {
			docsToPersistCount += root.segment[i].Count()
			memSegments++
		} else {
			fileSegments++
		}
	}

	atomic.StoreUint64(&s.stats.TotItemsToPersist, docsToPersistCount)
	atomic.StoreUint64(&s.stats.TotMemorySegmentsAtRoot, memSegments)
	atomic.StoreUint64(&s.stats.TotFileSegmentsAtRoot, fileSegments)

	// append new segment, if any, to end of the new index snapshot
	if next.data != nil {
		newSegmentSnapshot := &SegmentSnapshot{
			id:         next.id,
			segment:    next.data, // take ownership of next.data's ref-count
			cachedDocs: &cachedDocs{cache: nil},
			creator:    "introduceSegment",
		}
		newSnapshot.segment = append(newSnapshot.segment, newSegmentSnapshot)
		newSnapshot.offsets = append(newSnapshot.offsets, running)

		// increment numItemsIntroduced which tracks the number of items
		// queued for persistence.
		atomic.AddUint64(&s.stats.TotIntroducedItems, newSegmentSnapshot.Count())
		atomic.AddUint64(&s.stats.TotIntroducedSegmentsBatch, 1)
	}
	// copy old values
	for key, oldVal := range root.internal {
		newSnapshot.internal[key] = oldVal
	}
	// set new values and apply deletes
	for key, newVal := range next.internal {
		if newVal != nil {
			newSnapshot.internal[key] = newVal
		} else {
			delete(newSnapshot.internal, key)
		}
	}

	newSnapshot.updateSize()
	s.rootLock.Lock()
	if next.persisted != nil {
		s.rootPersisted = append(s.rootPersisted, next.persisted)
	}
	if next.persistedCallback != nil {
		s.persistedCallbacks = append(s.persistedCallbacks, next.persistedCallback)
	}
	// swap in new index snapshot
	newSnapshot.epoch = s.nextSnapshotEpoch
	s.nextSnapshotEpoch++
	rootPrev := s.root
	s.root = newSnapshot
	atomic.StoreUint64(&s.stats.CurRootEpoch, s.root.epoch)
	// release lock
	s.rootLock.Unlock()

	if rootPrev != nil {
		_ = rootPrev.DecRef()
	}

	close(next.applied)

	return nil
}

func (s *Scorch) introducePersist(persist *persistIntroduction) {
	atomic.AddUint64(&s.stats.TotIntroducePersistBeg, 1)
	defer atomic.AddUint64(&s.stats.TotIntroducePersistEnd, 1)

	s.rootLock.Lock()
	root := s.root
	root.AddRef()
	nextSnapshotEpoch := s.nextSnapshotEpoch
	s.nextSnapshotEpoch++
	s.rootLock.Unlock()

	defer func() { _ = root.DecRef() }()

	newIndexSnapshot := &IndexSnapshot{
		parent:   s,
		epoch:    nextSnapshotEpoch,
		segment:  make([]*SegmentSnapshot, len(root.segment)),
		offsets:  make([]uint64, len(root.offsets)),
		internal: make(map[string][]byte, len(root.internal)),
		refs:     1,
		creator:  "introducePersist",
	}

	var docsToPersistCount, memSegments, fileSegments uint64
	for i, segmentSnapshot := range root.segment {
		// see if this segment has been replaced
		if replacement, ok := persist.persisted[segmentSnapshot.id]; ok {
			newSegmentSnapshot := &SegmentSnapshot{
				id:         segmentSnapshot.id,
				segment:    replacement,
				deleted:    segmentSnapshot.deleted,
				cachedDocs: segmentSnapshot.cachedDocs,
				creator:    "introducePersist",
			}
			newIndexSnapshot.segment[i] = newSegmentSnapshot
			delete(persist.persisted, segmentSnapshot.id)

			// update items persisted incase of a new segment snapshot
			atomic.AddUint64(&s.stats.TotPersistedItems, newSegmentSnapshot.Count())
			atomic.AddUint64(&s.stats.TotPersistedSegments, 1)
			fileSegments++
		} else {
			newIndexSnapshot.segment[i] = root.segment[i]
			newIndexSnapshot.segment[i].segment.AddRef()

			if isMemorySegment(root.segment[i]) {
				docsToPersistCount += root.segment[i].Count()
				memSegments++
			} else {
				fileSegments++
			}
		}
		newIndexSnapshot.offsets[i] = root.offsets[i]
	}

	for k, v := range root.internal {
		newIndexSnapshot.internal[k] = v
	}

	atomic.StoreUint64(&s.stats.TotItemsToPersist, docsToPersistCount)
	atomic.StoreUint64(&s.stats.TotMemorySegmentsAtRoot, memSegments)
	atomic.StoreUint64(&s.stats.TotFileSegmentsAtRoot, fileSegments)
	newIndexSnapshot.updateSize()
	s.rootLock.Lock()
	rootPrev := s.root
	s.root = newIndexSnapshot
	atomic.StoreUint64(&s.stats.CurRootEpoch, s.root.epoch)
	s.rootLock.Unlock()

	if rootPrev != nil {
		_ = rootPrev.DecRef()
	}

	close(persist.applied)
}

// The introducer should definitely handle the segmentMerge.notify
// channel before exiting the introduceMerge.
func (s *Scorch) introduceMerge(nextMerge *segmentMerge) {
	atomic.AddUint64(&s.stats.TotIntroduceMergeBeg, 1)
	defer atomic.AddUint64(&s.stats.TotIntroduceMergeEnd, 1)

	s.rootLock.RLock()
	root := s.root
	root.AddRef()
	s.rootLock.RUnlock()

	defer func() { _ = root.DecRef() }()

	newSnapshot := &IndexSnapshot{
		parent:   s,
		internal: root.internal,
		refs:     1,
		creator:  "introduceMerge",
	}

	// iterate through current segments
	newSegmentDeleted := roaring.NewBitmap()
	var running, docsToPersistCount, memSegments, fileSegments uint64
	for i := range root.segment {
		segmentID := root.segment[i].id
		if segSnapAtMerge, ok := nextMerge.old[segmentID]; ok {
			// this segment is going away, see if anything else was deleted since we started the merge
			if segSnapAtMerge != nil && root.segment[i].deleted != nil {
				// assume all these deletes are new
				deletedSince := root.segment[i].deleted
				// if we already knew about some of them, remove
				if segSnapAtMerge.deleted != nil {
					deletedSince = roaring.AndNot(root.segment[i].deleted, segSnapAtMerge.deleted)
				}
				deletedSinceItr := deletedSince.Iterator()
				for deletedSinceItr.HasNext() {
					oldDocNum := deletedSinceItr.Next()
					newDocNum := nextMerge.oldNewDocNums[segmentID][oldDocNum]
					newSegmentDeleted.Add(uint32(newDocNum))
				}
			}
			// clean up the old segment map to figure out the
			// obsolete segments wrt root in meantime, whatever
			// segments left behind in old map after processing
			// the root segments would be the obsolete segment set
			delete(nextMerge.old, segmentID)
		} else if root.segment[i].LiveSize() > 0 {
			// this segment is staying
			newSnapshot.segment = append(newSnapshot.segment, &SegmentSnapshot{
				id:         root.segment[i].id,
				segment:    root.segment[i].segment,
				deleted:    root.segment[i].deleted,
				cachedDocs: root.segment[i].cachedDocs,
				creator:    root.segment[i].creator,
			})
			root.segment[i].segment.AddRef()
			newSnapshot.offsets = append(newSnapshot.offsets, running)
			running += root.segment[i].segment.Count()

			if isMemorySegment(root.segment[i]) {
				docsToPersistCount += root.segment[i].Count()
				memSegments++
			} else {
				fileSegments++
			}
		}

	}

	// before the newMerge introduction, need to clean the newly
	// merged segment wrt the current root segments, hence
	// applying the obsolete segment contents to newly merged segment
	for segID, ss := range nextMerge.old {
		obsoleted := ss.DocNumbersLive()
		if obsoleted != nil {
			obsoletedIter := obsoleted.Iterator()
			for obsoletedIter.HasNext() {
				oldDocNum := obsoletedIter.Next()
				newDocNum := nextMerge.oldNewDocNums[segID][oldDocNum]
				newSegmentDeleted.Add(uint32(newDocNum))
			}
		}
	}
	var skipped bool
	// In case where all the docs in the newly merged segment getting
	// deleted by the time we reach here, can skip the introduction.
	if nextMerge.new != nil &&
		nextMerge.new.Count() > newSegmentDeleted.GetCardinality() {
		// put new segment at end
		newSnapshot.segment = append(newSnapshot.segment, &SegmentSnapshot{
			id:         nextMerge.id,
			segment:    nextMerge.new, // take ownership for nextMerge.new's ref-count
			deleted:    newSegmentDeleted,
			cachedDocs: &cachedDocs{cache: nil},
			creator:    "introduceMerge",
		})
		newSnapshot.offsets = append(newSnapshot.offsets, running)
		atomic.AddUint64(&s.stats.TotIntroducedSegmentsMerge, 1)

		switch nextMerge.new.(type) {
		case segment.PersistedSegment:
			fileSegments++
		default:
			docsToPersistCount += nextMerge.new.Count() - newSegmentDeleted.GetCardinality()
			memSegments++
		}
	} else {
		skipped = true
		atomic.AddUint64(&s.stats.TotFileMergeIntroductionsObsoleted, 1)
	}

	atomic.StoreUint64(&s.stats.TotItemsToPersist, docsToPersistCount)
	atomic.StoreUint64(&s.stats.TotMemorySegmentsAtRoot, memSegments)
	atomic.StoreUint64(&s.stats.TotFileSegmentsAtRoot, fileSegments)

	newSnapshot.AddRef() // 1 ref for the nextMerge.notify response

	newSnapshot.updateSize()
	s.rootLock.Lock()
	// swap in new index snapshot
	newSnapshot.epoch = s.nextSnapshotEpoch
	s.nextSnapshotEpoch++
	rootPrev := s.root
	s.root = newSnapshot
	atomic.StoreUint64(&s.stats.CurRootEpoch, s.root.epoch)
	// release lock
	s.rootLock.Unlock()

	if rootPrev != nil {
		_ = rootPrev.DecRef()
	}

	// notify requester that we incorporated this
	nextMerge.notifyCh <- &mergeTaskIntroStatus{
		indexSnapshot: newSnapshot,
		skipped:       skipped}
	close(nextMerge.notifyCh)
}

func isMemorySegment(s *SegmentSnapshot) bool {
	switch s.segment.(type) {
	case segment.PersistedSegment:
		return false
	default:
		return true
	}
}
