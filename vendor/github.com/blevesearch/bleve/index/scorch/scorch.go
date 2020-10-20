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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/blevesearch/bleve/analysis"
	"github.com/blevesearch/bleve/document"
	"github.com/blevesearch/bleve/index"
	"github.com/blevesearch/bleve/index/scorch/segment"
	"github.com/blevesearch/bleve/index/store"
	"github.com/blevesearch/bleve/registry"
	bolt "go.etcd.io/bbolt"
)

const Name = "scorch"

const Version uint8 = 2

var ErrClosed = fmt.Errorf("scorch closed")

type Scorch struct {
	nextSegmentID uint64
	stats         Stats
	iStats        internalStats

	readOnly      bool
	version       uint8
	config        map[string]interface{}
	analysisQueue *index.AnalysisQueue
	path          string

	unsafeBatch bool

	rootLock             sync.RWMutex
	root                 *IndexSnapshot // holds 1 ref-count on the root
	rootPersisted        []chan error   // closed when root is persisted
	persistedCallbacks   []index.BatchCallback
	nextSnapshotEpoch    uint64
	eligibleForRemoval   []uint64        // Index snapshot epochs that are safe to GC.
	ineligibleForRemoval map[string]bool // Filenames that should not be GC'ed yet.

	numSnapshotsToKeep int
	closeCh            chan struct{}
	introductions      chan *segmentIntroduction
	persists           chan *persistIntroduction
	merges             chan *segmentMerge
	introducerNotifier chan *epochWatcher
	persisterNotifier  chan *epochWatcher
	rootBolt           *bolt.DB
	asyncTasks         sync.WaitGroup

	onEvent      func(event Event)
	onAsyncError func(err error)

	forceMergeRequestCh chan *mergerCtrl

	segPlugin segment.Plugin
}

type internalStats struct {
	persistEpoch          uint64
	persistSnapshotSize   uint64
	mergeEpoch            uint64
	mergeSnapshotSize     uint64
	newSegBufBytesAdded   uint64
	newSegBufBytesRemoved uint64
	analysisBytesAdded    uint64
	analysisBytesRemoved  uint64
}

func NewScorch(storeName string,
	config map[string]interface{},
	analysisQueue *index.AnalysisQueue) (index.Index, error) {
	rv := &Scorch{
		version:              Version,
		config:               config,
		analysisQueue:        analysisQueue,
		nextSnapshotEpoch:    1,
		closeCh:              make(chan struct{}),
		ineligibleForRemoval: map[string]bool{},
		forceMergeRequestCh:  make(chan *mergerCtrl, 1),
		segPlugin:            defaultSegmentPlugin,
	}

	forcedSegmentType, forcedSegmentVersion, err := configForceSegmentTypeVersion(config)
	if err != nil {
		return nil, err
	}
	if forcedSegmentType != "" && forcedSegmentVersion != 0 {
		err := rv.loadSegmentPlugin(forcedSegmentType,
			uint32(forcedSegmentVersion))
		if err != nil {
			return nil, err
		}
	}

	rv.root = &IndexSnapshot{parent: rv, refs: 1, creator: "NewScorch"}
	ro, ok := config["read_only"].(bool)
	if ok {
		rv.readOnly = ro
	}
	ub, ok := config["unsafe_batch"].(bool)
	if ok {
		rv.unsafeBatch = ub
	}
	ecbName, ok := config["eventCallbackName"].(string)
	if ok {
		rv.onEvent = RegistryEventCallbacks[ecbName]
	}
	aecbName, ok := config["asyncErrorCallbackName"].(string)
	if ok {
		rv.onAsyncError = RegistryAsyncErrorCallbacks[aecbName]
	}
	return rv, nil
}

// configForceSegmentTypeVersion checks if the caller has requested a
// specific segment type/version
func configForceSegmentTypeVersion(config map[string]interface{}) (string, uint32, error) {
	forcedSegmentVersion, err := parseToInteger(config["forceSegmentVersion"])
	if err != nil {
		return "", 0, nil
	}

	forcedSegmentType, ok := config["forceSegmentType"].(string)
	if !ok {
		return "", 0, fmt.Errorf(
			"forceSegmentVersion set to %d, must also specify forceSegmentType", forcedSegmentVersion)
	}

	return forcedSegmentType, uint32(forcedSegmentVersion), nil
}

func (s *Scorch) NumEventsBlocking() uint64 {
	eventsCompleted := atomic.LoadUint64(&s.stats.TotEventTriggerCompleted)
	eventsStarted := atomic.LoadUint64(&s.stats.TotEventTriggerStarted)
	return eventsStarted - eventsCompleted
}

func (s *Scorch) fireEvent(kind EventKind, dur time.Duration) {
	if s.onEvent != nil {
		atomic.AddUint64(&s.stats.TotEventTriggerStarted, 1)
		s.onEvent(Event{Kind: kind, Scorch: s, Duration: dur})
		atomic.AddUint64(&s.stats.TotEventTriggerCompleted, 1)
	}
}

func (s *Scorch) fireAsyncError(err error) {
	if s.onAsyncError != nil {
		s.onAsyncError(err)
	}
	atomic.AddUint64(&s.stats.TotOnErrors, 1)
}

func (s *Scorch) Open() error {
	err := s.openBolt()
	if err != nil {
		return err
	}

	s.asyncTasks.Add(1)
	go s.introducerLoop()

	if !s.readOnly && s.path != "" {
		s.asyncTasks.Add(1)
		go s.persisterLoop()
		s.asyncTasks.Add(1)
		go s.mergerLoop()
	}

	return nil
}

func (s *Scorch) openBolt() error {
	var ok bool
	s.path, ok = s.config["path"].(string)
	if !ok {
		return fmt.Errorf("must specify path")
	}
	if s.path == "" {
		s.unsafeBatch = true
	}

	var rootBoltOpt *bolt.Options
	if s.readOnly {
		rootBoltOpt = &bolt.Options{
			ReadOnly: true,
		}
	} else {
		if s.path != "" {
			err := os.MkdirAll(s.path, 0700)
			if err != nil {
				return err
			}
		}
	}

	rootBoltPath := s.path + string(os.PathSeparator) + "root.bolt"
	var err error
	if s.path != "" {
		s.rootBolt, err = bolt.Open(rootBoltPath, 0600, rootBoltOpt)
		if err != nil {
			return err
		}

		// now see if there is any existing state to load
		err = s.loadFromBolt()
		if err != nil {
			_ = s.Close()
			return err
		}
	}

	atomic.StoreUint64(&s.stats.TotFileSegmentsAtRoot, uint64(len(s.root.segment)))

	s.introductions = make(chan *segmentIntroduction)
	s.persists = make(chan *persistIntroduction)
	s.merges = make(chan *segmentMerge)
	s.introducerNotifier = make(chan *epochWatcher, 1)
	s.persisterNotifier = make(chan *epochWatcher, 1)
	s.closeCh = make(chan struct{})
	s.forceMergeRequestCh = make(chan *mergerCtrl, 1)

	if !s.readOnly && s.path != "" {
		err := s.removeOldZapFiles() // Before persister or merger create any new files.
		if err != nil {
			_ = s.Close()
			return err
		}
	}

	s.numSnapshotsToKeep = NumSnapshotsToKeep
	if v, ok := s.config["numSnapshotsToKeep"]; ok {
		var t int
		if t, err = parseToInteger(v); err != nil {
			return fmt.Errorf("numSnapshotsToKeep parse err: %v", err)
		}
		if t > 0 {
			s.numSnapshotsToKeep = t
		}
	}

	return nil
}

func (s *Scorch) Close() (err error) {
	startTime := time.Now()
	defer func() {
		s.fireEvent(EventKindClose, time.Since(startTime))
	}()

	s.fireEvent(EventKindCloseStart, 0)

	// signal to async tasks we want to close
	close(s.closeCh)
	// wait for them to close
	s.asyncTasks.Wait()
	// now close the root bolt
	if s.rootBolt != nil {
		err = s.rootBolt.Close()
		s.rootLock.Lock()
		if s.root != nil {
			err2 := s.root.DecRef()
			if err == nil {
				err = err2
			}
		}
		s.root = nil
		s.rootLock.Unlock()
	}

	return
}

func (s *Scorch) Update(doc *document.Document) error {
	b := index.NewBatch()
	b.Update(doc)
	return s.Batch(b)
}

func (s *Scorch) Delete(id string) error {
	b := index.NewBatch()
	b.Delete(id)
	return s.Batch(b)
}

// Batch applices a batch of changes to the index atomically
func (s *Scorch) Batch(batch *index.Batch) (err error) {
	start := time.Now()

	defer func() {
		s.fireEvent(EventKindBatchIntroduction, time.Since(start))
	}()

	resultChan := make(chan *index.AnalysisResult, len(batch.IndexOps))

	var numUpdates uint64
	var numDeletes uint64
	var numPlainTextBytes uint64
	var ids []string
	for docID, doc := range batch.IndexOps {
		if doc != nil {
			// insert _id field
			doc.AddField(document.NewTextFieldCustom("_id", nil, []byte(doc.ID), document.IndexField|document.StoreField, nil))
			numUpdates++
			numPlainTextBytes += doc.NumPlainTextBytes()
		} else {
			numDeletes++
		}
		ids = append(ids, docID)
	}

	// FIXME could sort ids list concurrent with analysis?

	if numUpdates > 0 {
		go func() {
			for _, doc := range batch.IndexOps {
				if doc != nil {
					aw := index.NewAnalysisWork(s, doc, resultChan)
					// put the work on the queue
					s.analysisQueue.Queue(aw)
				}
			}
		}()
	}

	// wait for analysis result
	analysisResults := make([]*index.AnalysisResult, int(numUpdates))
	var itemsDeQueued uint64
	var totalAnalysisSize int
	for itemsDeQueued < numUpdates {
		result := <-resultChan
		resultSize := result.Size()
		atomic.AddUint64(&s.iStats.analysisBytesAdded, uint64(resultSize))
		totalAnalysisSize += resultSize
		analysisResults[itemsDeQueued] = result
		itemsDeQueued++
	}
	close(resultChan)
	defer atomic.AddUint64(&s.iStats.analysisBytesRemoved, uint64(totalAnalysisSize))

	atomic.AddUint64(&s.stats.TotAnalysisTime, uint64(time.Since(start)))

	indexStart := time.Now()

	// notify handlers that we're about to introduce a segment
	s.fireEvent(EventKindBatchIntroductionStart, 0)

	var newSegment segment.Segment
	var bufBytes uint64
	if len(analysisResults) > 0 {
		newSegment, bufBytes, err = s.segPlugin.New(analysisResults)
		if err != nil {
			return err
		}
		atomic.AddUint64(&s.iStats.newSegBufBytesAdded, bufBytes)
	} else {
		atomic.AddUint64(&s.stats.TotBatchesEmpty, 1)
	}

	err = s.prepareSegment(newSegment, ids, batch.InternalOps, batch.PersistedCallback())
	if err != nil {
		if newSegment != nil {
			_ = newSegment.Close()
		}
		atomic.AddUint64(&s.stats.TotOnErrors, 1)
	} else {
		atomic.AddUint64(&s.stats.TotUpdates, numUpdates)
		atomic.AddUint64(&s.stats.TotDeletes, numDeletes)
		atomic.AddUint64(&s.stats.TotBatches, 1)
		atomic.AddUint64(&s.stats.TotIndexedPlainTextBytes, numPlainTextBytes)
	}

	atomic.AddUint64(&s.iStats.newSegBufBytesRemoved, bufBytes)
	atomic.AddUint64(&s.stats.TotIndexTime, uint64(time.Since(indexStart)))

	return err
}

func (s *Scorch) prepareSegment(newSegment segment.Segment, ids []string,
	internalOps map[string][]byte, persistedCallback index.BatchCallback) error {

	// new introduction
	introduction := &segmentIntroduction{
		id:                atomic.AddUint64(&s.nextSegmentID, 1),
		data:              newSegment,
		ids:               ids,
		obsoletes:         make(map[uint64]*roaring.Bitmap),
		internal:          internalOps,
		applied:           make(chan error),
		persistedCallback: persistedCallback,
	}

	if !s.unsafeBatch {
		introduction.persisted = make(chan error, 1)
	}

	// optimistically prepare obsoletes outside of rootLock
	s.rootLock.RLock()
	root := s.root
	root.AddRef()
	s.rootLock.RUnlock()

	defer func() { _ = root.DecRef() }()

	for _, seg := range root.segment {
		delta, err := seg.segment.DocNumbers(ids)
		if err != nil {
			return err
		}
		introduction.obsoletes[seg.id] = delta
	}

	introStartTime := time.Now()

	s.introductions <- introduction

	// block until this segment is applied
	err := <-introduction.applied
	if err != nil {
		return err
	}

	if introduction.persisted != nil {
		err = <-introduction.persisted
	}

	introTime := uint64(time.Since(introStartTime))
	atomic.AddUint64(&s.stats.TotBatchIntroTime, introTime)
	if atomic.LoadUint64(&s.stats.MaxBatchIntroTime) < introTime {
		atomic.StoreUint64(&s.stats.MaxBatchIntroTime, introTime)
	}

	return err
}

func (s *Scorch) SetInternal(key, val []byte) error {
	b := index.NewBatch()
	b.SetInternal(key, val)
	return s.Batch(b)
}

func (s *Scorch) DeleteInternal(key []byte) error {
	b := index.NewBatch()
	b.DeleteInternal(key)
	return s.Batch(b)
}

// Reader returns a low-level accessor on the index data. Close it to
// release associated resources.
func (s *Scorch) Reader() (index.IndexReader, error) {
	return s.currentSnapshot(), nil
}

func (s *Scorch) currentSnapshot() *IndexSnapshot {
	s.rootLock.RLock()
	rv := s.root
	if rv != nil {
		rv.AddRef()
	}
	s.rootLock.RUnlock()
	return rv
}

func (s *Scorch) Stats() json.Marshaler {
	return &s.stats
}

func (s *Scorch) diskFileStats(rootSegmentPaths map[string]struct{}) (uint64,
	uint64, uint64) {
	var numFilesOnDisk, numBytesUsedDisk, numBytesOnDiskByRoot uint64
	if s.path != "" {
		finfos, err := ioutil.ReadDir(s.path)
		if err == nil {
			for _, finfo := range finfos {
				if !finfo.IsDir() {
					numBytesUsedDisk += uint64(finfo.Size())
					numFilesOnDisk++
					if rootSegmentPaths != nil {
						fname := s.path + string(os.PathSeparator) + finfo.Name()
						if _, fileAtRoot := rootSegmentPaths[fname]; fileAtRoot {
							numBytesOnDiskByRoot += uint64(finfo.Size())
						}
					}
				}
			}
		}
	}
	// if no root files path given, then consider all disk files.
	if rootSegmentPaths == nil {
		return numFilesOnDisk, numBytesUsedDisk, numBytesUsedDisk
	}

	return numFilesOnDisk, numBytesUsedDisk, numBytesOnDiskByRoot
}

func (s *Scorch) StatsMap() map[string]interface{} {
	m := s.stats.ToMap()

	indexSnapshot := s.currentSnapshot()
	defer func() {
		_ = indexSnapshot.Close()
	}()

	rootSegPaths := indexSnapshot.diskSegmentsPaths()

	s.rootLock.RLock()
	m["CurFilesIneligibleForRemoval"] = uint64(len(s.ineligibleForRemoval))
	s.rootLock.RUnlock()

	numFilesOnDisk, numBytesUsedDisk, numBytesOnDiskByRoot := s.diskFileStats(rootSegPaths)

	m["CurOnDiskBytes"] = numBytesUsedDisk
	m["CurOnDiskFiles"] = numFilesOnDisk

	// TODO: consider one day removing these backwards compatible
	// names for apps using the old names
	m["updates"] = m["TotUpdates"]
	m["deletes"] = m["TotDeletes"]
	m["batches"] = m["TotBatches"]
	m["errors"] = m["TotOnErrors"]
	m["analysis_time"] = m["TotAnalysisTime"]
	m["index_time"] = m["TotIndexTime"]
	m["term_searchers_started"] = m["TotTermSearchersStarted"]
	m["term_searchers_finished"] = m["TotTermSearchersFinished"]
	m["num_plain_text_bytes_indexed"] = m["TotIndexedPlainTextBytes"]
	m["num_items_introduced"] = m["TotIntroducedItems"]
	m["num_items_persisted"] = m["TotPersistedItems"]
	m["num_recs_to_persist"] = m["TotItemsToPersist"]
	// total disk bytes found in index directory inclusive of older snapshots
	m["num_bytes_used_disk"] = numBytesUsedDisk
	// total disk bytes by the latest root index, exclusive of older snapshots
	m["num_bytes_used_disk_by_root"] = numBytesOnDiskByRoot
	// num_bytes_used_disk_by_root_reclaimable is an approximation about the
	// reclaimable disk space in an index. (eg: from a full compaction)
	m["num_bytes_used_disk_by_root_reclaimable"] = uint64(float64(numBytesOnDiskByRoot) *
		indexSnapshot.reClaimableDocsRatio())
	m["num_files_on_disk"] = numFilesOnDisk
	m["num_root_memorysegments"] = m["TotMemorySegmentsAtRoot"]
	m["num_root_filesegments"] = m["TotFileSegmentsAtRoot"]
	m["num_persister_nap_pause_completed"] = m["TotPersisterNapPauseCompleted"]
	m["num_persister_nap_merger_break"] = m["TotPersisterMergerNapBreak"]
	m["total_compaction_written_bytes"] = m["TotFileMergeWrittenBytes"]

	return m
}

func (s *Scorch) Analyze(d *document.Document) *index.AnalysisResult {
	return analyze(d)
}

func analyze(d *document.Document) *index.AnalysisResult {
	rv := &index.AnalysisResult{
		Document: d,
		Analyzed: make([]analysis.TokenFrequencies, len(d.Fields)+len(d.CompositeFields)),
		Length:   make([]int, len(d.Fields)+len(d.CompositeFields)),
	}

	for i, field := range d.Fields {
		if field.Options().IsIndexed() {
			fieldLength, tokenFreqs := field.Analyze()
			rv.Analyzed[i] = tokenFreqs
			rv.Length[i] = fieldLength

			if len(d.CompositeFields) > 0 && field.Name() != "_id" {
				// see if any of the composite fields need this
				for _, compositeField := range d.CompositeFields {
					compositeField.Compose(field.Name(), fieldLength, tokenFreqs)
				}
			}
		}
	}

	return rv
}

func (s *Scorch) Advanced() (store.KVStore, error) {
	return nil, nil
}

func (s *Scorch) AddEligibleForRemoval(epoch uint64) {
	s.rootLock.Lock()
	if s.root == nil || s.root.epoch != epoch {
		s.eligibleForRemoval = append(s.eligibleForRemoval, epoch)
	}
	s.rootLock.Unlock()
}

func (s *Scorch) MemoryUsed() (memUsed uint64) {
	indexSnapshot := s.currentSnapshot()
	if indexSnapshot == nil {
		return
	}

	defer func() {
		_ = indexSnapshot.Close()
	}()

	// Account for current root snapshot overhead
	memUsed += uint64(indexSnapshot.Size())

	// Account for snapshot that the persister may be working on
	persistEpoch := atomic.LoadUint64(&s.iStats.persistEpoch)
	persistSnapshotSize := atomic.LoadUint64(&s.iStats.persistSnapshotSize)
	if persistEpoch != 0 && indexSnapshot.epoch > persistEpoch {
		// the snapshot that the persister is working on isn't the same as
		// the current snapshot
		memUsed += persistSnapshotSize
	}

	// Account for snapshot that the merger may be working on
	mergeEpoch := atomic.LoadUint64(&s.iStats.mergeEpoch)
	mergeSnapshotSize := atomic.LoadUint64(&s.iStats.mergeSnapshotSize)
	if mergeEpoch != 0 && indexSnapshot.epoch > mergeEpoch {
		// the snapshot that the merger is working on isn't the same as
		// the current snapshot
		memUsed += mergeSnapshotSize
	}

	memUsed += (atomic.LoadUint64(&s.iStats.newSegBufBytesAdded) -
		atomic.LoadUint64(&s.iStats.newSegBufBytesRemoved))

	memUsed += (atomic.LoadUint64(&s.iStats.analysisBytesAdded) -
		atomic.LoadUint64(&s.iStats.analysisBytesRemoved))

	return memUsed
}

func (s *Scorch) markIneligibleForRemoval(filename string) {
	s.rootLock.Lock()
	s.ineligibleForRemoval[filename] = true
	s.rootLock.Unlock()
}

func (s *Scorch) unmarkIneligibleForRemoval(filename string) {
	s.rootLock.Lock()
	delete(s.ineligibleForRemoval, filename)
	s.rootLock.Unlock()
}

func init() {
	registry.RegisterIndexType(Name, NewScorch)
}

func parseToInteger(i interface{}) (int, error) {
	switch v := i.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil

	default:
		return 0, fmt.Errorf("expects int or float64 value")
	}
}
