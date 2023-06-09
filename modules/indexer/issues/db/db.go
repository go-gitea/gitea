// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/indexer/internal"
	inner_db "code.gitea.io/gitea/modules/indexer/internal/db"
	"code.gitea.io/gitea/modules/indexer/issues/base"
)

var _ base.Indexer = &Indexer{}

// Indexer implements Indexer interface to use database's like search
type Indexer struct {
	internal.Indexer
}

func NewIndexer() *Indexer {
	return &Indexer{
		Indexer: &inner_db.Indexer{},
	}
}

// Index dummy function
func (i *Indexer) Index(issue []*base.IndexerData) error {
	return nil
}

// Delete dummy function
func (i *Indexer) Delete(ids ...int64) error {
	return nil
}

// Search searches for issues
func (i *Indexer) Search(ctx context.Context, kw string, repoIDs []int64, limit, start int) (*base.SearchResult, error) {
	total, ids, err := issues_model.SearchIssueIDsByKeyword(ctx, kw, repoIDs, limit, start)
	if err != nil {
		return nil, err
	}
	result := base.SearchResult{
		Total: total,
		Hits:  make([]base.Match, 0, limit),
	}
	for _, id := range ids {
		result.Hits = append(result.Hits, base.Match{
			ID: id,
		})
	}
	return &result, nil
}
