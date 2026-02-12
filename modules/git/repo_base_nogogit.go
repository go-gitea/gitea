// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
	"path/filepath"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

const isGogit = false

// Repository represents a Git repository.
type Repository struct {
	Path string

	tagCache *ObjectCache[*Tag]

	mu                 sync.Mutex
	catFileBatchCloser CatFileBatchCloser
	catFileBatchInUse  bool

	Ctx             context.Context
	LastCommitCache *LastCommitCache

	objectFormat ObjectFormat
}

// OpenRepository opens the repository at the given path with the provided context.
func OpenRepository(ctx context.Context, repoPath string) (*Repository, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}
	exist, err := util.IsDir(repoPath)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, util.NewNotExistErrorf("no such file or directory")
	}

	return &Repository{
		Path:     repoPath,
		tagCache: newObjectCache[*Tag](),
		Ctx:      ctx,
	}, nil
}

// CatFileBatch obtains a "batch object provider" for this repository.
// It reuses an existing one if available, otherwise creates a new one.
func (repo *Repository) CatFileBatch(ctx context.Context) (_ CatFileBatch, closeFunc func(), err error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if repo.catFileBatchCloser == nil {
		repo.catFileBatchCloser, err = NewBatch(ctx, repo.Path)
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
	tempBatch, err := NewBatch(ctx, repo.Path)
	if err != nil {
		return nil, nil, err
	}
	return tempBatch, tempBatch.Close, nil
}

func (repo *Repository) Close() error {
	if repo == nil {
		return nil
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.catFileBatchCloser != nil {
		repo.catFileBatchCloser.Close()
		repo.catFileBatchCloser = nil
		repo.catFileBatchInUse = false
	}
	repo.LastCommitCache = nil
	repo.tagCache = nil
	return nil
}
