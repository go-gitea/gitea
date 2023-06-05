// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
)

// DBIndexer implements Indexer interface to use database's like search
type DBIndexer struct{}

// Init dummy function
func (i *DBIndexer) Init() (bool, error) {
	return false, nil
}

// Ping checks if database is available
func (i *DBIndexer) Ping() bool {
	return db.GetEngine(db.DefaultContext).Ping() != nil
}

// Index dummy function
func (i *DBIndexer) Index(issue []*IndexerData) error {
	return nil
}

// Delete dummy function
func (i *DBIndexer) Delete(ids ...int64) error {
	return nil
}

// Close dummy function
func (i *DBIndexer) Close() {
}

// Search dummy function
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
