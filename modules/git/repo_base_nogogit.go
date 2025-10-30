// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

const isGogit = false

// Repository represents a Git repository.
type Repository struct {
	Path string

	tagCache *ObjectCache[*Tag]

	gpgSettings *GPGSettings

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

// FIXME: for debugging purpose only
// At the moment, the old logic (debugTestAlwaysNewBatch=false: reuse the existing batch if not in use)
// causes random test failures: it makes the `t.Context()` occasionally canceled with unknown reasons.
// In theory, the `t.Context()` should never be affected by testing code and can never be canceled, but it does happen.
// The stranger thing is that the failure tests are almost around TestAPIPullUpdateByRebase,
// it almost are during MSSQL testing, sometimes PGSQL, never others.

// CatFileBatch obtains a CatFileBatch for this repository
func (repo *Repository) CatFileBatch(ctx context.Context, optAlwaysNewBatch ...bool) (_ CatFileBatch, closeFunc func(), err error) {
	if util.OptionalArg(optAlwaysNewBatch) {
		b, err := NewBatch(ctx, repo.Path)
		return b, b.Close, err
	}

	if repo.catFileBatchCloser == nil {
		repo.catFileBatchCloser, err = NewBatch(ctx, repo.Path)
		if err != nil {
			return nil, nil, err
		}
	}

	if !repo.catFileBatchInUse {
		repo.catFileBatchInUse = true
		return CatFileBatch(repo.catFileBatchCloser), func() {
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
	if repo.catFileBatchCloser != nil {
		repo.catFileBatchCloser.Close()
		repo.catFileBatchCloser = nil
		repo.catFileBatchInUse = false
	}
	repo.LastCommitCache = nil
	repo.tagCache = nil
	return nil
}
