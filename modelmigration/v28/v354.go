// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"
	"slices"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm/schemas"
)

// AddRepoSHAIndexToCommitStatus adds a (repo_id, sha) composite index to commit_status.
// The existing unique index leads with "index" so lookups by repo and commit could only
// use a single-column index, which is far too wide on busy instances.
func AddRepoSHAIndexToCommitStatus(ctx context.Context, x base.EngineMigration) error {
	indexes, err := x.Dialect().GetIndexes(x.DB(), ctx, "commit_status")
	if err != nil {
		return err
	}
	for _, idx := range indexes {
		if slices.Equal(idx.Cols, []string{"repo_id", "sha"}) {
			return nil // an equivalent index was already created, possibly by hand as a workaround
		}
	}

	newIndex := schemas.NewIndex("IDX_commit_status_repo_sha", schemas.IndexType)
	newIndex.AddColumn("repo_id", "sha")
	_, err = x.Exec(x.Dialect().CreateIndexSQL("commit_status", newIndex))
	return err
}
