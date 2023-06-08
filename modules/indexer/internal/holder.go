// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"sync"
)

type IndexerHolder struct {
	indexer Indexer
	mutex   sync.RWMutex
	cond    *sync.Cond
}

func NewIndexerHolder() *IndexerHolder {
	h := &IndexerHolder{}
	h.cond = sync.NewCond(h.mutex.RLocker())
	return h
}

func (h *IndexerHolder) Set(indexer Indexer) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.indexer = indexer
	h.cond.Broadcast()
}

// Get returns the indexer, blocking until it is set
// It never returns nil
func (h *IndexerHolder) Get() Indexer {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	for h.indexer == nil { // make sure it never return nil even called Set(nil)
		h.cond.Wait()
	}
	return h.indexer
}
