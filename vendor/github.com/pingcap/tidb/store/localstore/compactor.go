// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package localstore

import (
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/store/localstore/engine"
	"github.com/pingcap/tidb/terror"
	"github.com/pingcap/tidb/util/bytes"
)

const (
	deleteWorkerCnt = 3
)

// compactPolicy defines gc policy of MVCC storage.
type compactPolicy struct {
	// SafePoint specifies
	SafePoint int
	// TriggerInterval specifies how often should the compactor
	// scans outdated data.
	TriggerInterval time.Duration
	// BatchDeleteCnt specifies the batch size for
	// deleting outdated data transaction.
	BatchDeleteCnt int
}

var localCompactDefaultPolicy = compactPolicy{
	SafePoint:       20 * 1000, // in ms
	TriggerInterval: 10 * time.Second,
	BatchDeleteCnt:  100,
}

type localstoreCompactor struct {
	mu              sync.Mutex
	recentKeys      map[string]struct{}
	stopCh          chan struct{}
	delCh           chan kv.EncodedKey
	workerWaitGroup *sync.WaitGroup
	ticker          *time.Ticker
	db              engine.DB
	policy          compactPolicy
}

func (gc *localstoreCompactor) OnSet(k kv.Key) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.recentKeys[string(k)] = struct{}{}
}

func (gc *localstoreCompactor) OnDelete(k kv.Key) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.recentKeys[string(k)] = struct{}{}
}

func (gc *localstoreCompactor) getAllVersions(key kv.Key) ([]kv.EncodedKey, error) {
	var keys []kv.EncodedKey
	k := key
	for ver := kv.MaxVersion; ver.Ver > 0; ver.Ver-- {
		mvccK, _, err := gc.db.Seek(MvccEncodeVersionKey(key, ver))
		if terror.ErrorEqual(err, engine.ErrNotFound) {
			break
		}
		if err != nil {
			return nil, errors.Trace(err)
		}
		k, ver, err = MvccDecode(mvccK)
		if k.Cmp(key) != 0 {
			break
		}
		if err != nil {
			return nil, errors.Trace(err)
		}
		keys = append(keys, bytes.CloneBytes(mvccK))
	}
	return keys, nil
}

func (gc *localstoreCompactor) deleteWorker() {
	defer gc.workerWaitGroup.Done()
	cnt := 0
	batch := gc.db.NewBatch()
	for {
		select {
		case <-gc.stopCh:
			return
		case key := <-gc.delCh:
			cnt++
			batch.Delete(key)
			// Batch delete.
			if cnt == gc.policy.BatchDeleteCnt {
				log.Debugf("[kv] GC delete commit %d keys", batch.Len())
				err := gc.db.Commit(batch)
				if err != nil {
					log.Error(err)
				}
				batch = gc.db.NewBatch()
				cnt = 0
			}
		}
	}
}

func (gc *localstoreCompactor) checkExpiredKeysWorker() {
	defer gc.workerWaitGroup.Done()
	for {
		select {
		case <-gc.stopCh:
			log.Debug("[kv] GC stopped")
			return
		case <-gc.ticker.C:
			gc.mu.Lock()
			m := gc.recentKeys
			if len(m) == 0 {
				gc.mu.Unlock()
				continue
			}
			gc.recentKeys = make(map[string]struct{})
			gc.mu.Unlock()
			for k := range m {
				err := gc.Compact([]byte(k))
				if err != nil {
					log.Error(err)
				}
			}
		}
	}
}

func (gc *localstoreCompactor) filterExpiredKeys(keys []kv.EncodedKey) []kv.EncodedKey {
	var ret []kv.EncodedKey
	first := true
	currentTS := time.Now().UnixNano() / int64(time.Millisecond)
	// keys are always in descending order.
	for _, k := range keys {
		_, ver, err := MvccDecode(k)
		if err != nil {
			// Should not happen.
			panic(err)
		}
		ts := localVersionToTimestamp(ver)
		// Check timeout keys.
		if currentTS-int64(ts) >= int64(gc.policy.SafePoint) {
			// Skip first version.
			if first {
				first = false
				continue
			}
			ret = append(ret, k)
		}
	}
	return ret
}

func (gc *localstoreCompactor) Compact(k kv.Key) error {
	keys, err := gc.getAllVersions(k)
	if err != nil {
		return errors.Trace(err)
	}
	filteredKeys := gc.filterExpiredKeys(keys)
	if len(filteredKeys) > 0 {
		log.Debugf("[kv] GC send %d keys to delete worker", len(filteredKeys))
	}
	for _, key := range filteredKeys {
		gc.delCh <- key
	}
	return nil
}

func (gc *localstoreCompactor) Start() {
	// Start workers.
	gc.workerWaitGroup.Add(deleteWorkerCnt)
	for i := 0; i < deleteWorkerCnt; i++ {
		go gc.deleteWorker()
	}

	gc.workerWaitGroup.Add(1)
	go gc.checkExpiredKeysWorker()
}

func (gc *localstoreCompactor) Stop() {
	gc.ticker.Stop()
	close(gc.stopCh)
	// Wait for all workers to finish.
	gc.workerWaitGroup.Wait()
}

func newLocalCompactor(policy compactPolicy, db engine.DB) *localstoreCompactor {
	return &localstoreCompactor{
		recentKeys:      make(map[string]struct{}),
		stopCh:          make(chan struct{}),
		delCh:           make(chan kv.EncodedKey, 100),
		ticker:          time.NewTicker(policy.TriggerInterval),
		policy:          policy,
		db:              db,
		workerWaitGroup: &sync.WaitGroup{},
	}
}
