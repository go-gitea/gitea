// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_db "code.gitea.io/gitea/modules/indexer/internal/db"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
)

var _ internal.Indexer = &Indexer{}

// Indexer implements Indexer interface to use database's like search
type Indexer struct {
	indexer_internal.Indexer
}

func NewIndexer() *Indexer {
	return &Indexer{
		Indexer: &inner_db.Indexer{},
	}
}

// Index dummy function
func (i *Indexer) Index(_ context.Context, _ []*internal.IndexerData) error {
	return nil
}

// Delete dummy function
func (i *Indexer) Delete(_ context.Context, _ ...int64) error {
	return nil
}

// Search searches for issues
func (i *Indexer) Search(ctx context.Context, kw string, repoIDs []int64, limit, start int) (*internal.SearchResult, error) {
	total, ids, err := issues_model.SearchIssueIDsByKeyword(ctx, kw, repoIDs, limit, start)
	if err != nil {
		return nil, err
	}
	result := internal.SearchResult{
		Total: total,
		Hits:  make([]internal.Match, 0, limit),
	}
	for _, id := range ids {
		result.Hits = append(result.Hits, internal.Match{
			ID: id,
		})
	}
	return &result, nil
}
