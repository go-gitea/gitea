// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
)

// WatchMode specifies what kind of watch the user has on a repository
type WatchMode int8

const (
	// WatchModeNone don't watch
	WatchModeNone WatchMode = iota // 0
	// WatchModeNormal watch repository (from other sources)
	WatchModeNormal // 1
	// WatchModeDont explicit don't auto-watch
	WatchModeDont // 2
	// WatchModeAuto watch repository (from AutoWatchOnChanges)
	WatchModeAuto // 3
)

// Watch is connection request for receiving repository notification.
type Watch struct {
	ID          int64              `xorm:"pk autoincr"`
	UserID      int64              `xorm:"UNIQUE(watch)"`
	RepoID      int64              `xorm:"UNIQUE(watch)"`
	Mode        WatchMode          `xorm:"SMALLINT NOT NULL DEFAULT 1"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(Watch))
}

// GetWatch gets what kind of subscription a user has on a given repository; returns dummy record if none found
func GetWatch(ctx context.Context, userID, repoID int64) (Watch, error) {
	watch := Watch{UserID: userID, RepoID: repoID}
	has, err := db.GetEngine(ctx).Get(&watch)
	if err != nil {
		return watch, err
	}
	if !has {
		watch.Mode = WatchModeNone
	}
	return watch, nil
}

// IsWatchMode Decodes watchability of WatchMode
func IsWatchMode(mode WatchMode) bool {
	return mode != WatchModeNone && mode != WatchModeDont
}

// IsWatching checks if user has watched given repository.
func IsWatching(ctx context.Context, userID, repoID int64) bool {
	watch, err := GetWatch(ctx, userID, repoID)
	return err == nil && IsWatchMode(watch.Mode)
}

func watchRepoMode(ctx context.Context, watch Watch, mode WatchMode) (err error) {
	if watch.Mode == mode {
		return nil
	}
	if mode == WatchModeAuto && (watch.Mode == WatchModeDont || IsWatchMode(watch.Mode)) {
		// Don't auto watch if already watching or deliberately not watching
		return nil
	}

	hadrec := watch.Mode != WatchModeNone
	needsrec := mode != WatchModeNone
	repodiff := 0

	if IsWatchMode(mode) && !IsWatchMode(watch.Mode) {
		repodiff = 1
	} else if !IsWatchMode(mode) && IsWatchMode(watch.Mode) {
		repodiff = -1
	}

	watch.Mode = mode

	e := db.GetEngine(ctx)

	if !hadrec && needsrec {
		watch.Mode = mode
		if _, err = e.Insert(watch); err != nil {
			return err
		}
	} else if needsrec {
		watch.Mode = mode
		if _, err := e.ID(watch.ID).AllCols().Update(watch); err != nil {
			return err
		}
	} else if _, err = e.Delete(Watch{ID: watch.ID}); err != nil {
		return err
	}
	if repodiff != 0 {
		_, err = e.Exec("UPDATE `repository` SET num_watches = num_watches + ? WHERE id = ?", repodiff, watch.RepoID)
	}
	return err
}

// WatchRepoMode watch repository in specific mode.
func WatchRepoMode(ctx context.Context, userID, repoID int64, mode WatchMode) (err error) {
	var watch Watch
	if watch, err = GetWatch(ctx, userID, repoID); err != nil {
		return err
	}
	return watchRepoMode(ctx, watch, mode)
}

// WatchRepo watch or unwatch repository.
func WatchRepo(ctx context.Context, userID, repoID int64, doWatch bool) (err error) {
	var watch Watch
	if watch, err = GetWatch(ctx, userID, repoID); err != nil {
		return err
	}
	if !doWatch && watch.Mode == WatchModeAuto {
		err = watchRepoMode(ctx, watch, WatchModeDont)
	} else if !doWatch {
		err = watchRepoMode(ctx, watch, WatchModeNone)
	} else {
		err = watchRepoMode(ctx, watch, WatchModeNormal)
	}
	return err
}

// GetWatchers returns all watchers of given repository.
func GetWatchers(ctx context.Context, repoID int64) ([]*Watch, error) {
	watches := make([]*Watch, 0, 10)
	return watches, db.GetEngine(ctx).Where("`watch`.repo_id=?", repoID).
		And("`watch`.mode<>?", WatchModeDont).
		And("`user`.is_active=?", true).
		And("`user`.prohibit_login=?", false).
		Join("INNER", "`user`", "`user`.id = `watch`.user_id").
		Find(&watches)
}

// GetRepoWatchersIDs returns IDs of watchers for a given repo ID
// but avoids joining with `user` for performance reasons
// User permissions must be verified elsewhere if required
func GetRepoWatchersIDs(ctx context.Context, repoID int64) ([]int64, error) {
	ids := make([]int64, 0, 64)
	return ids, db.GetEngine(ctx).Table("watch").
		Where("watch.repo_id=?", repoID).
		And("watch.mode<>?", WatchModeDont).
		Select("user_id").
		Find(&ids)
}

// GetRepoWatchers returns range of users watching given repository.
func GetRepoWatchers(ctx context.Context, repoID int64, opts db.ListOptions) ([]*user_model.User, error) {
	sess := db.GetEngine(ctx).Where("watch.repo_id=?", repoID).
		Join("LEFT", "watch", "`user`.id=`watch`.user_id").
		And("`watch`.mode<>?", WatchModeDont)
	if opts.Page > 0 {
		sess = db.SetSessionPagination(sess, &opts)
		users := make([]*user_model.User, 0, opts.PageSize)

		return users, sess.Find(&users)
	}

	users := make([]*user_model.User, 0, 8)
	return users, sess.Find(&users)
}

// WatchIfAuto subscribes to repo if AutoWatchOnChanges is set
func WatchIfAuto(ctx context.Context, userID, repoID int64, isWrite bool) error {
	if !isWrite || !setting.Service.AutoWatchOnChanges {
		return nil
	}
	watch, err := GetWatch(ctx, userID, repoID)
	if err != nil {
		return err
	}
	if watch.Mode != WatchModeNone {
		return nil
	}
	return watchRepoMode(ctx, watch, WatchModeAuto)
}
