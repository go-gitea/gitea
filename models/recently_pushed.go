// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package models

import (
	"context"
	"strings"
	"time"

	"code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/user"

	"xorm.io/builder"
)

type RecentlyPushedBranches struct {
	Repo     *repo.Repository
	BaseRepo *repo.Repository
	RefName  string
	Time     time.Time
}

// GetRecentlyPushedBranches returns all actions where a user recently pushed but no PRs are created yet.
func GetRecentlyPushedBranches(ctx context.Context, u *user.User) (recentlyPushedBranches []*RecentlyPushedBranches, err error) {
	limit := time.Now().Add(-24 * time.Hour).Unix()

	actions := []*activities.Action{}
	// We're fetching the last three commits activity actions within the limit...
	err = db.GetEngine(ctx).
		Select("action.ref_name, action.repo_id, action.created_unix").
		Join("LEFT", "pull_request", "pull_request.head_branch = replace(action.ref_name, 'refs/heads/', '')").
		Join("LEFT", "issue", "pull_request.issue_id = issue.id").
		Join("LEFT", "repository", "action.repo_id = repository.id").
		Where(builder.And(
			builder.Eq{"action.op_type": activities.ActionCommitRepo},
			// ...done by the current user
			builder.Eq{"action.act_user_id": u.ID},
			// ...which have been pushed to a fork or a branch different from the default branch
			builder.Or(
				builder.Expr("repository.default_branch != replace(action.ref_name, 'refs/heads/', '')"),
				builder.Eq{"repository.is_fork": true},
			),
			// ...and don't have an open or closed PR corresponding to that branch.
			builder.Or(
				builder.IsNull{"pull_request.id"},
				builder.And(
					builder.Eq{"pull_request.has_merged": false},
					builder.Eq{"issue.is_closed": true},
					builder.Expr("action.created_unix > issue.closed_unix"),
				),
			),
			builder.Gte{"action.created_unix": limit},
		)).
		Limit(3).
		GroupBy("action.ref_name, action.repo_id, action.created_unix").
		Desc("action.created_unix").
		Find(&actions)
	if err != nil {
		return nil, err
	}

	repoIDs := []int64{}
	for _, a := range actions {
		repoIDs = append(repoIDs, a.RepoID)
	}

	// Because we need the repo name and url, we need to fetch all repos from recent pushes
	// and, if they are forked, the parent repo as well.
	repos := make(map[int64]*repo.Repository, len(repoIDs))
	err = db.GetEngine(ctx).
		Where(builder.Or(
			builder.In("repository.id", repoIDs),
			builder.In("repository.id",
				builder.Select("repository.fork_id").
					From("repository").
					Where(builder.In("repository.id", repoIDs)),
			),
		)).
		Find(&repos)
	if err != nil {
		return nil, err
	}

	owners := make(map[int64]*user.User)
	err = db.GetEngine(ctx).
		Where(builder.Or(
			builder.In("repository.id", repoIDs),
			builder.In("repository.id",
				builder.Select("repository.fork_id").
					From("repository").
					Where(builder.In("repository.id", repoIDs)),
			),
		)).
		Join("LEFT", "repository", "repository.owner_id = user.id").
		Find(&owners)
	if err != nil {
		return nil, err
	}

	recentlyPushedBranches = []*RecentlyPushedBranches{}
	for _, a := range actions {
		pushed := &RecentlyPushedBranches{
			Repo:     repos[a.RepoID],
			BaseRepo: repos[a.RepoID],
			RefName:  strings.Replace(a.RefName, "refs/heads/", "", 1),
			Time:     a.GetCreate(),
		}

		if pushed.Repo.IsFork {
			pushed.BaseRepo = repos[pushed.Repo.ForkID]
			pushed.BaseRepo.Owner = owners[pushed.BaseRepo.OwnerID]
		}

		pushed.Repo.Owner = owners[pushed.Repo.OwnerID]

		recentlyPushedBranches = append(recentlyPushedBranches, pushed)
	}

	return recentlyPushedBranches, nil
}
