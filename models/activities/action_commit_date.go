// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// ActionCommitDate represents a commit's author date for heatmap display
type ActionCommitDate struct {
	ID              int64              `xorm:"pk autoincr"`
	ActionID        int64              `xorm:"INDEX"`
	CommitSha1      string             `xorm:"VARCHAR(64)"`
	CommitTimestamp timeutil.TimeStamp `xorm:"INDEX"`
}

func init() {
	db.RegisterModel(new(ActionCommitDate))
}

// InsertActionCommitDates inserts commit date records for an action
func InsertActionCommitDates(
	ctx context.Context, actionID int64, commits []struct {
		Sha1      string
		Timestamp timeutil.TimeStamp
	},
) error {
	if len(commits) == 0 {
		return nil
	}

	records := make([]*ActionCommitDate, len(commits))
	for i, commit := range commits {
		records[i] = &ActionCommitDate{
			ActionID:        actionID,
			CommitSha1:      commit.Sha1,
			CommitTimestamp: commit.Timestamp,
		}
	}

	_, err := db.GetEngine(ctx).Insert(&records)
	return err
}

// DeleteActionCommitDates removes commit date records for an action
func DeleteActionCommitDates(ctx context.Context, actionID int64) error {
	_, err := db.GetEngine(ctx).Where("action_id = ?", actionID).Delete(new(ActionCommitDate))
	return err
}
