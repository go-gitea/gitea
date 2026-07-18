// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
	"sync"

	"gitea.dev/modules/log"
)

const isGogit = false

type Repository struct {
	RepositoryBase

	mu                 sync.Mutex
	catFileBatchCloser CatFileBatchCloser
	catFileBatchInUse  bool
}

func openRepositoryInternal(_ *Repository) error {
	return nil
}

// CatFileBatch obtains a "batch object provider" for this repository.
// It reuses an existing one if available, otherwise creates a new one.
func (repo *Repository) CatFileBatch(ctx context.Context) (_ CatFileBatch, closeFunc func(), err error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if repo.catFileBatchCloser != nil && !repo.catFileBatchInUse {
		if ctx != repo.catFileBatchCloser.Context() {
			repo.catFileBatchCloser.Close()
			repo.catFileBatchCloser = nil
			repo.catFileBatchInUse = false
		}
	}

	if repo.catFileBatchCloser == nil {
		repo.catFileBatchCloser, err = NewBatch(ctx, repo)
		if err != nil {
			repo.catFileBatchCloser = nil // otherwise it is "interface(nil)" and will cause wrong logic
			return nil, nil, err
		}
	}

	if !repo.catFileBatchInUse {
		repo.catFileBatchInUse = true
		return CatFileBatch(repo.catFileBatchCloser), func() {
			repo.mu.Lock()
			defer repo.mu.Unlock()
			repo.catFileBatchInUse = false
		}, nil
	}

	log.Debug("Opening temporary cat file batch for: %s", repo.Path)
	tempBatch, err := NewBatch(ctx, repo)
	if err != nil {
		return nil, nil, err
	}
	return tempBatch, tempBatch.Close, nil
}

func (repo *Repository) closeInternal() error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.catFileBatchCloser != nil {
		repo.catFileBatchCloser.Close()
		repo.catFileBatchCloser = nil
		repo.catFileBatchInUse = false
	}
	return nil
}
