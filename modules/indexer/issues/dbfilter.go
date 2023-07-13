// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
)

// filterIssuesByDB filters the given issuesIDs by the database.
// It is used to filter out issues coming from some indexers that are not supported fining filtering.
// Once all indexers support filtering, this function can be removed.
func filterIssuesByDB(ctx context.Context, issuesIDs []int64, options *SearchOptions) ([]int64, error) {
	if filterIssuesFunc != nil {
		return filterIssuesFunc(ctx, issuesIDs, options)
	}
	return issuesIDs, nil
}

var filterIssuesFunc func(ctx context.Context, issuesIDs []int64, options *SearchOptions) ([]int64, error)

// RegisterFilterIssuesFunc registers a function to filter issues by database.
// It's for issue_model to register its own filter function.
// Why not just put the function body here?
// Because modules can't depend on models by design.
// Although some packages have broken this rule, it's still a good practice to follow it.
func RegisterFilterIssuesFunc(f func(ctx context.Context, issuesIDs []int64, options *SearchOptions) ([]int64, error)) {
	filterIssuesFunc = f
}
