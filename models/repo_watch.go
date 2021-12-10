// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
)

// RepoWatchMode specifies what kind of watch the user has on a repository
type RepoWatchMode int8

const (
	// RepoWatchModeNone don't watch
	RepoWatchModeNone RepoWatchMode = iota // 0
	// RepoWatchModeNormal watch repository (from other sources)
	RepoWatchModeNormal // 1
	// RepoWatchModeDont explicit don't auto-watch
	RepoWatchModeDont // 2
	// RepoWatchModeAuto watch repository (from AutoWatchOnChanges)
	RepoWatchModeAuto // 3
)

// Watch is connection request for receiving repository notification.
type Watch struct {
	ID          int64              `xorm:"pk autoincr"`
	UserID      int64              `xorm:"UNIQUE(watch)"`
	RepoID      int64              `xorm:"UNIQUE(watch)"`
	Mode        RepoWatchMode      `xorm:"SMALLINT NOT NULL DEFAULT 1"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(Watch))
}

// getWatch gets what kind of subscription a user has on a given repository; returns dummy record if none found
func getWatch(e db.Engine, userID, repoID int64) (Watch, error) {
	watch := Watch{UserID: userID, RepoID: repoID}
	has, err := e.Get(&watch)
	if err != nil {
		return watch, err
	}
	if !has {
		watch.Mode = RepoWatchModeNone
	}
	return watch, nil
}

// Decodes watchability of RepoWatchMode
func isWatchMode(mode RepoWatchMode) bool {
	return mode != RepoWatchModeNone && mode != RepoWatchModeDont
}

// IsWatching checks if user has watched given repository.
func IsWatching(userID, repoID int64) bool {
	watch, err := getWatch(db.GetEngine(db.DefaultContext), userID, repoID)
	return err == nil && isWatchMode(watch.Mode)
}

func watchRepoMode(e db.Engine, watch Watch, mode RepoWatchMode) (err error) {
	if watch.Mode == mode {
		return nil
	}
	if mode == RepoWatchModeAuto && (watch.Mode == RepoWatchModeDont || isWatchMode(watch.Mode)) {
		// Don't auto watch if already watching or deliberately not watching
		return nil
	}

	hadrec := watch.Mode != RepoWatchModeNone
	needsrec := mode != RepoWatchModeNone
	repodiff := 0

	if isWatchMode(mode) && !isWatchMode(watch.Mode) {
		repodiff = 1
	} else if !isWatchMode(mode) && isWatchMode(watch.Mode) {
		repodiff = -1
	}

	watch.Mode = mode

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
func WatchRepoMode(userID, repoID int64, mode RepoWatchMode) (err error) {
	var watch Watch
	if watch, err = getWatch(db.GetEngine(db.DefaultContext), userID, repoID); err != nil {
		return err
	}
	return watchRepoMode(db.GetEngine(db.DefaultContext), watch, mode)
}

func watchRepo(e db.Engine, userID, repoID int64, doWatch bool) (err error) {
	var watch Watch
	if watch, err = getWatch(e, userID, repoID); err != nil {
		return err
	}
	if !doWatch && watch.Mode == RepoWatchModeAuto {
		err = watchRepoMode(e, watch, RepoWatchModeDont)
	} else if !doWatch {
		err = watchRepoMode(e, watch, RepoWatchModeNone)
	} else {
		err = watchRepoMode(e, watch, RepoWatchModeNormal)
	}
	return err
}

// WatchRepo watch or unwatch repository.
func WatchRepo(userID, repoID int64, watch bool) (err error) {
	return watchRepo(db.GetEngine(db.DefaultContext), userID, repoID, watch)
}

func getWatchers(e db.Engine, repoID int64) ([]*Watch, error) {
	watches := make([]*Watch, 0, 10)
	return watches, e.Where("`watch`.repo_id=?", repoID).
		And("`watch`.mode<>?", RepoWatchModeDont).
		And("`user`.is_active=?", true).
		And("`user`.prohibit_login=?", false).
		Join("INNER", "`user`", "`user`.id = `watch`.user_id").
		Find(&watches)
}

// GetWatchers returns all watchers of given repository.
func GetWatchers(repoID int64) ([]*Watch, error) {
	return getWatchers(db.GetEngine(db.DefaultContext), repoID)
}

// GetRepoWatchersIDs returns IDs of watchers for a given repo ID
// but avoids joining with `user` for performance reasons
// User permissions must be verified elsewhere if required
func GetRepoWatchersIDs(repoID int64) ([]int64, error) {
	return getRepoWatchersIDs(db.GetEngine(db.DefaultContext), repoID)
}

func getRepoWatchersIDs(e db.Engine, repoID int64) ([]int64, error) {
	ids := make([]int64, 0, 64)
	return ids, e.Table("watch").
		Where("watch.repo_id=?", repoID).
		And("watch.mode<>?", RepoWatchModeDont).
		Select("user_id").
		Find(&ids)
}

// GetRepoWatchers returns range of users watching given repository.
func GetRepoWatchers(repoID int64, opts db.ListOptions) ([]*user_model.User, error) {
	sess := db.GetEngine(db.DefaultContext).Where("watch.repo_id=?", repoID).
		Join("LEFT", "watch", "`user`.id=`watch`.user_id").
		And("`watch`.mode<>?", RepoWatchModeDont)
	if opts.Page > 0 {
		sess = db.SetSessionPagination(sess, &opts)
		users := make([]*user_model.User, 0, opts.PageSize)

		return users, sess.Find(&users)
	}

	users := make([]*user_model.User, 0, 8)
	return users, sess.Find(&users)
}

func notifyWatchers(ctx context.Context, actions ...*Action) error {
	var watchers []*Watch
	var repo *repo_model.Repository
	var err error
	var permCode []bool
	var permIssue []bool
	var permPR []bool

	e := db.GetEngine(ctx)

	for _, act := range actions {
		repoChanged := repo == nil || repo.ID != act.RepoID

		if repoChanged {
			// Add feeds for user self and all watchers.
			watchers, err = getWatchers(e, act.RepoID)
			if err != nil {
				return fmt.Errorf("get watchers: %v", err)
			}
		}

		// Add feed for actioner.
		act.UserID = act.ActUserID
		if _, err = e.InsertOne(act); err != nil {
			return fmt.Errorf("insert new actioner: %v", err)
		}

		if repoChanged {
			act.loadRepo()
			repo = act.Repo

			// check repo owner exist.
			if err := act.Repo.GetOwner(ctx); err != nil {
				return fmt.Errorf("can't get repo owner: %v", err)
			}
		} else if act.Repo == nil {
			act.Repo = repo
		}

		// Add feed for organization
		if act.Repo.Owner.IsOrganization() && act.ActUserID != act.Repo.Owner.ID {
			act.ID = 0
			act.UserID = act.Repo.Owner.ID
			if _, err = e.InsertOne(act); err != nil {
				return fmt.Errorf("insert new actioner: %v", err)
			}
		}

		if repoChanged {
			permCode = make([]bool, len(watchers))
			permIssue = make([]bool, len(watchers))
			permPR = make([]bool, len(watchers))
			for i, watcher := range watchers {
				user, err := user_model.GetUserByIDEngine(e, watcher.UserID)
				if err != nil {
					permCode[i] = false
					permIssue[i] = false
					permPR[i] = false
					continue
				}
				perm, err := getUserRepoPermission(ctx, repo, user)
				if err != nil {
					permCode[i] = false
					permIssue[i] = false
					permPR[i] = false
					continue
				}
				permCode[i] = perm.CanRead(unit.TypeCode)
				permIssue[i] = perm.CanRead(unit.TypeIssues)
				permPR[i] = perm.CanRead(unit.TypePullRequests)
			}
		}

		for i, watcher := range watchers {
			if act.ActUserID == watcher.UserID {
				continue
			}
			act.ID = 0
			act.UserID = watcher.UserID
			act.Repo.Units = nil

			switch act.OpType {
			case ActionCommitRepo, ActionPushTag, ActionDeleteTag, ActionPublishRelease, ActionDeleteBranch:
				if !permCode[i] {
					continue
				}
			case ActionCreateIssue, ActionCommentIssue, ActionCloseIssue, ActionReopenIssue:
				if !permIssue[i] {
					continue
				}
			case ActionCreatePullRequest, ActionCommentPull, ActionMergePullRequest, ActionClosePullRequest, ActionReopenPullRequest:
				if !permPR[i] {
					continue
				}
			}

			if _, err = e.InsertOne(act); err != nil {
				return fmt.Errorf("insert new action: %v", err)
			}
		}
	}
	return nil
}

// NotifyWatchers creates batch of actions for every watcher.
func NotifyWatchers(actions ...*Action) error {
	return notifyWatchers(db.DefaultContext, actions...)
}

// NotifyWatchersActions creates batch of actions for every watcher.
func NotifyWatchersActions(acts []*Action) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	for _, act := range acts {
		if err := notifyWatchers(ctx, act); err != nil {
			return err
		}
	}
	return committer.Commit()
}

func watchIfAuto(e db.Engine, userID, repoID int64, isWrite bool) error {
	if !isWrite || !setting.Service.AutoWatchOnChanges {
		return nil
	}
	watch, err := getWatch(e, userID, repoID)
	if err != nil {
		return err
	}
	if watch.Mode != RepoWatchModeNone {
		return nil
	}
	return watchRepoMode(e, watch, RepoWatchModeAuto)
}

// WatchIfAuto subscribes to repo if AutoWatchOnChanges is set
func WatchIfAuto(userID, repoID int64, isWrite bool) error {
	return watchIfAuto(db.GetEngine(db.DefaultContext), userID, repoID, isWrite)
}
