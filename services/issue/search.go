// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"

	issues_model "code.gitea.io/gitea/models/issues"
	issues_indexer "code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// Search search issues with sort acorrding the given conditions
func Search(ctx context.Context, opts *issues_model.IssuesOptions) (int64, issues_model.IssueList, error) {
	if setting.Indexer.IssueType == "db" || opts.Keyword == "" {
		issues, err := issues_model.Issues(ctx, opts)
		if err != nil {
			return 0, nil, err
		}
		total, err := issues_model.CountIssues(ctx, opts)
		if err != nil {
			return 0, nil, err
		}
		return total, issues, nil
	}

	total, issueIDs, err := issues_indexer.Search(ctx, opts)
	if err != nil {
		return 0, nil, err
	}
	issues, err := issues_model.FindIssuesByIDs(ctx, issueIDs, util.OptionalBoolNone)
	if err != nil {
		return 0, nil, err
	}

	return total, issues, nil
}
