// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	//"fmt"
	//"strconv"

	"code.gitea.io/gitea/models/db"
	//"code.gitea.io/gitea/modules/log"
	//"code.gitea.io/gitea/modules/setting"
	//"code.gitea.io/gitea/modules/timeutil"

	//"xorm.io/builder"
	//"xorm.io/xorm"
)

// GetSubscribed returns subscribed issues and pull requests
func GetSubscriptions(pageSize int, user *User) ([]*Issue, error) {
	sql := "SELECT * FROM issue WHERE (issue.id IN (SELECT issue_watch.issue_id FROM issue_watch WHERE is_watching = ? AND user_id = ?))"+
	"OR ((issue.id IN ((SELECT comment.issue_id FROM comment WHERE poster_id = ?))) AND (NOT issue.id IN (SELECT issue_watch.issue_id FROM issue_watch WHERE user_id = ? AND is_watching = ?)))"+
	"OR ((issue.id IN (SELECT issue.id FROM issue WHERE issue.poster_id = ?)) AND (NOT issue.id IN (SELECT issue_watch.issue_id FROM issue_watch WHERE user_id = ? AND is_watching = ?)))"
	//issues := make([]*Issue, 0, pageSize)
	var issues []*Issue
	return issues, db.GetEngine(db.DefaultContext).SQL(sql, true, user.ID, user.ID, user.ID, false, user.ID, user.ID, false).Find(&issues)
}

// GetSubscriptionsCount counts the subscribed issues/PRs
func GetSubscriptionsCount(user *User) (int64, error) {
	return getSubscriptionsCount(db.GetEngine(db.DefaultContext), user)
}

func getSubscriptionsCount(e db.Engine, user *User) (count int64, err error) {
	sql := "SELECT * FROM issue WHERE (issue.id IN (SELECT issue_watch.issue_id FROM issue_watch WHERE is_watching = ? AND user_id = ?))"+
	"OR ((issue.id IN ((SELECT comment.issue_id FROM comment WHERE poster_id = ?))) AND (NOT issue.id IN (SELECT issue_watch.issue_id FROM issue_watch WHERE user_id = ? AND is_watching = ?)))"+
	"OR ((issue.id IN (SELECT issue.id FROM issue WHERE issue.poster_id = ?)) AND (NOT issue.id IN (SELECT issue_watch.issue_id FROM issue_watch WHERE user_id = ? AND is_watching = ?)))"

	count, err = e.SQL(sql, true, user.ID, user.ID, user.ID, false, user.ID, user.ID, false).
		/*Where("(issue.id IN (SELECT issue_watch.issue_id FROM issue_watch WHERE is_watching = ? AND user_id = ?))"+
		"OR ((issue.id IN ((SELECT comment.issue_id FROM comment WHERE poster_id = ?))) AND (NOT issue.id IN (SELECT issue_watch.issue_id FROM issue_watch WHERE user_id = ? AND is_watching = ?)))"+
		"OR ((issue.id IN (SELECT issue.id FROM issue WHERE issue.poster_id = ?)) AND (NOT issue.id IN (SELECT issue_watch.issue_id FROM issue_watch WHERE user_id = ? AND is_watching = ?)))", 
		true, user.ID, user.ID, user.ID, false, user.ID, user.ID, false).*/
		Count(&Issue{})
	return
}
