// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	issue_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/indexer/issues/db"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
)

// reFilter filters the given issuesIDs by the database.
// It is used to filter out issues coming from some indexers that are not supported fining filtering.
// Once all indexers support filtering, this function and this file can be removed.
func reFilter(ctx context.Context, issuesIDs []int64, options *SearchOptions) ([]int64, error) {
	opts, err := db.ToDBOptions(ctx, (*internal.SearchOptions)(options))
	if err != nil {
		return nil, err
	}

	opts.IssueIDs = issuesIDs

	ids, _, err := issue_model.IssueIDs(ctx, opts)
	return ids, err
}
