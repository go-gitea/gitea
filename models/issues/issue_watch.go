// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// IssueWatch is connection request for receiving issue notification.
type IssueWatch struct {
	ID          int64              `xorm:"pk autoincr"`
	IssueID     int64              `xorm:"UNIQUE(watch) NOT NULL"`
	UserID      int64              `xorm:"UNIQUE(watch) NOT NULL"`
	IsWatching  bool               `xorm:"NOT NULL"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

func init() {
	db.RegisterModel(new(IssueWatch))
}

// IssueWatchList contains IssueWatch
type IssueWatchList []*IssueWatch

// CreateOrUpdateIssueWatch set watching for a user and issue
func CreateOrUpdateIssueWatch(userID, issueID int64, isWatching bool) error {
	iw, exists, err := GetIssueWatch(db.DefaultContext, userID, issueID)
	if err != nil {
		return err
	}

	if !exists {
		iw = &IssueWatch{
			UserID:     userID,
			IssueID:    issueID,
			IsWatching: isWatching,
		}

		if _, err := db.GetEngine(db.DefaultContext).Insert(iw); err != nil {
			return err
		}
	} else {
		iw.IsWatching = isWatching

		if _, err := db.GetEngine(db.DefaultContext).ID(iw.ID).Cols("is_watching", "updated_unix").Update(iw); err != nil {
			return err
		}
	}
	return nil
}

// GetIssueWatch returns all IssueWatch objects from db by user and issue
// the current Web-UI need iw object for watchers AND explicit non-watchers
func GetIssueWatch(ctx context.Context, userID, issueID int64) (iw *IssueWatch, exists bool, err error) {
	iw = new(IssueWatch)
	exists, err = db.GetEngine(ctx).
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Get(iw)
	return iw, exists, err
}

// GetIssueWatchersIDs returns IDs of subscribers or explicit unsubscribers to a given issue id
// but avoids joining with `user` for performance reasons
// User permissions must be verified elsewhere if required
func GetIssueWatchersIDs(ctx context.Context, issueID int64, watching bool) ([]int64, error) {
	ids := make([]int64, 0, 64)
	return ids, db.GetEngine(ctx).Table("issue_watch").
		Where("issue_id=?", issueID).
		And("is_watching = ?", watching).
		Select("user_id").
		Find(&ids)
}

// RemoveIssueWatchersByRepoID remove issue watchers by repoID
func RemoveIssueWatchersByRepoID(ctx context.Context, userID, repoID int64) error {
	_, err := db.GetEngine(ctx).
		Join("INNER", "issue", "`issue`.id = `issue_watch`.issue_id AND `issue`.repo_id = ?", repoID).
		Where("`issue_watch`.user_id = ?", userID).
		Delete(new(IssueWatch))
	return err
}
