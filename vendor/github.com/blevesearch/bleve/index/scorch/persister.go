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
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/index/scorch/segment"
	bolt "go.etcd.io/bbolt"
)

// DefaultPersisterNapTimeMSec is kept to zero as this helps in direct
// persistence of segments with the default safe batch option.
// If the default safe batch option results in high number of
// files on disk, then users may initialise this configuration parameter
// with higher values so that the persister will nap a bit within it's
// work loop to favour better in-memory merging of segments to result
// in fewer segment files on disk. But that may come with an indexing
// performance overhead.
// Unsafe batch users are advised to override this to higher value
// for better performance especially with high data density.
var DefaultPersisterNapTimeMSec int = 0 // ms

// DefaultPersisterNapUnderNumFiles helps in controlling the pace of
// persister. At times of a slow merger progress with heavy file merging
// operations, its better to pace down the persister for letting the merger
// to catch up within a range defined by this parameter.
// Fewer files on disk (as per the merge plan) would result in keeping the
// file handle usage under limit, faster disk merger and a healthier index.
// Its been observed that such a loosely sync'ed introducer-persister-merger
// trio results in better overall performance.
var DefaultPersisterNapUnderNumFiles int = 1000

var DefaultMemoryPressurePauseThreshold uint64 = math.MaxUint64

type persisterOptions struct {
	// PersisterNapTimeMSec controls the wait/delay injected into
	// persistence workloop to improve the chances for
	// a healthier and heavier in-memory merging
	PersisterNapTimeMSec int

	// PersisterNapTimeMSec > 0, and the number of files is less than
	// PersisterNapUnderNumFiles, then the persister will sleep
	// PersisterNapTimeMSec amount of time to improve the chances for
	// a healthier and heavier in-memory merging
	PersisterNapUnderNumFiles int

	// MemoryPressurePauseThreshold let persister to have a better leeway
	// for prudently performing the memory merge of segments on a memory
	// pressure situation. Here the config value is an upper threshold
	// for the number of paused application threads. The default value would
	// be a very high number to always favour the merging of memory segments.
	MemoryPressurePauseThreshold uint64
}

type notificationChan chan struct{}

func (s *Scorch) persisterLoop() {
	defer s.asyncTasks.Done()

	var persistWatchers []*epochWatcher
	var lastPersistedEpoch, lastMergedEpoch uint64
	var ew *epochWatcher

	var unpersistedCallbacks []index.BatchCallback

	po, err := s.parsePersisterOptions()
	if err != nil {
		s.fireAsyncError(fmt.Errorf("persisterOptions json parsing err: %v", err))
		s.asyncTasks.Done()
		return
	}

OUTER:
	for {
		atomic.AddUint64(&s.stats.TotPersistLoopBeg, 1)

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
		lastMergedEpoch, persistWatchers = s.pausePersisterForMergerCatchUp(lastPersistedEpoch,
			lastMergedEpoch, persistWatchers, po)

		var ourSnapshot *IndexSnapshot
		var ourPersisted []chan error
		var ourPersistedCallbacks []index.BatchCallback

		// check to see if there is a new snapshot to persist
		s.rootLock.Lock()
		if s.root != nil && s.root.epoch > lastPersistedEpoch {
			ourSnapshot = s.root
			ourSnapshot.AddRef()
			ourPersisted = s.rootPersisted
			s.rootPersisted = nil
			ourPersistedCallbacks = s.persistedCallbacks
			s.persistedCallbacks = nil
			atomic.StoreUint64(&s.iStats.persistSnapshotSize, uint64(ourSnapshot.Size()))
			atomic.StoreUint64(&s.iStats.persistEpoch, ourSnapshot.epoch)
		}
		s.rootLock.Unlock()

		if ourSnapshot != nil {
			startTime := time.Now()

			err := s.persistSnapshot(ourSnapshot, po)
			for _, ch := range ourPersisted {
				if err != nil {
					ch <- err
				}
				close(ch)
			}
			if err != nil {
				atomic.StoreUint64(&s.iStats.persistEpoch, 0)
				if err == segment.ErrClosed {
					// index has been closed
					_ = ourSnapshot.DecRef()
					break OUTER
				}

				// save this current snapshot's persistedCallbacks, to invoke during
				// the retry attempt
				unpersistedCallbacks = append(unpersistedCallbacks, ourPersistedCallbacks...)

				s.fireAsyncError(fmt.Errorf("got err persisting snapshot: %v", err))
				_ = ourSnapshot.DecRef()
				atomic.AddUint64(&s.stats.TotPersistLoopErr, 1)
				continue OUTER
			}

			if unpersistedCallbacks != nil {
				// in the event of this being a retry attempt for persisting a snapshot
				// that had earlier failed, prepend the persistedCallbacks associated
				// with earlier segment(s) to the latest persistedCallbacks
				ourPersistedCallbacks = append(unpersistedCallbacks, ourPersistedCallbacks...)
				unpersistedCallbacks = nil
			}

			for i := range ourPersistedCallbacks {
				ourPersistedCallbacks[i](err)
			}

			atomic.StoreUint64(&s.stats.LastPersistedEpoch, ourSnapshot.epoch)

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
				atomic.AddUint64(&s.stats.TotPersistLoopProgress, 1)
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

		atomic.AddUint64(&s.stats.TotPersistLoopWait, 1)

		select {
		case <-s.closeCh:
			break OUTER
		case <-w.notifyCh:
			// woken up, next loop should pick up work
			atomic.AddUint64(&s.stats.TotPersistLoopWaitNotified, 1)
		case ew = <-s.persisterNotifier:
			// if the watchers are already caught up then let them wait,
			// else let them continue to do the catch up
			persistWatchers = append(persistWatchers, ew)
		}

		atomic.AddUint64(&s.stats.TotPersistLoopEnd, 1)
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

func (s *Scorch) pausePersisterForMergerCatchUp(lastPersistedEpoch uint64,
	lastMergedEpoch uint64, persistWatchers []*epochWatcher,
	po *persisterOptions) (uint64, []*epochWatcher) {

	// First, let the watchers proceed if they lag behind
	persistWatchers = notifyMergeWatchers(lastPersistedEpoch, persistWatchers)

	// Check the merger lag by counting the segment files on disk,
	numFilesOnDisk, _, _ := s.diskFileStats(nil)

	// On finding fewer files on disk, persister takes a short pause
	// for sufficient in-memory segments to pile up for the next
	// memory merge cum persist loop.
	if numFilesOnDisk < uint64(po.PersisterNapUnderNumFiles) &&
		po.PersisterNapTimeMSec > 0 && s.NumEventsBlocking() == 0 {
		select {
		case <-s.closeCh:
		case <-time.After(time.Millisecond * time.Duration(po.PersisterNapTimeMSec)):
			atomic.AddUint64(&s.stats.TotPersisterNapPauseCompleted, 1)

		case ew := <-s.persisterNotifier:
			// unblock the merger in meantime
			persistWatchers = append(persistWatchers, ew)
			lastMergedEpoch = ew.epoch
			persistWatchers = notifyMergeWatchers(lastPersistedEpoch, persistWatchers)
			atomic.AddUint64(&s.stats.TotPersisterMergerNapBreak, 1)
		}
		return lastMergedEpoch, persistWatchers
	}

	// Finding too many files on disk could be due to two reasons.
	// 1. Too many older snapshots awaiting the clean up.
	// 2. The merger could be lagging behind on merging the disk files.
	if numFilesOnDisk > uint64(po.PersisterNapUnderNumFiles) {
		s.removeOldData()
		numFilesOnDisk, _, _ = s.diskFileStats(nil)
	}

	// Persister pause until the merger catches up to reduce the segment
	// file count under the threshold.
	// But if there is memory pressure, then skip this sleep maneuvers.
OUTER:
	for po.PersisterNapUnderNumFiles > 0 &&
		numFilesOnDisk >= uint64(po.PersisterNapUnderNumFiles) &&
		lastMergedEpoch < lastPersistedEpoch {
		atomic.AddUint64(&s.stats.TotPersisterSlowMergerPause, 1)

		select {
		case <-s.closeCh:
			break OUTER
		case ew := <-s.persisterNotifier:
			persistWatchers = append(persistWatchers, ew)
			lastMergedEpoch = ew.epoch
		}

		atomic.AddUint64(&s.stats.TotPersisterSlowMergerResume, 1)

		// let the watchers proceed if they lag behind
		persistWatchers = notifyMergeWatchers(lastPersistedEpoch, persistWatchers)

		numFilesOnDisk, _, _ = s.diskFileStats(nil)
	}

	return lastMergedEpoch, persistWatchers
}

func (s *Scorch) parsePersisterOptions() (*persisterOptions, error) {
	po := persisterOptions{
		PersisterNapTimeMSec:         DefaultPersisterNapTimeMSec,
		PersisterNapUnderNumFiles:    DefaultPersisterNapUnderNumFiles,
		MemoryPressurePauseThreshold: DefaultMemoryPressurePauseThreshold,
	}
	if v, ok := s.config["scorchPersisterOptions"]; ok {
		b, err := json.Marshal(v)
		if err != nil {
			return &po, err
		}

		err = json.Unmarshal(b, &po)
		if err != nil {
			return &po, err
		}
	}
	return &po, nil
}

func (s *Scorch) persistSnapshot(snapshot *IndexSnapshot,
	po *persisterOptions) error {
	// Perform in-memory segment merging only when the memory pressure is
	// below the configured threshold, else the persister performs the
	// direct persistence of segments.
	if s.NumEventsBlocking() < po.MemoryPressurePauseThreshold {
		persisted, err := s.persistSnapshotMaybeMerge(snapshot)
		if err != nil {
			return err
		}
		if persisted {
			return nil
		}
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
	var sbs []segment.Segment
	var sbsDrops []*roaring.Bitmap
	var sbsIndexes []int

	for i, segmentSnapshot := range snapshot.segment {
		if _, ok := segmentSnapshot.segment.(segment.PersistedSegment); !ok {
			sbs = append(sbs, segmentSnapshot.segment)
			sbsDrops = append(sbsDrops, segmentSnapshot.deleted)
			sbsIndexes = append(sbsIndexes, i)
		}
	}

	if len(sbs) < DefaultMinSegmentsForInMemoryMerge {
		return false, nil
	}

	newSnapshot, newSegmentID, err := s.mergeSegmentBases(
		snapshot, sbs, sbsDrops, sbsIndexes)
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
		creator:  "persistSnapshotMaybeMerge",
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

func prepareBoltSnapshot(snapshot *IndexSnapshot, tx *bolt.Tx, path string,
	segPlugin segment.Plugin) ([]string, map[uint64]string, error) {
	snapshotsBucket, err := tx.CreateBucketIfNotExists(boltSnapshotsBucket)
	if err != nil {
		return nil, nil, err
	}
	newSnapshotKey := segment.EncodeUvarintAscending(nil, snapshot.epoch)
	snapshotBucket, err := snapshotsBucket.CreateBucketIfNotExists(newSnapshotKey)
	if err != nil {
		return nil, nil, err
	}

	// persist meta values
	metaBucket, err := snapshotBucket.CreateBucketIfNotExists(boltMetaDataKey)
	if err != nil {
		return nil, nil, err
	}
	err = metaBucket.Put(boltMetaDataSegmentTypeKey, []byte(segPlugin.Type()))
	if err != nil {
		return nil, nil, err
	}
	buf := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(buf, segPlugin.Version())
	err = metaBucket.Put(boltMetaDataSegmentVersionKey, buf)
	if err != nil {
		return nil, nil, err
	}

	// persist internal values
	internalBucket, err := snapshotBucket.CreateBucketIfNotExists(boltInternalKey)
	if err != nil {
		return nil, nil, err
	}
	// TODO optimize writing these in order?
	for k, v := range snapshot.internal {
		err = internalBucket.Put([]byte(k), v)
		if err != nil {
			return nil, nil, err
		}
	}

	var filenames []string
	newSegmentPaths := make(map[uint64]string)

	// first ensure that each segment in this snapshot has been persisted
	for _, segmentSnapshot := range snapshot.segment {
		snapshotSegmentKey := segment.EncodeUvarintAscending(nil, segmentSnapshot.id)
		snapshotSegmentBucket, err := snapshotBucket.CreateBucketIfNotExists(snapshotSegmentKey)
		if err != nil {
			return nil, nil, err
		}
		switch seg := segmentSnapshot.segment.(type) {
		case segment.PersistedSegment:
			segPath := seg.Path()
			filename := strings.TrimPrefix(segPath, path+string(os.PathSeparator))
			err = snapshotSegmentBucket.Put(boltPathKey, []byte(filename))
			if err != nil {
				return nil, nil, err
			}
			filenames = append(filenames, filename)
		case segment.UnpersistedSegment:
			// need to persist this to disk
			filename := zapFileName(segmentSnapshot.id)
			path := path + string(os.PathSeparator) + filename
			err = seg.Persist(path)
			if err != nil {
				return nil, nil, fmt.Errorf("error persisting segment: %v", err)
			}
			newSegmentPaths[segmentSnapshot.id] = path
			err = snapshotSegmentBucket.Put(boltPathKey, []byte(filename))
			if err != nil {
				return nil, nil, err
			}
			filenames = append(filenames, filename)
		default:
			return nil, nil, fmt.Errorf("unknown segment type: %T", seg)
		}
		// store current deleted bits
		var roaringBuf bytes.Buffer
		if segmentSnapshot.deleted != nil {
			_, err = segmentSnapshot.deleted.WriteTo(&roaringBuf)
			if err != nil {
				return nil, nil, fmt.Errorf("error persisting roaring bytes: %v", err)
			}
			err = snapshotSegmentBucket.Put(boltDeletedKey, roaringBuf.Bytes())
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return filenames, newSegmentPaths, nil
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

	filenames, newSegmentPaths, err := prepareBoltSnapshot(snapshot, tx, s.path, s.segPlugin)
	if err != nil {
		return err
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
			newSegments[segmentID], err = s.segPlugin.Open(path)
			if err != nil {
				return fmt.Errorf("error opening new segment at %s, %v", path, err)
			}
		}

		persist := &persistIntroduction{
			persisted: newSegments,
			applied:   make(notificationChan),
		}

		select {
		case <-s.closeCh:
			return segment.ErrClosed
		case s.persists <- persist:
		}

		select {
		case <-s.closeCh:
			return segment.ErrClosed
		case <-persist.applied:
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
var boltMetaDataKey = []byte{'m'}
var boltMetaDataSegmentTypeKey = []byte("type")
var boltMetaDataSegmentVersionKey = []byte("version")

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
				s.AddEligibleForRemoval(snapshotEpoch)
				continue
			}
			snapshot := snapshots.Bucket(k)
			if snapshot == nil {
				log.Printf("snapshot key, but bucket missing %x, continuing", k)
				s.AddEligibleForRemoval(snapshotEpoch)
				continue
			}
			indexSnapshot, err := s.loadSnapshot(snapshot)
			if err != nil {
				log.Printf("unable to load snapshot, %v, continuing", err)
				s.AddEligibleForRemoval(snapshotEpoch)
				continue
			}
			indexSnapshot.epoch = snapshotEpoch
			// set the nextSegmentID
			s.nextSegmentID, err = s.maxSegmentIDOnDisk()
			if err != nil {
				return err
			}
			s.nextSegmentID++
			s.rootLock.Lock()
			s.nextSnapshotEpoch = snapshotEpoch + 1
			rootPrev := s.root
			s.root = indexSnapshot
			s.rootLock.Unlock()

			if rootPrev != nil {
				_ = rootPrev.DecRef()
			}

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
			return fmt.Errorf("snapshot with epoch: %v - doesn't exist", epoch)
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
		creator:  "loadSnapshot",
	}
	// first we look for the meta-data bucket, this will tell us
	// which segment type/version was used for this snapshot
	// all operations for this scorch will use this type/version
	metaBucket := snapshot.Bucket(boltMetaDataKey)
	if metaBucket == nil {
		_ = rv.DecRef()
		return nil, fmt.Errorf("meta-data bucket missing")
	}
	segmentType := string(metaBucket.Get(boltMetaDataSegmentTypeKey))
	segmentVersion := binary.BigEndian.Uint32(
		metaBucket.Get(boltMetaDataSegmentVersionKey))
	err := s.loadSegmentPlugin(segmentType, segmentVersion)
	if err != nil {
		_ = rv.DecRef()
		return nil, fmt.Errorf(
			"unable to load correct segment wrapper: %v", err)
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
		} else if k[0] != boltMetaDataKey[0] {
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
	segment, err := s.segPlugin.Open(segmentPath)
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
		if !deletedBitmap.IsEmpty() {
			rv.deleted = deletedBitmap
		}
	}

	return rv, nil
}

func (s *Scorch) removeOldData() {
	removed, err := s.removeOldBoltSnapshots()
	if err != nil {
		s.fireAsyncError(fmt.Errorf("got err removing old bolt snapshots: %v", err))
	}
	atomic.AddUint64(&s.stats.TotSnapshotsRemovedFromMetaStore, uint64(removed))

	err = s.removeOldZapFiles()
	if err != nil {
		s.fireAsyncError(fmt.Errorf("got err removing old zap files: %v", err))
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

	if len(persistedEpochs) <= s.numSnapshotsToKeep {
		// we need to keep everything
		return 0, nil
	}

	// make a map of epochs to protect from deletion
	protectedEpochs := make(map[uint64]struct{}, s.numSnapshotsToKeep)
	for _, epoch := range persistedEpochs[0:s.numSnapshotsToKeep] {
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

	if len(epochsToRemove) == 0 {
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
