// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"context"
	"fmt"
	"sync"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
)

var indexer = newWrappedIndexer()

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

// Ping checks if elastic is available
func (w *wrappedIndexer) Ping() bool {
	indexer, err := w.get()
	if err != nil {
		log.Warn("Failed to get indexer: %v", err)
		return false
	}
	return indexer.Ping()
}

func (w *wrappedIndexer) Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *repoChanges) error {
	indexer, err := w.get()
	if err != nil {
		return err
	}
	return indexer.Index(ctx, repo, sha, changes)
}

func (w *wrappedIndexer) Delete(repoID int64) error {
	indexer, err := w.get()
	if err != nil {
		return err
	}
	return indexer.Delete(repoID)
}

func (w *wrappedIndexer) Search(ctx context.Context, repoIDs []int64, language, keyword string, page, pageSize int, isMatch bool) (int64, []*SearchResult, []*SearchResultLanguages, error) {
	indexer, err := w.get()
	if err != nil {
		return 0, nil, nil, err
	}
	return indexer.Search(ctx, repoIDs, language, keyword, page, pageSize, isMatch)
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
