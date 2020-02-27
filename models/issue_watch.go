// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
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

// IssueWatchList contains IssueWatch
type IssueWatchList []*IssueWatch

// CreateOrUpdateIssueWatch set watching for a user and issue
func CreateOrUpdateIssueWatch(userID, issueID int64, isWatching bool) error {
	iw, exists, err := getIssueWatch(x, userID, issueID)
	if err != nil {
		return err
	}

	if !exists {
		iw = &IssueWatch{
			UserID:     userID,
			IssueID:    issueID,
			IsWatching: isWatching,
		}

		if _, err := x.Insert(iw); err != nil {
			return err
		}
	} else {
		iw.IsWatching = isWatching

		if _, err := x.ID(iw.ID).Cols("is_watching", "updated_unix").Update(iw); err != nil {
			return err
		}
	}
	return nil
}

// GetIssueWatch returns all IssueWatch objects from db by user and issue
// the current Web-UI need iw object for watchers AND explicit non-watchers
func GetIssueWatch(userID, issueID int64) (iw *IssueWatch, exists bool, err error) {
	return getIssueWatch(x, userID, issueID)
}

// Return watcher AND explicit non-watcher if entry in db exist
func getIssueWatch(e Engine, userID, issueID int64) (iw *IssueWatch, exists bool, err error) {
	iw = new(IssueWatch)
	exists, err = e.
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Get(iw)
	return
}

// GetIssueWatchersIDs returns IDs of subscribers or explicit unsubscribers to a given issue id
// but avoids joining with `user` for performance reasons
// User permissions must be verified elsewhere if required
func GetIssueWatchersIDs(issueID int64, watching bool) ([]int64, error) {
	return getIssueWatchersIDs(x, issueID, watching)
}

func getIssueWatchersIDs(e Engine, issueID int64, watching bool) ([]int64, error) {
	ids := make([]int64, 0, 64)
	return ids, e.Table("issue_watch").
		Where("issue_id=?", issueID).
		And("is_watching = ?", watching).
		Select("user_id").
		Find(&ids)
}

// GetIssueWatchers returns watchers/unwatchers of a given issue
func GetIssueWatchers(issueID int64) (IssueWatchList, error) {
	return getIssueWatchers(x, issueID)
}

func getIssueWatchers(e Engine, issueID int64) (watches IssueWatchList, err error) {
	err = e.
		Where("`issue_watch`.issue_id = ?", issueID).
		And("`issue_watch`.is_watching = ?", true).
		And("`user`.is_active = ?", true).
		And("`user`.prohibit_login = ?", false).
		Join("INNER", "`user`", "`user`.id = `issue_watch`.user_id").
		Find(&watches)
	return
}

func removeIssueWatchersByRepoID(e Engine, userID int64, repoID int64) error {
	iw := &IssueWatch{
		IsWatching: false,
	}
	_, err := e.
		Join("INNER", "issue", "`issue`.id = `issue_watch`.issue_id AND `issue`.repo_id = ?", repoID).
		Cols("is_watching", "updated_unix").
		Where("`issue_watch`.user_id = ?", userID).
		Update(iw)
	return err
}

// LoadWatchUsers return watching users
func (iwl IssueWatchList) LoadWatchUsers() (users UserList, err error) {
	return iwl.loadWatchUsers(x)
}

func (iwl IssueWatchList) loadWatchUsers(e Engine) (users UserList, err error) {
	if len(iwl) == 0 {
		return []*User{}, nil
	}

	var userIDs = make([]int64, 0, len(iwl))
	for _, iw := range iwl {
		if iw.IsWatching {
			userIDs = append(userIDs, iw.UserID)
		}
	}

	if len(userIDs) == 0 {
		return []*User{}, nil
	}

	err = e.In("id", userIDs).Find(&users)

	return
}
