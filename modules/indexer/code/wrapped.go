// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"fmt"
	"sync"
)

var (
	indexer = newWrappedIndexer()
)

// ErrWrappedIndexerClosed is the error returned if the indexer was closed before it was ready
var ErrWrappedIndexerClosed = fmt.Errorf("Indexer closed before ready")

type wrappedIndexer struct {
	internal Indexer
	lock     sync.RWMutex
	cond     *sync.Cond
	closed   bool
}

func newWrappedIndexer() *wrappedIndexer {
	w := &wrappedIndexer{}
	w.cond = sync.NewCond(w.lock.RLocker())
	return w
}

func (w *wrappedIndexer) set(indexer Indexer) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if w.closed {
		// Too late!
		indexer.Close()
	}
	w.internal = indexer
	w.cond.Broadcast()
}

func (w *wrappedIndexer) get() (Indexer, error) {
	w.lock.RLock()
	defer w.lock.RUnlock()
	if w.internal == nil {
		if w.closed {
			return nil, ErrWrappedIndexerClosed
		}
		w.cond.Wait()
		if w.closed {
			return nil, ErrWrappedIndexerClosed
		}
	}
	return w.internal, nil
}

func (w *wrappedIndexer) Index(repoID int64) error {
	indexer, err := w.get()
	if err != nil {
		return err
	}
	return indexer.Index(repoID)
}

func (w *wrappedIndexer) Delete(repoID int64) error {
	indexer, err := w.get()
	if err != nil {
		return err
	}
	return indexer.Delete(repoID)
}

func (w *wrappedIndexer) Search(repoIDs []int64, keyword string, page, pageSize int) (int64, []*SearchResult, error) {
	indexer, err := w.get()
	if err != nil {
		return 0, nil, err
	}
	return indexer.Search(repoIDs, keyword, page, pageSize)

}

func (w *wrappedIndexer) Close() {
	w.lock.Lock()
	defer w.lock.Unlock()
	if w.closed {
		return
	}
	w.closed = true
	w.cond.Broadcast()
	if w.internal != nil {
		w.internal.Close()
	}
}
