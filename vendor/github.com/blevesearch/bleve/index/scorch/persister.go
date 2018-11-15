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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/index/scorch/segment"
	"github.com/blevesearch/bleve/index/scorch/segment/zap"
	"github.com/boltdb/bolt"
)

var DefaultChunkFactor uint32 = 1024

// Arbitrary number, need to make it configurable.
// Lower values like 10/making persister really slow
// doesn't work well as it is creating more files to
// persist for in next persist iteration and spikes the # FDs.
// Ideal value should let persister also proceed at
// an optimum pace so that the merger can skip
// many intermediate snapshots.
// This needs to be based on empirical data.
// TODO - may need to revisit this approach/value.
var epochDistance = uint64(5)

type notificationChan chan struct{}

func (s *Scorch) persisterLoop() {
	defer s.asyncTasks.Done()

	var persistWatchers []*epochWatcher
	var lastPersistedEpoch, lastMergedEpoch uint64
	var ew *epochWatcher
OUTER:
	for {
		select {
		case <-s.closeCh:
			break OUTER
		case ew = <-s.persisterNotifier:
			persistWatchers = append(persistWatchers, ew)
		default:
		}
		if ew != nil && ew.epoch > lastMergedEpoch {
			lastMergedEpoch = ew.epoch
		}
		persistWatchers = s.pausePersisterForMergerCatchUp(lastPersistedEpoch,
			&lastMergedEpoch, persistWatchers)

		var ourSnapshot *IndexSnapshot
		var ourPersisted []chan error

		// check to see if there is a new snapshot to persist
		s.rootLock.Lock()
		if s.root != nil && s.root.epoch > lastPersistedEpoch {
			ourSnapshot = s.root
			ourSnapshot.AddRef()
			ourPersisted = s.rootPersisted
			s.rootPersisted = nil
		}
		s.rootLock.Unlock()

		if ourSnapshot != nil {
			startTime := time.Now()

			err := s.persistSnapshot(ourSnapshot)
			for _, ch := range ourPersisted {
				if err != nil {
					ch <- err
				}
				close(ch)
			}
			if err != nil {
				s.fireAsyncError(fmt.Errorf("got err persisting snapshot: %v", err))
				_ = ourSnapshot.DecRef()
				continue OUTER
			}

			lastPersistedEpoch = ourSnapshot.epoch
			for _, ew := range persistWatchers {
				close(ew.notifyCh)
			}

			persistWatchers = nil
			_ = ourSnapshot.DecRef()

			changed := false
			s.rootLock.RLock()
			if s.root != nil && s.root.epoch != lastPersistedEpoch {
				changed = true
			}
			s.rootLock.RUnlock()

			s.fireEvent(EventKindPersisterProgress, time.Since(startTime))

			if changed {
				continue OUTER
			}
		}

		// tell the introducer we're waiting for changes
		w := &epochWatcher{
			epoch:    lastPersistedEpoch,
			notifyCh: make(notificationChan, 1),
		}

		select {
		case <-s.closeCh:
			break OUTER
		case s.introducerNotifier <- w:
		}

		s.removeOldData() // might as well cleanup while waiting

		select {
		case <-s.closeCh:
			break OUTER
		case <-w.notifyCh:
			// woken up, next loop should pick up work
			continue OUTER
		case ew = <-s.persisterNotifier:
			// if the watchers are already caught up then let them wait,
			// else let them continue to do the catch up
			persistWatchers = append(persistWatchers, ew)
		}
	}
}

func notifyMergeWatchers(lastPersistedEpoch uint64,
	persistWatchers []*epochWatcher) []*epochWatcher {
	var watchersNext []*epochWatcher
	for _, w := range persistWatchers {
		if w.epoch < lastPersistedEpoch {
			close(w.notifyCh)
		} else {
			watchersNext = append(watchersNext, w)
		}
	}
	return watchersNext
}

func (s *Scorch) pausePersisterForMergerCatchUp(lastPersistedEpoch uint64, lastMergedEpoch *uint64,
	persistWatchers []*epochWatcher) []*epochWatcher {

	// first, let the watchers proceed if they lag behind
	persistWatchers = notifyMergeWatchers(lastPersistedEpoch, persistWatchers)

OUTER:
	// check for slow merger and await until the merger catch up
	for lastPersistedEpoch > *lastMergedEpoch+epochDistance {

		select {
		case <-s.closeCh:
			break OUTER
		case ew := <-s.persisterNotifier:
			persistWatchers = append(persistWatchers, ew)
			*lastMergedEpoch = ew.epoch
		}

		// let the watchers proceed if they lag behind
		persistWatchers = notifyMergeWatchers(lastPersistedEpoch, persistWatchers)
	}

	return persistWatchers
}

func (s *Scorch) persistSnapshot(snapshot *IndexSnapshot) error {
	persisted, err := s.persistSnapshotMaybeMerge(snapshot)
	if err != nil {
		return err
	}
	if persisted {
		return nil
	}

	return s.persistSnapshotDirect(snapshot)
}

// DefaultMinSegmentsForInMemoryMerge represents the default number of
// in-memory zap segments that persistSnapshotMaybeMerge() needs to
// see in an IndexSnapshot before it decides to merge and persist
// those segments
var DefaultMinSegmentsForInMemoryMerge = 2

// persistSnapshotMaybeMerge examines the snapshot and might merge and
// persist the in-memory zap segments if there are enough of them
func (s *Scorch) persistSnapshotMaybeMerge(snapshot *IndexSnapshot) (
	bool, error) {
	// collect the in-memory zap segments (SegmentBase instances)
	var sbs []*zap.SegmentBase
	var sbsDrops []*roaring.Bitmap
	var sbsIndexes []int

	for i, segmentSnapshot := range snapshot.segment {
		if sb, ok := segmentSnapshot.segment.(*zap.SegmentBase); ok {
			sbs = append(sbs, sb)
			sbsDrops = append(sbsDrops, segmentSnapshot.deleted)
			sbsIndexes = append(sbsIndexes, i)
		}
	}

	if len(sbs) < DefaultMinSegmentsForInMemoryMerge {
		return false, nil
	}

	_, newSnapshot, newSegmentID, err := s.mergeSegmentBases(
		snapshot, sbs, sbsDrops, sbsIndexes, DefaultChunkFactor)
	if err != nil {
		return false, err
	}
	if newSnapshot == nil {
		return false, nil
	}

	defer func() {
		_ = newSnapshot.DecRef()
	}()

	mergedSegmentIDs := map[uint64]struct{}{}
	for _, idx := range sbsIndexes {
		mergedSegmentIDs[snapshot.segment[idx].id] = struct{}{}
	}

	// construct a snapshot that's logically equivalent to the input
	// snapshot, but with merged segments replaced by the new segment
	equiv := &IndexSnapshot{
		parent:   snapshot.parent,
		segment:  make([]*SegmentSnapshot, 0, len(snapshot.segment)),
		internal: snapshot.internal,
		epoch:    snapshot.epoch,
	}

	// copy to the equiv the segments that weren't replaced
	for _, segment := range snapshot.segment {
		if _, wasMerged := mergedSegmentIDs[segment.id]; !wasMerged {
			equiv.segment = append(equiv.segment, segment)
		}
	}

	// append to the equiv the new segment
	for _, segment := range newSnapshot.segment {
		if segment.id == newSegmentID {
			equiv.segment = append(equiv.segment, &SegmentSnapshot{
				id:      newSegmentID,
				segment: segment.segment,
				deleted: nil, // nil since merging handled deletions
			})
			break
		}
	}

	err = s.persistSnapshotDirect(equiv)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *Scorch) persistSnapshotDirect(snapshot *IndexSnapshot) (err error) {
	// start a write transaction
	tx, err := s.rootBolt.Begin(true)
	if err != nil {
		return err
	}
	// defer rollback on error
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	snapshotsBucket, err := tx.CreateBucketIfNotExists(boltSnapshotsBucket)
	if err != nil {
		return err
	}
	newSnapshotKey := segment.EncodeUvarintAscending(nil, snapshot.epoch)
	snapshotBucket, err := snapshotsBucket.CreateBucketIfNotExists(newSnapshotKey)
	if err != nil {
		return err
	}

	// persist internal values
	internalBucket, err := snapshotBucket.CreateBucketIfNotExists(boltInternalKey)
	if err != nil {
		return err
	}
	// TODO optimize writing these in order?
	for k, v := range snapshot.internal {
		err = internalBucket.Put([]byte(k), v)
		if err != nil {
			return err
		}
	}

	var filenames []string
	newSegmentPaths := make(map[uint64]string)

	// first ensure that each segment in this snapshot has been persisted
	for _, segmentSnapshot := range snapshot.segment {
		snapshotSegmentKey := segment.EncodeUvarintAscending(nil, segmentSnapshot.id)
		snapshotSegmentBucket, err := snapshotBucket.CreateBucketIfNotExists(snapshotSegmentKey)
		if err != nil {
			return err
		}
		switch seg := segmentSnapshot.segment.(type) {
		case *zap.SegmentBase:
			// need to persist this to disk
			filename := zapFileName(segmentSnapshot.id)
			path := s.path + string(os.PathSeparator) + filename
			err = zap.PersistSegmentBase(seg, path)
			if err != nil {
				return fmt.Errorf("error persisting segment: %v", err)
			}
			newSegmentPaths[segmentSnapshot.id] = path
			err = snapshotSegmentBucket.Put(boltPathKey, []byte(filename))
			if err != nil {
				return err
			}
			filenames = append(filenames, filename)
		case *zap.Segment:
			path := seg.Path()
			filename := strings.TrimPrefix(path, s.path+string(os.PathSeparator))
			err = snapshotSegmentBucket.Put(boltPathKey, []byte(filename))
			if err != nil {
				return err
			}
			filenames = append(filenames, filename)
		default:
			return fmt.Errorf("unknown segment type: %T", seg)
		}
		// store current deleted bits
		var roaringBuf bytes.Buffer
		if segmentSnapshot.deleted != nil {
			_, err = segmentSnapshot.deleted.WriteTo(&roaringBuf)
			if err != nil {
				return fmt.Errorf("error persisting roaring bytes: %v", err)
			}
			err = snapshotSegmentBucket.Put(boltDeletedKey, roaringBuf.Bytes())
			if err != nil {
				return err
			}
		}
	}

	// we need to swap in a new root only when we've persisted 1 or
	// more segments -- whereby the new root would have 1-for-1
	// replacements of in-memory segments with file-based segments
	//
	// other cases like updates to internal values only, and/or when
	// there are only deletions, are already covered and persisted by
	// the newly populated boltdb snapshotBucket above
	if len(newSegmentPaths) > 0 {
		// now try to open all the new snapshots
		newSegments := make(map[uint64]segment.Segment)
		defer func() {
			for _, s := range newSegments {
				if s != nil {
					// cleanup segments that were opened but not
					// swapped into the new root
					_ = s.Close()
				}
			}
		}()
		for segmentID, path := range newSegmentPaths {
			newSegments[segmentID], err = zap.Open(path)
			if err != nil {
				return fmt.Errorf("error opening new segment at %s, %v", path, err)
			}
		}

		s.rootLock.Lock()
		newIndexSnapshot := &IndexSnapshot{
			parent:   s,
			epoch:    s.nextSnapshotEpoch,
			segment:  make([]*SegmentSnapshot, len(s.root.segment)),
			offsets:  make([]uint64, len(s.root.offsets)),
			internal: make(map[string][]byte, len(s.root.internal)),
			refs:     1,
		}
		s.nextSnapshotEpoch++
		for i, segmentSnapshot := range s.root.segment {
			// see if this segment has been replaced
			if replacement, ok := newSegments[segmentSnapshot.id]; ok {
				newSegmentSnapshot := &SegmentSnapshot{
					id:         segmentSnapshot.id,
					segment:    replacement,
					deleted:    segmentSnapshot.deleted,
					cachedDocs: segmentSnapshot.cachedDocs,
				}
				newIndexSnapshot.segment[i] = newSegmentSnapshot
				delete(newSegments, segmentSnapshot.id)
				// update items persisted incase of a new segment snapshot
				atomic.AddUint64(&s.stats.numItemsPersisted, newSegmentSnapshot.Count())
			} else {
				newIndexSnapshot.segment[i] = s.root.segment[i]
				newIndexSnapshot.segment[i].segment.AddRef()
			}
			newIndexSnapshot.offsets[i] = s.root.offsets[i]
		}
		for k, v := range s.root.internal {
			newIndexSnapshot.internal[k] = v
		}

		rootPrev := s.root
		s.root = newIndexSnapshot
		s.rootLock.Unlock()
		if rootPrev != nil {
			_ = rootPrev.DecRef()
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	err = s.rootBolt.Sync()
	if err != nil {
		return err
	}

	// allow files to become eligible for removal after commit, such
	// as file segments from snapshots that came from the merger
	s.rootLock.Lock()
	for _, filename := range filenames {
		delete(s.ineligibleForRemoval, filename)
	}
	s.rootLock.Unlock()

	return nil
}

func zapFileName(epoch uint64) string {
	return fmt.Sprintf("%012x.zap", epoch)
}

// bolt snapshot code

var boltSnapshotsBucket = []byte{'s'}
var boltPathKey = []byte{'p'}
var boltDeletedKey = []byte{'d'}
var boltInternalKey = []byte{'i'}

func (s *Scorch) loadFromBolt() error {
	return s.rootBolt.View(func(tx *bolt.Tx) error {
		snapshots := tx.Bucket(boltSnapshotsBucket)
		if snapshots == nil {
			return nil
		}
		foundRoot := false
		c := snapshots.Cursor()
		for k, _ := c.Last(); k != nil; k, _ = c.Prev() {
			_, snapshotEpoch, err := segment.DecodeUvarintAscending(k)
			if err != nil {
				log.Printf("unable to parse segment epoch %x, continuing", k)
				continue
			}
			if foundRoot {
				s.eligibleForRemoval = append(s.eligibleForRemoval, snapshotEpoch)
				continue
			}
			snapshot := snapshots.Bucket(k)
			if snapshot == nil {
				log.Printf("snapshot key, but bucket missing %x, continuing", k)
				s.eligibleForRemoval = append(s.eligibleForRemoval, snapshotEpoch)
				continue
			}
			indexSnapshot, err := s.loadSnapshot(snapshot)
			if err != nil {
				log.Printf("unable to load snapshot, %v, continuing", err)
				s.eligibleForRemoval = append(s.eligibleForRemoval, snapshotEpoch)
				continue
			}
			indexSnapshot.epoch = snapshotEpoch
			// set the nextSegmentID
			s.nextSegmentID, err = s.maxSegmentIDOnDisk()
			if err != nil {
				return err
			}
			s.nextSegmentID++
			s.nextSnapshotEpoch = snapshotEpoch + 1
			s.rootLock.Lock()
			if s.root != nil {
				_ = s.root.DecRef()
			}
			s.root = indexSnapshot
			s.rootLock.Unlock()
			foundRoot = true
		}
		return nil
	})
}

// LoadSnapshot loads the segment with the specified epoch
// NOTE: this is currently ONLY intended to be used by the command-line tool
func (s *Scorch) LoadSnapshot(epoch uint64) (rv *IndexSnapshot, err error) {
	err = s.rootBolt.View(func(tx *bolt.Tx) error {
		snapshots := tx.Bucket(boltSnapshotsBucket)
		if snapshots == nil {
			return nil
		}
		snapshotKey := segment.EncodeUvarintAscending(nil, epoch)
		snapshot := snapshots.Bucket(snapshotKey)
		if snapshot == nil {
			return nil
		}
		rv, err = s.loadSnapshot(snapshot)
		return err
	})
	if err != nil {
		return nil, err
	}
	return rv, nil
}

func (s *Scorch) loadSnapshot(snapshot *bolt.Bucket) (*IndexSnapshot, error) {

	rv := &IndexSnapshot{
		parent:   s,
		internal: make(map[string][]byte),
		refs:     1,
	}
	var running uint64
	c := snapshot.Cursor()
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		if k[0] == boltInternalKey[0] {
			internalBucket := snapshot.Bucket(k)
			err := internalBucket.ForEach(func(key []byte, val []byte) error {
				copiedVal := append([]byte(nil), val...)
				rv.internal[string(key)] = copiedVal
				return nil
			})
			if err != nil {
				_ = rv.DecRef()
				return nil, err
			}
		} else {
			segmentBucket := snapshot.Bucket(k)
			if segmentBucket == nil {
				_ = rv.DecRef()
				return nil, fmt.Errorf("segment key, but bucket missing % x", k)
			}
			segmentSnapshot, err := s.loadSegment(segmentBucket)
			if err != nil {
				_ = rv.DecRef()
				return nil, fmt.Errorf("failed to load segment: %v", err)
			}
			_, segmentSnapshot.id, err = segment.DecodeUvarintAscending(k)
			if err != nil {
				_ = rv.DecRef()
				return nil, fmt.Errorf("failed to decode segment id: %v", err)
			}
			rv.segment = append(rv.segment, segmentSnapshot)
			rv.offsets = append(rv.offsets, running)
			running += segmentSnapshot.segment.Count()
		}
	}
	return rv, nil
}

func (s *Scorch) loadSegment(segmentBucket *bolt.Bucket) (*SegmentSnapshot, error) {
	pathBytes := segmentBucket.Get(boltPathKey)
	if pathBytes == nil {
		return nil, fmt.Errorf("segment path missing")
	}
	segmentPath := s.path + string(os.PathSeparator) + string(pathBytes)
	segment, err := zap.Open(segmentPath)
	if err != nil {
		return nil, fmt.Errorf("error opening bolt segment: %v", err)
	}

	rv := &SegmentSnapshot{
		segment:    segment,
		cachedDocs: &cachedDocs{cache: nil},
	}
	deletedBytes := segmentBucket.Get(boltDeletedKey)
	if deletedBytes != nil {
		deletedBitmap := roaring.NewBitmap()
		r := bytes.NewReader(deletedBytes)
		_, err := deletedBitmap.ReadFrom(r)
		if err != nil {
			_ = segment.Close()
			return nil, fmt.Errorf("error reading deleted bytes: %v", err)
		}
		rv.deleted = deletedBitmap
	}

	return rv, nil
}

type uint64Descending []uint64

func (p uint64Descending) Len() int           { return len(p) }
func (p uint64Descending) Less(i, j int) bool { return p[i] > p[j] }
func (p uint64Descending) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (s *Scorch) removeOldData() {
	removed, err := s.removeOldBoltSnapshots()
	if err != nil {
		s.fireAsyncError(fmt.Errorf("got err removing old bolt snapshots: %v", err))
	}

	if removed > 0 {
		err = s.removeOldZapFiles()
		if err != nil {
			s.fireAsyncError(fmt.Errorf("got err removing old zap files: %v", err))
		}
	}
}

// NumSnapshotsToKeep represents how many recent, old snapshots to
// keep around per Scorch instance.  Useful for apps that require
// rollback'ability.
var NumSnapshotsToKeep = 1

// Removes enough snapshots from the rootBolt so that the
// s.eligibleForRemoval stays under the NumSnapshotsToKeep policy.
func (s *Scorch) removeOldBoltSnapshots() (numRemoved int, err error) {
	persistedEpochs, err := s.RootBoltSnapshotEpochs()
	if err != nil {
		return 0, err
	}

	if len(persistedEpochs) <= NumSnapshotsToKeep {
		// we need to keep everything
		return 0, nil
	}

	// make a map of epochs to protect from deletion
	protectedEpochs := make(map[uint64]struct{}, NumSnapshotsToKeep)
	for _, epoch := range persistedEpochs[0:NumSnapshotsToKeep] {
		protectedEpochs[epoch] = struct{}{}
	}

	var epochsToRemove []uint64
	var newEligible []uint64
	s.rootLock.Lock()
	for _, epoch := range s.eligibleForRemoval {
		if _, ok := protectedEpochs[epoch]; ok {
			// protected
			newEligible = append(newEligible, epoch)
		} else {
			epochsToRemove = append(epochsToRemove, epoch)
		}
	}
	s.eligibleForRemoval = newEligible
	s.rootLock.Unlock()

	if len(epochsToRemove) <= 0 {
		return 0, nil
	}

	tx, err := s.rootBolt.Begin(true)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		if err == nil {
			err = s.rootBolt.Sync()
		}
	}()

	snapshots := tx.Bucket(boltSnapshotsBucket)
	if snapshots == nil {
		return 0, nil
	}

	for _, epochToRemove := range epochsToRemove {
		k := segment.EncodeUvarintAscending(nil, epochToRemove)
		err = snapshots.DeleteBucket(k)
		if err == bolt.ErrBucketNotFound {
			err = nil
		}
		if err == nil {
			numRemoved++
		}
	}

	return numRemoved, err
}

func (s *Scorch) maxSegmentIDOnDisk() (uint64, error) {
	currFileInfos, err := ioutil.ReadDir(s.path)
	if err != nil {
		return 0, err
	}

	var rv uint64
	for _, finfo := range currFileInfos {
		fname := finfo.Name()
		if filepath.Ext(fname) == ".zap" {
			prefix := strings.TrimSuffix(fname, ".zap")
			id, err2 := strconv.ParseUint(prefix, 16, 64)
			if err2 != nil {
				return 0, err2
			}
			if id > rv {
				rv = id
			}
		}
	}
	return rv, err
}

// Removes any *.zap files which aren't listed in the rootBolt.
func (s *Scorch) removeOldZapFiles() error {
	liveFileNames, err := s.loadZapFileNames()
	if err != nil {
		return err
	}

	currFileInfos, err := ioutil.ReadDir(s.path)
	if err != nil {
		return err
	}

	s.rootLock.RLock()

	for _, finfo := range currFileInfos {
		fname := finfo.Name()
		if filepath.Ext(fname) == ".zap" {
			if _, exists := liveFileNames[fname]; !exists && !s.ineligibleForRemoval[fname] {
				err := os.Remove(s.path + string(os.PathSeparator) + fname)
				if err != nil {
					log.Printf("got err removing file: %s, err: %v", fname, err)
				}
			}
		}
	}

	s.rootLock.RUnlock()

	return nil
}

func (s *Scorch) RootBoltSnapshotEpochs() ([]uint64, error) {
	var rv []uint64
	err := s.rootBolt.View(func(tx *bolt.Tx) error {
		snapshots := tx.Bucket(boltSnapshotsBucket)
		if snapshots == nil {
			return nil
		}
		sc := snapshots.Cursor()
		for sk, _ := sc.Last(); sk != nil; sk, _ = sc.Prev() {
			_, snapshotEpoch, err := segment.DecodeUvarintAscending(sk)
			if err != nil {
				continue
			}
			rv = append(rv, snapshotEpoch)
		}
		return nil
	})
	return rv, err
}

// Returns the *.zap file names that are listed in the rootBolt.
func (s *Scorch) loadZapFileNames() (map[string]struct{}, error) {
	rv := map[string]struct{}{}
	err := s.rootBolt.View(func(tx *bolt.Tx) error {
		snapshots := tx.Bucket(boltSnapshotsBucket)
		if snapshots == nil {
			return nil
		}
		sc := snapshots.Cursor()
		for sk, _ := sc.First(); sk != nil; sk, _ = sc.Next() {
			snapshot := snapshots.Bucket(sk)
			if snapshot == nil {
				continue
			}
			segc := snapshot.Cursor()
			for segk, _ := segc.First(); segk != nil; segk, _ = segc.Next() {
				if segk[0] == boltInternalKey[0] {
					continue
				}
				segmentBucket := snapshot.Bucket(segk)
				if segmentBucket == nil {
					continue
				}
				pathBytes := segmentBucket.Get(boltPathKey)
				if pathBytes == nil {
					continue
				}
				pathString := string(pathBytes)
				rv[string(pathString)] = struct{}{}
			}
		}
		return nil
	})

	return rv, err
}
