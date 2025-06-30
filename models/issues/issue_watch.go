// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
)

// IssueWatch is connection request for receiving issue notification.
type IssueWatch struct {
	ID          int64              `xorm:"pk autoincr"`
	UserID      int64              `xorm:"UNIQUE(watch) NOT NULL"`
	IssueID     int64              `xorm:"UNIQUE(watch) NOT NULL"`
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
func CreateOrUpdateIssueWatch(ctx context.Context, userID, issueID int64, isWatching bool) error {
	iw, exists, err := GetIssueWatch(ctx, userID, issueID)
	if err != nil {
		return err
	}

	if !exists {
		iw = &IssueWatch{
			UserID:     userID,
			IssueID:    issueID,
			IsWatching: isWatching,
		}

		if _, err := db.GetEngine(ctx).Insert(iw); err != nil {
			return err
		}
	} else {
		iw.IsWatching = isWatching

		if _, err := db.GetEngine(ctx).ID(iw.ID).Cols("is_watching", "updated_unix").Update(iw); err != nil {
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

// CheckIssueWatch check if an user is watching an issue
// it takes participants and repo watch into account
func CheckIssueWatch(ctx context.Context, user *user_model.User, issue *Issue) (bool, error) {
	iw, exist, err := GetIssueWatch(ctx, user.ID, issue.ID)
	if err != nil {
		return false, err
	}
	if exist {
		return iw.IsWatching, nil
	}
	w, err := repo_model.GetWatch(ctx, user.ID, issue.RepoID)
	if err != nil {
		return false, err
	}
	return repo_model.IsWatchMode(w.Mode) || IsUserParticipantsOfIssue(ctx, user, issue), nil
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

// GetIssueWatchers returns watchers/unwatchers of a given issue
func GetIssueWatchers(ctx context.Context, issueID int64, listOptions db.ListOptions) (IssueWatchList, error) {
	sess := db.GetEngine(ctx).
		Where("`issue_watch`.issue_id = ?", issueID).
		And("`issue_watch`.is_watching = ?", true).
		And("`user`.is_active = ?", true).
		And("`user`.prohibit_login = ?", false).
		Join("INNER", "`user`", "`user`.id = `issue_watch`.user_id")

	if listOptions.Page > 0 {
		sess = db.SetSessionPagination(sess, &listOptions)
		watches := make([]*IssueWatch, 0, listOptions.PageSize)
		return watches, sess.Find(&watches)
	}
	watches := make([]*IssueWatch, 0, 8)
	return watches, sess.Find(&watches)
}

// CountIssueWatchers count watchers/unwatchers of a given issue
func CountIssueWatchers(ctx context.Context, issueID int64) (int64, error) {
	return db.GetEngine(ctx).
		Where("`issue_watch`.issue_id = ?", issueID).
		And("`issue_watch`.is_watching = ?", true).
		And("`user`.is_active = ?", true).
		And("`user`.prohibit_login = ?", false).
		Join("INNER", "`user`", "`user`.id = `issue_watch`.user_id").Count(new(IssueWatch))
}

// RemoveIssueWatchersByRepoID remove issue watchers by repoID
func RemoveIssueWatchersByRepoID(ctx context.Context, userID, repoID int64) error {
	_, err := db.GetEngine(ctx).
		Join("INNER", "issue", "`issue`.id = `issue_watch`.issue_id AND `issue`.repo_id = ?", repoID).
		Where("`issue_watch`.user_id = ?", userID).
		Delete(new(IssueWatch))
	return err
}
