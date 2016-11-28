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

package autoid

import (
	"sync"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta"
)

const (
	step = 1000
)

// Allocator is an auto increment id generator.
// Just keep id unique actually.
type Allocator interface {
	// Alloc allocs the next autoID for table with tableID.
	// It gets a batch of autoIDs at a time. So it does not need to access storage for each call.
	Alloc(tableID int64) (int64, error)
	// Rebase rebases the autoID base for table with tableID and the new base value.
	// If allocIDs is true, it will allocate some IDs and save to the cache.
	// If allocIDs is false, it will not allocate IDs.
	Rebase(tableID, newBase int64, allocIDs bool) error
}

type allocator struct {
	mu    sync.Mutex
	base  int64
	end   int64
	store kv.Storage
	dbID  int64
}

// Rebase implements autoid.Allocator Rebase interface.
func (alloc *allocator) Rebase(tableID, newBase int64, allocIDs bool) error {
	if tableID == 0 {
		return errors.New("Invalid tableID")
	}

	alloc.mu.Lock()
	defer alloc.mu.Unlock()
	if newBase <= alloc.base {
		return nil
	}
	if newBase <= alloc.end {
		alloc.base = newBase
		return nil
	}

	return kv.RunInNewTxn(alloc.store, true, func(txn kv.Transaction) error {
		m := meta.NewMeta(txn)
		end, err := m.GetAutoTableID(alloc.dbID, tableID)
		if err != nil {
			return errors.Trace(err)
		}

		if newBase <= end {
			return nil
		}
		newStep := newBase - end + step
		if !allocIDs {
			newStep = newBase - end
		}
		end, err = m.GenAutoTableID(alloc.dbID, tableID, newStep)
		if err != nil {
			return errors.Trace(err)
		}

		alloc.end = end
		alloc.base = newBase
		if !allocIDs {
			alloc.base = alloc.end
		}
		return nil
	})
}

// Alloc implements autoid.Allocator Alloc interface.
func (alloc *allocator) Alloc(tableID int64) (int64, error) {
	if tableID == 0 {
		return 0, errors.New("Invalid tableID")
	}
	alloc.mu.Lock()
	defer alloc.mu.Unlock()
	if alloc.base == alloc.end { // step
		err := kv.RunInNewTxn(alloc.store, true, func(txn kv.Transaction) error {
			m := meta.NewMeta(txn)
			base, err1 := m.GetAutoTableID(alloc.dbID, tableID)
			if err1 != nil {
				return errors.Trace(err1)
			}
			end, err1 := m.GenAutoTableID(alloc.dbID, tableID, step)
			if err1 != nil {
				return errors.Trace(err1)
			}

			alloc.end = end
			if end == step {
				alloc.base = base
			} else {
				alloc.base = end - step
			}
			return nil
		})

		if err != nil {
			return 0, errors.Trace(err)
		}
	}

	alloc.base++
	log.Debugf("[kv] Alloc id %d, table ID:%d, from %p, database ID:%d", alloc.base, tableID, alloc, alloc.dbID)
	return alloc.base, nil
}

var (
	memID     int64
	memIDLock sync.Mutex
)

type memoryAllocator struct {
	mu   sync.Mutex
	base int64
	end  int64
	dbID int64
}

// Rebase implements autoid.Allocator Rebase interface.
func (alloc *memoryAllocator) Rebase(tableID, newBase int64, allocIDs bool) error {
	// TODO: implement it.
	return nil
}

// Alloc implements autoid.Allocator Alloc interface.
func (alloc *memoryAllocator) Alloc(tableID int64) (int64, error) {
	if tableID == 0 {
		return 0, errors.New("Invalid tableID")
	}
	alloc.mu.Lock()
	defer alloc.mu.Unlock()
	if alloc.base == alloc.end { // step
		memIDLock.Lock()
		memID = memID + step
		alloc.end = memID
		alloc.base = alloc.end - step
		memIDLock.Unlock()
	}
	alloc.base++
	return alloc.base, nil
}

// NewAllocator returns a new auto increment id generator on the store.
func NewAllocator(store kv.Storage, dbID int64) Allocator {
	return &allocator{
		store: store,
		dbID:  dbID,
	}
}

// NewMemoryAllocator returns a new auto increment id generator in memory.
func NewMemoryAllocator(dbID int64) Allocator {
	return &memoryAllocator{
		dbID: dbID,
	}
}
