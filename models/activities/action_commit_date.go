// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// UserHeatmapCommit stores an individual commit's author timestamp for heatmap display.
// Decoupled from the action table — keyed by user and repo instead of action ID.
type UserHeatmapCommit struct {
	ID              int64              `xorm:"pk autoincr"`
	UserID          int64              `xorm:"INDEX"`
	RepoID          int64              `xorm:"INDEX"`
	CommitSha1      string             `xorm:"VARCHAR(64)"`
	CommitTimestamp timeutil.TimeStamp `xorm:"INDEX"`
}

func init() {
	db.RegisterModel(new(UserHeatmapCommit))
}

// InsertUserHeatmapCommits inserts commit date records for heatmap display,
// filtering out commits with zero or negative timestamps.
func InsertUserHeatmapCommits(ctx context.Context, userID, repoID int64, commits []UserHeatmapCommit) error {
	if len(commits) == 0 {
		return nil
	}

	records := make([]*UserHeatmapCommit, 0, len(commits))
	for i := range commits {
		if commits[i].CommitTimestamp <= 0 {
			continue
		}
		records = append(records, &UserHeatmapCommit{
			UserID:          userID,
			RepoID:          repoID,
			CommitSha1:      commits[i].CommitSha1,
			CommitTimestamp: commits[i].CommitTimestamp,
		})
	}

	if len(records) == 0 {
		return nil
	}

	_, err := db.GetEngine(ctx).Insert(&records)
	return err
}

// DeleteUserHeatmapCommitsByRepo removes all heatmap commit records for a repo.
func DeleteUserHeatmapCommitsByRepo(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Delete(new(UserHeatmapCommit))
	return err
}
