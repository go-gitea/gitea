// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// IssueRedirect stores a mapping from an old (repo, index) to the current issue ID after a move.
type IssueRedirect struct {
	ID             int64 `xorm:"pk autoincr"`
	OriginalRepoID int64 `xorm:"INDEX NOT NULL"`
	OriginalIndex  int64 `xorm:"NOT NULL"`
	IssueID        int64 `xorm:"INDEX NOT NULL"`
}

func init() {
	db.RegisterModel(new(IssueRedirect))
}

// CreateIssueRedirect inserts a redirect record for a moved issue.
func CreateIssueRedirect(ctx context.Context, originalRepoID, originalIndex, issueID int64) error {
	_, err := db.GetEngine(ctx).Insert(&IssueRedirect{
		OriginalRepoID: originalRepoID,
		OriginalIndex:  originalIndex,
		IssueID:        issueID,
	})
	return err
}

// GetIssueRedirect returns the redirect record for the given original repo and index
func GetIssueRedirect(ctx context.Context, originalRepoID, originalIndex int64) (*IssueRedirect, error) {
	r, has, err := db.Get[IssueRedirect](ctx, builder.Eq{"original_repo_id": originalRepoID, "original_index": originalIndex})
	if err != nil {
		return nil, err
	} else if !has {
		return nil, util.NewNotExistErrorf("issue redirect for repo %d index %d does not exist", originalRepoID, originalIndex)
	}
	return r, nil
}
