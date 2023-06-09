// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/indexer/internal"
	inner_db "code.gitea.io/gitea/modules/indexer/internal/db"
)

var _ Indexer = &DBIndexer{}

// DBIndexer implements Indexer interface to use database's like search
type DBIndexer struct {
	internal.Indexer
}

func NewDBIndexer() *DBIndexer {
	return &DBIndexer{
		Indexer: &inner_db.Indexer{},
	}
}

// Index dummy function
func (i *DBIndexer) Index(issue []*IndexerData) error {
	return nil
}

// Delete dummy function
func (i *DBIndexer) Delete(ids ...int64) error {
	return nil
}

// Search searches for issues
func (i *DBIndexer) Search(ctx context.Context, kw string, repoIDs []int64, limit, start int) (*SearchResult, error) {
	total, ids, err := issues_model.SearchIssueIDsByKeyword(ctx, kw, repoIDs, limit, start)
	if err != nil {
		return nil, err
	}
	result := SearchResult{
		Total: total,
		Hits:  make([]Match, 0, limit),
	}
	for _, id := range ids {
		result.Hits = append(result.Hits, Match{
			ID: id,
		})
	}
	return &result, nil
}
