// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/indexer/internal"
)

// Indexer defines an interface to index and search code contents
type Indexer interface {
	internal.Indexer
	Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *RepoChanges) error
	Delete(ctx context.Context, repoID int64) error
	Search(ctx context.Context, opts *SearchOptions) (int64, []*SearchResult, []*SearchResultLanguages, error)
	SupportedSearchModes() []indexer.SearchMode
}

type SearchOptions struct {
	RepoIDs  []int64
	Keyword  string
	Language string

	SearchMode indexer.SearchModeType

	db.Paginator
}

// NewDummyIndexer returns a dummy indexer
func NewDummyIndexer() Indexer {
	return &dummyIndexer{
		Indexer: internal.NewDummyIndexer(),
	}
}

type dummyIndexer struct {
	internal.Indexer
}

func (d *dummyIndexer) SupportedSearchModes() []indexer.SearchMode {
	return nil
}

func (d *dummyIndexer) Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *RepoChanges) error {
	return fmt.Errorf("indexer is not ready")
}

func (d *dummyIndexer) Delete(ctx context.Context, repoID int64) error {
	return fmt.Errorf("indexer is not ready")
}

func (d *dummyIndexer) Search(ctx context.Context, opts *SearchOptions) (int64, []*SearchResult, []*SearchResultLanguages, error) {
	return 0, nil, nil, fmt.Errorf("indexer is not ready")
}
