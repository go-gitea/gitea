// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
)

// IssueWatchMode specifies what kind of watch the user has on a issue
type IssueWatchMode int8

const (
	// IssueWatchModeNormal watch issue
	IssueWatchModeNormal IssueWatchMode = iota + 1 // 1
	// IssueWatchModeDont explicit don't watch
	IssueWatchModeDont // 2
	// IssueWatchModeAuto watch issue (from AutoWatchOnIssueChanges)
	IssueWatchModeAuto // 3
)

// IssueWatch is connection request for receiving issue notification.
type IssueWatch struct {
	ID          int64              `xorm:"pk autoincr"`
	UserID      int64              `xorm:"UNIQUE(watch) NOT NULL"`
	IssueID     int64              `xorm:"UNIQUE(watch) NOT NULL"`
	Mode        IssueWatchMode     `xorm:"NOT NULL DEFAULT 1"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

// IssueWatchList contains IssueWatch
type IssueWatchList []*IssueWatch

// CreateOrUpdateIssueWatchMode set IssueWatchMode for a user and issue
func CreateOrUpdateIssueWatchMode(userID, issueID int64, mode IssueWatchMode) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := createOrUpdateIssueWatchMode(sess, userID, issueID, mode); err != nil {
		return err
	}
	return sess.Commit()
}

func createOrUpdateIssueWatchMode(e Engine, userID, issueID int64, mode IssueWatchMode) error {
	if _, err := e.Exec(fmt.Sprintf("INSERT INTO issue_watch(user_id,issue_id,mode,created_unix,updated_unix) SELECT %d,%d,%d,%d,%d WHERE NOT EXISTS(SELECT 1 FROM issue_watch WHERE user_id = %d AND issue_id = %d);", userID, issueID, mode, time.Now().Unix(), time.Now().Unix(), userID, issueID)); err != nil {
		return err
	}
	iw, exist, err := getIssueWatch(e, userID, issueID)
	if err != nil && !exist {
		return err
	}
	iw.Mode = mode
	iw.UpdatedUnix = timeutil.TimeStampNow()
	if _, err := e.ID(iw.ID).Cols("updated_unix", "mode").Update(iw); err != nil {
		return err
	}
	return nil
}

//DeleteIssueWatch delete an IssueWatch entry of an user to an given issue if exist
func DeleteIssueWatch(userID, issueID int64) error {
	return deleteIssueWatch(x, userID, issueID)
}

func deleteIssueWatch(e Engine, userID, issueID int64) error {
	iw, exists, err := getIssueWatch(e, userID, issueID)
	if err != nil {
		return err
	}
	if exists {
		_, err = e.ID(iw.ID).Delete(iw)
		return err
	}
	return nil
}

// GetIssueWatch returns all IssueWatch objects from db by user and issue
func GetIssueWatch(userID, issueID int64) (iw *IssueWatch, exists bool, err error) {
	return getIssueWatch(x, userID, issueID)
}

func getIssueWatch(e Engine, userID, issueID int64) (iw *IssueWatch, exists bool, err error) {
	iw = new(IssueWatch)
	exists, err = e.
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Get(iw)
	return
}

// GetIssueWatchersIDs returns IDs of subscribers to a given issue id
// but avoids joining with `user` for performance reasons
// User permissions must be verified elsewhere if required
func GetIssueWatchersIDs(issueID int64, modes ...IssueWatchMode) ([]int64, error) {
	if len(modes) == 0 {
		modes = []IssueWatchMode{IssueWatchModeNormal, IssueWatchModeAuto}
	}
	return getIssueWatchersIDs(x, issueID, modes...)
}
func getIssueWatchersIDs(e Engine, issueID int64, modes ...IssueWatchMode) ([]int64, error) {
	ids := make([]int64, 0, 64)
	if len(modes) == 0 {
		return nil, fmt.Errorf("no IssueWatchMode set")
	}
	return ids, e.Table("issue_watch").
		Where("issue_id=?", issueID).
		In("mode", modes).
		Select("user_id").
		Find(&ids)
}

// GetIssueWatchers returns IssueWatch entry's based on modes of a given issue
func GetIssueWatchers(issueID int64, listOptions ListOptions, modes ...IssueWatchMode) (IssueWatchList, error) {
	if len(modes) == 0 {
		modes = []IssueWatchMode{IssueWatchModeNormal, IssueWatchModeAuto}
	}
	return getIssueWatchers(x, issueID, listOptions, modes...)
}

func getIssueWatchers(e Engine, issueID int64, listOptions ListOptions, modes ...IssueWatchMode) (watches IssueWatchList, err error) {
	if len(modes) == 0 {
		return nil, fmt.Errorf("no IssueWatchMode set")
	}
	sess := e.
		Where("`issue_watch`.issue_id = ?", issueID).
		In("`issue_watch`.mode", modes).
		And("`user`.is_active = ?", true).
		And("`user`.prohibit_login = ?", false).
		Join("INNER", "`user`", "`user`.id = `issue_watch`.user_id")

	if listOptions.Page == 0 {
		sess = listOptions.setSessionPagination(sess)
	}
	err = sess.Find(&watches)
	return
}

func removeIssueWatchersByRepoID(e Engine, userID int64, repoID int64) error {
	_, err := e.
		Join("INNER", "issue", "`issue`.id = `issue_watch`.issue_id AND `issue`.repo_id = ?", repoID).
		Cols("mode", "updated_unix").
		Where("`issue_watch`.user_id = ?", userID).
		Delete(new(IssueWatch))
	return err
}

// IsWatching is true if user iw watching either repo or issue (backwards compatibility)
func (iw IssueWatch) IsWatching() bool {
	issue, err := GetIssueByID(iw.IssueID)
	if err != nil {
		// fail silent since template expect only bool
		log.Warn("IssueWatch.IsWatching: GetIssueByID: ", err)
		return false
	}
	// if RepoWatch is true ...
	if IsWatching(iw.UserID, issue.RepoID) && iw.Mode != IssueWatchModeDont {
		return true
	}

	return iw.Mode == IssueWatchModeNormal || iw.Mode == IssueWatchModeAuto
}

// GetWatchUsers return watching users based on an issue watch list
func (iwl IssueWatchList) GetWatchUsers() (users UserList, err error) {
	return iwl.getWatchUsers(x)
}

func (iwl IssueWatchList) getWatchUsers(e Engine) (users UserList, err error) {
	if len(iwl) == 0 {
		return []*User{}, nil
	}

	var userIDs = make([]int64, 0, len(iwl))
	for _, iw := range iwl {
		userIDs = append(userIDs, iw.UserID)
	}

	err = e.In("id", userIDs).Find(&users)

	return
}
