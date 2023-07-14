// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/indexer/issues/db"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
)

// reFilter filters the given issuesIDs by the database.
// It is used to filter out issues coming from some indexers that are not supported fining filtering.
// Once all indexers support filtering, this function can be removed.
func reFilter(ctx context.Context, issuesIDs []int64, options *SearchOptions) ([]int64, error) {
	if reFilterFunc != nil {
		return reFilterFunc(ctx, issuesIDs, options)
	}
	return nil, fmt.Errorf("reFilterFunc is not registered")
}

var reFilterFunc func(ctx context.Context, issuesIDs []int64, options *SearchOptions) ([]int64, error)

// Why not just put the function body here to avoid RegisterReFilterFunc and RegisterDBSearch?
// Because modules can't depend on models by design.
// Although some packages have broken this rule, it's still a good practice to follow it.

// RegisterReFilterFunc registers a function to filter issues by database.
// It's for issue_model to register its own filter function.
func RegisterReFilterFunc(f func(ctx context.Context, issuesIDs []int64, options *SearchOptions) ([]int64, error)) {
	reFilterFunc = f
}

func RegisterDBSearch(f func(ctx context.Context, options *SearchOptions) ([]int64, int64, error)) {
	db.SearchFunc = func(ctx context.Context, options *internal.SearchOptions) (*internal.SearchResult, error) {
		ids, total, err := f(ctx, (*SearchOptions)(options))
		if err != nil {
			return nil, err
		}
		hits := make([]internal.Match, 0, len(ids))
		for _, id := range ids {
			hits = append(hits, internal.Match{
				ID: id,
			})
		}
		return &internal.SearchResult{
			Hits:      hits,
			Total:     total,
			Imprecise: false,
		}, nil
	}
}
