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
	"github.com/blevesearch/bleve/index/scorch/segment"
)

type segmentIntroduction struct {
	id        uint64
	data      segment.Segment
	obsoletes map[uint64]*roaring.Bitmap
	ids       []string
	internal  map[string][]byte

	applied   chan error
	persisted chan error
}

type epochWatcher struct {
	epoch    uint64
	notifyCh notificationChan
}

type snapshotReversion struct {
	snapshot  *IndexSnapshot
	applied   chan error
	persisted chan error
}

func (s *Scorch) mainLoop() {
	var epochWatchers []*epochWatcher
OUTER:
	for {
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

		case revertTo := <-s.revertToSnapshots:
			err := s.revertToSnapshot(revertTo)
			if err != nil {
				continue OUTER
			}
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
	// acquire lock
	s.rootLock.Lock()

	nsegs := len(s.root.segment)

	// prepare new index snapshot
	newSnapshot := &IndexSnapshot{
		parent:   s,
		segment:  make([]*SegmentSnapshot, 0, nsegs+1),
		offsets:  make([]uint64, 0, nsegs+1),
		internal: make(map[string][]byte, len(s.root.internal)),
		epoch:    s.nextSnapshotEpoch,
		refs:     1,
	}
	s.nextSnapshotEpoch++

	// iterate through current segments
	var running uint64
	for i := range s.root.segment {
		// see if optimistic work included this segment
		delta, ok := next.obsoletes[s.root.segment[i].id]
		if !ok {
			var err error
			delta, err = s.root.segment[i].segment.DocNumbers(next.ids)
			if err != nil {
				s.rootLock.Unlock()
				next.applied <- fmt.Errorf("error computing doc numbers: %v", err)
				close(next.applied)
				_ = newSnapshot.DecRef()
				return err
			}
		}

		newss := &SegmentSnapshot{
			id:         s.root.segment[i].id,
			segment:    s.root.segment[i].segment,
			cachedDocs: s.root.segment[i].cachedDocs,
		}

		// apply new obsoletions
		if s.root.segment[i].deleted == nil {
			newss.deleted = delta
		} else {
			newss.deleted = roaring.Or(s.root.segment[i].deleted, delta)
		}

		// check for live size before copying
		if newss.LiveSize() > 0 {
			newSnapshot.segment = append(newSnapshot.segment, newss)
			s.root.segment[i].segment.AddRef()
			newSnapshot.offsets = append(newSnapshot.offsets, running)
			running += s.root.segment[i].Count()
		}
	}

	// append new segment, if any, to end of the new index snapshot
	if next.data != nil {
		newSegmentSnapshot := &SegmentSnapshot{
			id:         next.id,
			segment:    next.data, // take ownership of next.data's ref-count
			cachedDocs: &cachedDocs{cache: nil},
		}
		newSnapshot.segment = append(newSnapshot.segment, newSegmentSnapshot)
		newSnapshot.offsets = append(newSnapshot.offsets, running)

		// increment numItemsIntroduced which tracks the number of items
		// queued for persistence.
		atomic.AddUint64(&s.stats.numItemsIntroduced, newSegmentSnapshot.Count())
	}
	// copy old values
	for key, oldVal := range s.root.internal {
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
	if next.persisted != nil {
		s.rootPersisted = append(s.rootPersisted, next.persisted)
	}
	// swap in new index snapshot
	rootPrev := s.root
	s.root = newSnapshot
	// release lock
	s.rootLock.Unlock()

	if rootPrev != nil {
		_ = rootPrev.DecRef()
	}

	close(next.applied)

	return nil
}

func (s *Scorch) introduceMerge(nextMerge *segmentMerge) {
	// acquire lock
	s.rootLock.Lock()

	// prepare new index snapshot
	currSize := len(s.root.segment)
	newSize := currSize + 1 - len(nextMerge.old)

	// empty segments deletion
	if nextMerge.new == nil {
		newSize--
	}

	newSnapshot := &IndexSnapshot{
		parent:   s,
		segment:  make([]*SegmentSnapshot, 0, newSize),
		offsets:  make([]uint64, 0, newSize),
		internal: s.root.internal,
		epoch:    s.nextSnapshotEpoch,
		refs:     1,
	}
	s.nextSnapshotEpoch++

	// iterate through current segments
	newSegmentDeleted := roaring.NewBitmap()
	var running uint64
	for i := range s.root.segment {
		segmentID := s.root.segment[i].id
		if segSnapAtMerge, ok := nextMerge.old[segmentID]; ok {
			// this segment is going away, see if anything else was deleted since we started the merge
			if segSnapAtMerge != nil && s.root.segment[i].deleted != nil {
				// assume all these deletes are new
				deletedSince := s.root.segment[i].deleted
				// if we already knew about some of them, remove
				if segSnapAtMerge.deleted != nil {
					deletedSince = roaring.AndNot(s.root.segment[i].deleted, segSnapAtMerge.deleted)
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

		} else if s.root.segment[i].LiveSize() > 0 {
			// this segment is staying
			newSnapshot.segment = append(newSnapshot.segment, &SegmentSnapshot{
				id:         s.root.segment[i].id,
				segment:    s.root.segment[i].segment,
				deleted:    s.root.segment[i].deleted,
				cachedDocs: s.root.segment[i].cachedDocs,
			})
			s.root.segment[i].segment.AddRef()
			newSnapshot.offsets = append(newSnapshot.offsets, running)
			running += s.root.segment[i].Count()
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
		})
		newSnapshot.offsets = append(newSnapshot.offsets, running)
	}

	newSnapshot.AddRef() // 1 ref for the nextMerge.notify response

	// swap in new segment
	rootPrev := s.root
	s.root = newSnapshot
	// release lock
	s.rootLock.Unlock()

	if rootPrev != nil {
		_ = rootPrev.DecRef()
	}

	// notify requester that we incorporated this
	nextMerge.notify <- newSnapshot
	close(nextMerge.notify)
}

func (s *Scorch) revertToSnapshot(revertTo *snapshotReversion) error {
	if revertTo.snapshot == nil {
		err := fmt.Errorf("Cannot revert to a nil snapshot")
		revertTo.applied <- err
		return err
	}

	// acquire lock
	s.rootLock.Lock()

	// prepare a new index snapshot, based on next snapshot
	newSnapshot := &IndexSnapshot{
		parent:   s,
		segment:  make([]*SegmentSnapshot, len(revertTo.snapshot.segment)),
		offsets:  revertTo.snapshot.offsets,
		internal: revertTo.snapshot.internal,
		epoch:    s.nextSnapshotEpoch,
		refs:     1,
	}
	s.nextSnapshotEpoch++

	// iterate through segments
	for i, segmentSnapshot := range revertTo.snapshot.segment {
		newSnapshot.segment[i] = &SegmentSnapshot{
			id:         segmentSnapshot.id,
			segment:    segmentSnapshot.segment,
			deleted:    segmentSnapshot.deleted,
			cachedDocs: segmentSnapshot.cachedDocs,
		}
		newSnapshot.segment[i].segment.AddRef()

		// remove segment from ineligibleForRemoval map
		filename := zapFileName(segmentSnapshot.id)
		delete(s.ineligibleForRemoval, filename)
	}

	if revertTo.persisted != nil {
		s.rootPersisted = append(s.rootPersisted, revertTo.persisted)
	}

	// swap in new snapshot
	rootPrev := s.root
	s.root = newSnapshot
	// release lock
	s.rootLock.Unlock()

	if rootPrev != nil {
		_ = rootPrev.DecRef()
	}

	close(revertTo.applied)

	return nil
}
