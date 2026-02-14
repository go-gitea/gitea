// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// CommitDateEntry holds a commit SHA and its author timestamp for heatmap display.
// Used to pass commit date data from the notifier to the feed system without persisting it on Action.
type CommitDateEntry struct {
	Sha1      string
	Timestamp timeutil.TimeStamp
}

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
func InsertActionCommitDates(ctx context.Context, actionID int64, commits []CommitDateEntry) error {
	if len(commits) == 0 {
		return nil
	}

	records := make([]*ActionCommitDate, 0, len(commits))
	for _, commit := range commits {
		records = append(records, &ActionCommitDate{
			ActionID:        actionID,
			CommitSha1:      commit.Sha1,
			CommitTimestamp: commit.Timestamp,
		})
	}

	_, err := db.GetEngine(ctx).Insert(&records)
	return err
}

// DeleteActionCommitDates removes commit date records for an action
func DeleteActionCommitDates(ctx context.Context, actionID int64) error {
	_, err := db.GetEngine(ctx).Where("action_id = ?", actionID).Delete(new(ActionCommitDate))
	return err
}
