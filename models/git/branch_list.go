// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

type BranchList []*Branch

func (branches BranchList) LoadDeletedBy(ctx context.Context) error {
	ids := container.Set[int64]{}
	for _, branch := range branches {
		if !branch.IsDeleted {
			continue
		}
		ids.Add(branch.DeletedByID)
	}
	usersMap := make(map[int64]*user_model.User, len(ids))
	if err := db.GetEngine(ctx).In("id", ids.Values()).Find(&usersMap); err != nil {
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
	ids := container.Set[int64]{}
	for _, branch := range branches {
		if branch.PusherID > 0 { // pusher_id maybe zero because some branches are sync by backend with no pusher
			ids.Add(branch.PusherID)
		}
	}
	usersMap := make(map[int64]*user_model.User, len(ids))
	if err := db.GetEngine(ctx).In("id", ids.Values()).Find(&usersMap); err != nil {
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
	ids := container.Set[int64]{}
	for _, branch := range branches {
		if branch.RepoID > 0 {
			ids.Add(branch.RepoID)
		}
	}
	reposMap := make(map[int64]*repo_model.Repository, len(ids))
	if err := db.GetEngine(ctx).In("id", ids.Values()).Find(&reposMap); err != nil {
		return err
	}
	for _, branch := range branches {
		if branch.RepoID <= 0 {
			continue
		}
		branch.Repo = reposMap[branch.RepoID]
	}
	return nil
}

type FindBranchOptions struct {
	db.ListOptions
	RepoID             int64
	RepoCond           builder.Cond
	ExcludeBranchNames []string
	CommitCond         builder.Cond
	PusherID           int64
	IsDeletedBranch    util.OptionalBool
	CommitAfterUnix    int64
	CommitBeforeUnix   int64
	OrderBy            string
	Keyword            string

	// find branch by pull request
	PullRequestCond builder.Cond
}

func (opts FindBranchOptions) ToConds() builder.Cond {
	cond := builder.NewCond()

	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.RepoCond != nil {
		cond = cond.And(opts.RepoCond)
	}

	if len(opts.ExcludeBranchNames) > 0 {
		cond = cond.And(builder.NotIn("branch.name", opts.ExcludeBranchNames))
	}

	if opts.CommitCond != nil {
		cond = cond.And(opts.CommitCond)
	}

	if opts.PusherID > 0 {
		cond = cond.And(builder.Eq{"branch.pusher_id": opts.PusherID})
	}

	if !opts.IsDeletedBranch.IsNone() {
		cond = cond.And(builder.Eq{"branch.is_deleted": opts.IsDeletedBranch.IsTrue()})
	}
	if opts.Keyword != "" {
		cond = cond.And(builder.Like{"name", opts.Keyword})
	}

	if opts.CommitAfterUnix != 0 {
		cond = cond.And(builder.Gte{"branch.commit_time": opts.CommitAfterUnix})
	}
	if opts.CommitBeforeUnix != 0 {
		cond = cond.And(builder.Lte{"branch.commit_time": opts.CommitBeforeUnix})
	}

	if opts.PullRequestCond != nil {
		cond = cond.And(opts.PullRequestCond)
	}

	return cond
}

func (opts FindBranchOptions) ToOrders() string {
	orderBy := opts.OrderBy
	if !opts.IsDeletedBranch.IsFalse() { // if deleted branch included, put them at the end
		if orderBy != "" {
			orderBy += ", "
		}
		orderBy += "is_deleted ASC"
	}
	if orderBy == "" {
		// the commit_time might be the same, so add the "name" to make sure the order is stable
		return "commit_time DESC, name ASC"
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
