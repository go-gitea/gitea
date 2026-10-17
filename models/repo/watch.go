// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
)

// WatchMode specifies what kind of watch the user has on a repository
type WatchMode int8

const (
	WatchModeNone WatchMode = iota // 0 watch nothing unless mentioned

	WatchModeNormal // 1 proactively watching (all or custom)
	WatchModeDont   // 2 ignore the repo
	WatchModeAuto   // 3 automatically watching (from AutoWatchOnChanges)
)

// WatchType is the `watch` column gating one kind of notification
type WatchType string

const (
	WatchPullRequests WatchType = "pull_requests"
	WatchIssues       WatchType = "issues"
	WatchReleases     WatchType = "releases"
)

// Watch is connection request for receiving repository notification.
type Watch struct {
	ID          int64              `xorm:"pk autoincr"`
	UserID      int64              `xorm:"UNIQUE(watch)"`
	RepoID      int64              `xorm:"UNIQUE(watch)"`
	Mode        WatchMode          `xorm:"SMALLINT NOT NULL DEFAULT 1"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	IncludePullRequests bool `xorm:"NOT NULL DEFAULT true"`
	IncludeIssues       bool `xorm:"NOT NULL DEFAULT true"`
	IncludeReleases     bool `xorm:"NOT NULL DEFAULT true"`
}

func init() {
	db.RegisterModel(new(Watch))
}

// GetWatch gets what kind of subscription a user has on a given repository; returns dummy record if none found
func GetWatch(ctx context.Context, userID, repoID int64) (*Watch, error) {
	watch, has, err := db.Get[Watch](ctx, builder.Eq{"user_id": userID, "repo_id": repoID})
	if err != nil {
		return watch, err
	}
	if watch == nil { // the dummy record must mirror the column defaults
		watch = &Watch{UserID: userID, RepoID: repoID, IncludePullRequests: true, IncludeIssues: true, IncludeReleases: true}
	}
	if !has {
		watch.Mode = WatchModeNone
	}
	return watch, nil
}

// IsIgnoring reports whether the user muted the repository entirely
func (w *Watch) IsIgnoring() bool {
	return w.Mode == WatchModeDont
}

// IsWatching reports whether the watch counts the user as a watcher of the repository
func (w *Watch) IsWatching() bool {
	return IsWatchModeWatching(w.Mode)
}

// IsWatchingAll reports whether every event is enabled, which is the "all activity" mode
func (w *Watch) IsWatchingAll() bool {
	return w.IncludePullRequests && w.IncludeIssues && w.IncludeReleases
}

func (w *Watch) IsWatchingAny() bool {
	return w.IncludePullRequests || w.IncludeIssues || w.IncludeReleases
}

// SelectedMode returns the mode the user picked in the watch menu
func (w *Watch) SelectedMode() string {
	switch {
	case w.IsIgnoring():
		return "ignore"
	case !IsWatchModeWatching(w.Mode), !w.IsWatchingAny():
		return "participate" // also the default while there is no watch row
	case w.IsWatchingAll():
		return "all"
	}
	return "custom"
}

// IsWatchModeWatching Decodes watchability of WatchMode
func IsWatchModeWatching(mode WatchMode) bool {
	return mode != WatchModeNone && mode != WatchModeDont
}

// IsWatchingRepo checks if user has watched given repository.
func IsWatchingRepo(ctx context.Context, userID, repoID int64) bool {
	watch, err := GetWatch(ctx, userID, repoID)
	return err == nil && IsWatchModeWatching(watch.Mode)
}

func watchRepoByMode(ctx context.Context, watch *Watch, mode WatchMode) (err error) {
	if watch.Mode == mode {
		return nil
	}
	if mode == WatchModeAuto && (watch.Mode == WatchModeDont || IsWatchModeWatching(watch.Mode)) {
		// Don't auto watch if already watching or deliberately not watching
		return nil
	}

	hadWatchModeSet := watch.Mode != WatchModeNone
	needSetWatchMode := mode != WatchModeNone
	repoWatchDelta := 0

	if IsWatchModeWatching(mode) && !IsWatchModeWatching(watch.Mode) {
		repoWatchDelta = 1
	} else if !IsWatchModeWatching(mode) && IsWatchModeWatching(watch.Mode) {
		repoWatchDelta = -1
	}

	if repoWatchDelta == 1 { // starting to watch resets the options, otherwise a custom selection survives
		watch.IncludePullRequests, watch.IncludeIssues, watch.IncludeReleases = true, true, true
	}
	watch.Mode = mode

	if !hadWatchModeSet && needSetWatchMode {
		if err = db.Insert(ctx, watch); err != nil {
			return err
		}
	} else if needSetWatchMode {
		if _, err := db.GetEngine(ctx).ID(watch.ID).AllCols().Update(watch); err != nil {
			return err
		}
	} else if _, err = db.DeleteByID[Watch](ctx, watch.ID); err != nil {
		return err
	}
	if repoWatchDelta != 0 {
		_, err = db.GetEngine(ctx).Exec("UPDATE `repository` SET num_watches = num_watches + ? WHERE id = ?", repoWatchDelta, watch.RepoID)
	}
	return err
}

// WatchRepoAuto watch or unwatch repository.
func WatchRepoAuto(ctx context.Context, doer *user_model.User, repo *Repository, doWatch bool) error {
	watch, err := GetWatch(ctx, doer.ID, repo.ID)
	if err != nil {
		return err
	}
	if !doWatch && watch.Mode == WatchModeAuto {
		return watchRepoByMode(ctx, watch, WatchModeDont)
	} else if !doWatch {
		return watchRepoByMode(ctx, watch, WatchModeNone)
	}

	if user_model.IsUserBlockedBy(ctx, doer, repo.OwnerID) {
		return user_model.ErrBlockedUser
	}

	return watchRepoByMode(ctx, watch, WatchModeNormal)
}

type WatchOptions struct {
	Mode WatchMode

	WatchPullRequests bool
	WatchIssues       bool
	WatchReleases     bool
}

// WatchRepoWithOptions starts watching the repository and subscribes to the given events
func WatchRepoWithOptions(ctx context.Context, doer *user_model.User, repo *Repository, opts WatchOptions) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		watch, err := GetWatch(ctx, doer.ID, repo.ID)
		if err != nil {
			return err
		}
		err = watchRepoByMode(ctx, watch, opts.Mode)
		if err != nil {
			return err
		}
		if opts.Mode == WatchModeNormal {
			_, err = db.GetEngine(ctx).Where("user_id=? AND repo_id=?", doer.ID, repo.ID).
				Cols("include_pull_requests", "include_issues", "include_releases").
				Update(&Watch{IncludePullRequests: opts.WatchPullRequests, IncludeIssues: opts.WatchIssues, IncludeReleases: opts.WatchReleases})
		}
		return err
	})
}

// GetUserWatches returns the watches of one user, keyed by repository ID
func GetUserWatches(ctx context.Context, userID int64, repoIDs []int64) (map[int64]*Watch, error) {
	if len(repoIDs) == 0 {
		return map[int64]*Watch{}, nil
	}
	watches := make([]*Watch, 0, len(repoIDs))
	if err := db.GetEngine(ctx).Where("user_id=?", userID).
		In("repo_id", repoIDs).
		And("mode<>?", WatchModeDont).
		Find(&watches); err != nil {
		return nil, err
	}
	watchesByRepo := make(map[int64]*Watch, len(watches))
	for _, watch := range watches {
		watchesByRepo[watch.RepoID] = watch
	}
	return watchesByRepo, nil
}

// GetWatchers returns all watchers of given repository, skipping those subscribed to no event.
func GetWatchers(ctx context.Context, repoID int64) ([]*Watch, error) {
	watches := make([]*Watch, 0, 10)
	return watches, db.GetEngine(ctx).Where("`watch`.repo_id=?", repoID).
		And("`watch`.mode<>?", WatchModeDont).
		And(builder.Or(
			builder.Eq{"`watch`.include_pull_requests": true},
			builder.Eq{"`watch`.include_issues": true},
			builder.Eq{"`watch`.include_releases": true},
		)).
		And("`user`.is_active=?", true).
		And("`user`.prohibit_login=?", false).
		Join("INNER", "`user`", "`user`.id = `watch`.user_id").
		Find(&watches)
}

// GetRepoIgnorersIDs returns IDs of users who muted the given repo ID
func GetRepoIgnorersIDs(ctx context.Context, repoID int64) ([]int64, error) {
	ids := make([]int64, 0, 8)
	return ids, db.GetEngine(ctx).Table("watch").
		Where("repo_id=?", repoID).
		And("mode=?", WatchModeDont).
		Select("user_id").
		Find(&ids)
}

// GetRepoWatchersIDs returns IDs of watchers for a given repo ID that opted into watchType
// but avoids joining with `user` for performance reasons
// User permissions must be verified elsewhere if required
func GetRepoWatchersIDs(ctx context.Context, repoID int64, watchType WatchType) ([]int64, error) {
	ids := make([]int64, 0, 64)
	var watchColName string
	switch watchType {
	case WatchPullRequests:
		watchColName = "include_pull_requests"
	case WatchIssues:
		watchColName = "include_issues"
	case WatchReleases:
		watchColName = "include_releases"
	default:
		panic("invalid WatchType")
	}
	return ids, db.GetEngine(ctx).Table("watch").
		Where("watch.repo_id=?", repoID).
		And("watch.mode<>?", WatchModeDont).
		And(builder.Eq{watchColName: true}).
		Select("user_id").
		Find(&ids)
}

// GetRepoWatchers returns range of users watching given repository.
func GetRepoWatchers(ctx context.Context, repoID int64, opts db.ListOptions) ([]*user_model.User, error) {
	sess := db.GetEngine(ctx).Where("watch.repo_id=?", repoID).
		Join("LEFT", "watch", "`user`.id=`watch`.user_id").
		And("`watch`.mode<>?", WatchModeDont)
	if opts.Page > 0 {
		db.SetSessionPagination(sess, &opts)
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
	return watchRepoByMode(ctx, watch, WatchModeAuto)
}

// ClearRepoWatches clears all watches for a repository and from the user that watched it.
// Used when a repository is set to private.
func ClearRepoWatches(ctx context.Context, repoID int64) error {
	if _, err := db.Exec(ctx, "UPDATE `repository` SET num_watches = 0 WHERE id = ?", repoID); err != nil {
		return err
	}

	return db.DeleteBeans(ctx, Watch{RepoID: repoID})
}
