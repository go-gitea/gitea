// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import "sync"

type IndexerHolder[T Indexer] struct {
	indexer T
	mutex   sync.RWMutex
	cond    *sync.Cond
}

func NewIndexerHolder[T Indexer](_ T) *IndexerHolder[T] {
	h := &IndexerHolder[T]{}
	h.cond = sync.NewCond(h.mutex.RLocker())
	return h
}

func (h *IndexerHolder[T]) Set(indexer T) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.indexer = indexer
	h.cond.Broadcast()
}

func (h *IndexerHolder[T]) Get() T {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	if h.indexer == nil {
		h.cond.Wait()
	}
	return h.indexer
}
