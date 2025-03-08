// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package feed

import (
	"context"
	"fmt"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
)

// GetFeeds returns actions according to the provided options
func GetFeeds(ctx context.Context, opts activities_model.GetFeedsOptions) (activities_model.ActionList, int64, error) {
	return activities_model.GetFeeds(ctx, opts)
}

// NotifyWatchers creates batch of actions for every watcher.
// It could insert duplicate actions for a repository action, like this:
// * Original action: UserID=1 (the real actor), ActUserID=1
// * Organization action: UserID=100 (the repo's org), ActUserID=1
// * Watcher action: UserID=20 (a user who is watching a repo), ActUserID=1
func notifyWatchers(ctx context.Context, act *activities_model.Action) error {
	var watchers []*repo_model.Watch
	var repo *repo_model.Repository
	var err error
	var permCode []bool
	var permIssue []bool
	var permPR []bool

	repoChanged := repo == nil || repo.ID != act.RepoID

	if repoChanged {
		// Add feeds for user self and all watchers.
		watchers, err = repo_model.GetWatchers(ctx, act.RepoID)
		if err != nil {
			return fmt.Errorf("get watchers: %w", err)
		}
	}

	// Add feed for actioner.
	act.UserID = act.ActUserID
	if err = db.Insert(ctx, act); err != nil {
		return fmt.Errorf("insert new actioner: %w", err)
	}

	if repoChanged {
		act.LoadRepo(ctx)
		repo = act.Repo

		// check repo owner exist.
		if err := act.Repo.LoadOwner(ctx); err != nil {
			return fmt.Errorf("can't get repo owner: %w", err)
		}
	} else if act.Repo == nil {
		act.Repo = repo
	}

	// Add feed for organization
	if act.Repo.Owner.IsOrganization() && act.ActUserID != act.Repo.Owner.ID {
		act.ID = 0
		act.UserID = act.Repo.Owner.ID
		if err = db.Insert(ctx, act); err != nil {
			return fmt.Errorf("insert new actioner: %w", err)
		}
	}

	if repoChanged {
		permCode = make([]bool, len(watchers))
		permIssue = make([]bool, len(watchers))
		permPR = make([]bool, len(watchers))
		for i, watcher := range watchers {
			user, err := user_model.GetUserByID(ctx, watcher.UserID)
			if err != nil {
				permCode[i] = false
				permIssue[i] = false
				permPR[i] = false
				continue
			}
			perm, err := access_model.GetUserRepoPermission(ctx, repo, user)
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
		case activities_model.ActionCommitRepo, activities_model.ActionPushTag, activities_model.ActionDeleteTag, activities_model.ActionPublishRelease, activities_model.ActionDeleteBranch:
			if !permCode[i] {
				continue
			}
		case activities_model.ActionCreateIssue, activities_model.ActionCommentIssue, activities_model.ActionCloseIssue, activities_model.ActionReopenIssue:
			if !permIssue[i] {
				continue
			}
		case activities_model.ActionCreatePullRequest, activities_model.ActionCommentPull, activities_model.ActionMergePullRequest, activities_model.ActionClosePullRequest, activities_model.ActionReopenPullRequest, activities_model.ActionAutoMergePullRequest:
			if !permPR[i] {
				continue
			}
		}

		if err = db.Insert(ctx, act); err != nil {
			return fmt.Errorf("insert new action: %w", err)
		}
	}

	return nil
}

// NotifyWatchersActions creates batch of actions for every watcher.
func NotifyWatchers(ctx context.Context, acts ...*activities_model.Action) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		for _, act := range acts {
			if err := notifyWatchers(ctx, act); err != nil {
				return err
			}
		}
		return nil
	})
}
