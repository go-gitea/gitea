// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

// Follow represents relations of user and their followers.
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
func IsFollowing(ctx context.Context, userID, followID int64) bool {
	has, _ := db.GetEngine(ctx).Get(&Follow{UserID: userID, FollowID: followID})
	return has
}

// FollowUser marks someone be another's follower.
func FollowUser(ctx context.Context, userID, followID int64) (err error) {
	if userID == followID || IsFollowing(ctx, userID, followID) {
		return nil
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = db.Insert(ctx, &Follow{UserID: userID, FollowID: followID}); err != nil {
		return err
	}

	if _, err = db.Exec(ctx, "UPDATE `user` SET num_followers = num_followers + 1 WHERE id = ?", followID); err != nil {
		return err
	}

	if _, err = db.Exec(ctx, "UPDATE `user` SET num_following = num_following + 1 WHERE id = ?", userID); err != nil {
		return err
	}
	return committer.Commit()
}

// UnfollowUser unmarks someone as another's follower.
func UnfollowUser(ctx context.Context, userID, followID int64) (err error) {
	if userID == followID || !IsFollowing(ctx, userID, followID) {
		return nil
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if _, err = db.DeleteByBean(ctx, &Follow{UserID: userID, FollowID: followID}); err != nil {
		return err
	}

	if _, err = db.Exec(ctx, "UPDATE `user` SET num_followers = num_followers - 1 WHERE id = ?", followID); err != nil {
		return err
	}

	if _, err = db.Exec(ctx, "UPDATE `user` SET num_following = num_following - 1 WHERE id = ?", userID); err != nil {
		return err
	}
	return committer.Commit()
}
