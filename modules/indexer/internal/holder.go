// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import "sync"

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

func (h *IndexerHolder) Get() Indexer {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	if h.indexer == nil {
		h.cond.Wait()
	}
	return h.indexer
}
