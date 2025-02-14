// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/optional"

	"xorm.io/builder"
)

type BranchList []*Branch

func (branches BranchList) LoadDeletedBy(ctx context.Context) error {
	ids := container.FilterSlice(branches, func(branch *Branch) (int64, bool) {
		return branch.DeletedByID, branch.IsDeleted
	})

	usersMap := make(map[int64]*user_model.User, len(ids))
	if err := db.GetEngine(ctx).In("id", ids).Find(&usersMap); err != nil {
		return err
	}
	for _, branch := range branches {
		if !branch.IsDeleted {
			continue
		}
		branch.DeletedBy = usersMap[branch.DeletedByID]
		if branch.DeletedBy == nil {
			branch.DeletedBy = user_model.NewGhostUser()
		}
	}
	return nil
}

func (branches BranchList) LoadPusher(ctx context.Context) error {
	ids := container.FilterSlice(branches, func(branch *Branch) (int64, bool) {
		// pusher_id maybe zero because some branches are sync by backend with no pusher
		return branch.PusherID, branch.PusherID > 0
	})

	usersMap := make(map[int64]*user_model.User, len(ids))
	if err := db.GetEngine(ctx).In("id", ids).Find(&usersMap); err != nil {
		return err
	}
	for _, branch := range branches {
		if branch.PusherID <= 0 {
			continue
		}
		branch.Pusher = usersMap[branch.PusherID]
		if branch.Pusher == nil {
			branch.Pusher = user_model.NewGhostUser()
		}
	}
	return nil
}

func (branches BranchList) LoadRepo(ctx context.Context) error {
	ids := container.FilterSlice(branches, func(branch *Branch) (int64, bool) {
		return branch.RepoID, branch.RepoID > 0 && branch.Repo == nil
	})

	reposMap := make(map[int64]*repo_model.Repository, len(ids))
	if err := db.GetEngine(ctx).In("id", ids).Find(&reposMap); err != nil {
		return err
	}
	for _, branch := range branches {
		if branch.RepoID <= 0 || branch.Repo != nil {
			continue
		}
		branch.Repo = reposMap[branch.RepoID]
	}
	return nil
}

type FindBranchOptions struct {
	db.ListOptions
	RepoID             int64
	ExcludeBranchNames []string
	IsDeletedBranch    optional.Option[bool]
	OrderBy            string
	Keyword            string
}

func (opts FindBranchOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}

	if len(opts.ExcludeBranchNames) > 0 {
		cond = cond.And(builder.NotIn("name", opts.ExcludeBranchNames))
	}
	if opts.IsDeletedBranch.Has() {
		cond = cond.And(builder.Eq{"is_deleted": opts.IsDeletedBranch.Value()})
	}
	if opts.Keyword != "" {
		cond = cond.And(builder.Like{"name", opts.Keyword})
	}
	return cond
}

func (opts FindBranchOptions) ToOrders() string {
	orderBy := opts.OrderBy
	if orderBy == "" {
		// the commit_time might be the same, so add the "name" to make sure the order is stable
		orderBy = "commit_time DESC, name ASC"
	}
	if opts.IsDeletedBranch.ValueOrDefault(true) { // if deleted branch included, put them at the beginning
		orderBy = "is_deleted ASC, " + orderBy
	}
	return orderBy
}

func FindBranchNames(ctx context.Context, opts FindBranchOptions) ([]string, error) {
	sess := db.GetEngine(ctx).Select("name").Where(opts.ToConds())
	if opts.PageSize > 0 && !opts.IsListAll() {
		sess = db.SetSessionPagination(sess, &opts.ListOptions)
	}

	var branches []string
	if err := sess.Table("branch").OrderBy(opts.ToOrders()).Find(&branches); err != nil {
		return nil, err
	}
	return branches, nil
}

func FindBranchesByRepoAndBranchName(ctx context.Context, repoBranches map[int64]string) (map[int64]string, error) {
	cond := builder.NewCond()
	for repoID, branchName := range repoBranches {
		cond = cond.Or(builder.And(builder.Eq{"repo_id": repoID}, builder.Eq{"name": branchName}))
	}
	var branches []*Branch
	if err := db.GetEngine(ctx).
		Where(cond).Find(&branches); err != nil {
		return nil, err
	}
	branchMap := make(map[int64]string, len(branches))
	for _, branch := range branches {
		branchMap[branch.RepoID] = branch.CommitID
	}
	return branchMap, nil
}
