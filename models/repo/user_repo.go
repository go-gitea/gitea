// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models/db"
)

// GetStarredRepos returns the repos starred by a particular user
func GetStarredRepos(userID int64, private bool, listOptions db.ListOptions) ([]*Repository, error) {
	sess := db.GetEngine(db.DefaultContext).Where("star.uid=?", userID).
		Join("LEFT", "star", "`repository`.id=`star`.repo_id")
	if !private {
		sess = sess.And("is_private=?", false)
	}

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)

		repos := make([]*Repository, 0, listOptions.PageSize)
		return repos, sess.Find(&repos)
	}

	repos := make([]*Repository, 0, 10)
	return repos, sess.Find(&repos)
}

// GetWatchedRepos returns the repos watched by a particular user
func GetWatchedRepos(userID int64, private bool, listOptions db.ListOptions) ([]*Repository, int64, error) {
	sess := db.GetEngine(db.DefaultContext).Where("watch.user_id=?", userID).
		And("`watch`.mode<>?", WatchModeDont).
		Join("LEFT", "watch", "`repository`.id=`watch`.repo_id")
	if !private {
		sess = sess.And("is_private=?", false)
	}

	if listOptions.Page != 0 {
		sess = db.SetSessionPagination(sess, &listOptions)

		repos := make([]*Repository, 0, listOptions.PageSize)
		total, err := sess.FindAndCount(&repos)
		return repos, total, err
	}

	repos := make([]*Repository, 0, 10)
	total, err := sess.FindAndCount(&repos)
	return repos, total, err
}
