// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// Follow represents relations of user and his/her followers.
type Follow struct {
	ID          int64              `xorm:"pk autoincr"`
	UserID      int64              `xorm:"UNIQUE(follow)"`
	FollowID    int64              `xorm:"UNIQUE(follow)"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
}

func init() {
	db.RegisterModel(new(Follow))
}

// IsFollowing returns true if user is following followID.
func IsFollowing(userID, followID int64) bool {
	has, _ := db.GetEngine(db.DefaultContext).Get(&Follow{UserID: userID, FollowID: followID})
	return has
}

// FollowUser marks someone be another's follower.
func FollowUser(userID, followID int64) (err error) {
	if userID == followID || IsFollowing(userID, followID) {
		return nil
	}

	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Insert(&Follow{UserID: userID, FollowID: followID}); err != nil {
		return err
	}

	if _, err = sess.Exec("UPDATE `user` SET num_followers = num_followers + 1 WHERE id = ?", followID); err != nil {
		return err
	}

	if _, err = sess.Exec("UPDATE `user` SET num_following = num_following + 1 WHERE id = ?", userID); err != nil {
		return err
	}
	return sess.Commit()
}

// UnfollowUser unmarks someone as another's follower.
func UnfollowUser(userID, followID int64) (err error) {
	if userID == followID || !IsFollowing(userID, followID) {
		return nil
	}

	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Delete(&Follow{UserID: userID, FollowID: followID}); err != nil {
		return err
	}

	if _, err = sess.Exec("UPDATE `user` SET num_followers = num_followers - 1 WHERE id = ?", followID); err != nil {
		return err
	}

	if _, err = sess.Exec("UPDATE `user` SET num_following = num_following - 1 WHERE id = ?", userID); err != nil {
		return err
	}
	return sess.Commit()
}
