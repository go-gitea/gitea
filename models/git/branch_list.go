// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

type BranchList []*Branch

func (branches BranchList) LoadDeletedBy(ctx context.Context) error {
	ids := container.Set[int64]{}
	for _, branch := range branches {
		ids.Add(branch.DeletedByID)
	}
	usersMap := make(map[int64]*user_model.User, len(ids))
	if err := db.GetEngine(ctx).In("id", ids.Values()).Find(&usersMap); err != nil {
		return err
	}
	for _, branch := range branches {
		branch.DeletedBy = usersMap[branch.DeletedByID]
		if branch.DeletedBy == nil {
			branch.DeletedBy = user_model.NewGhostUser()
		}
	}
	return nil
}

// LoadAllBranches loads all branches of a repository from database includes deleted branches
func LoadAllBranches(ctx context.Context, repoID int64) (BranchList, error) {
	var branches []*Branch
	err := db.GetEngine(ctx).Where("repo_id=?", repoID).
		Find(&branches)
	return branches, err
}

type FindBranchOptions struct {
	db.ListOptions
	RepoID             int64
	ExcludeBranchNames []string
	IsDeletedBranch    util.OptionalBool
}

func (opts *FindBranchOptions) Cond() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}

	if len(opts.ExcludeBranchNames) > 0 {
		cond = cond.And(builder.NotIn("name", opts.ExcludeBranchNames))
	}
	if !opts.IsDeletedBranch.IsNone() {
		cond = cond.And(builder.Eq{"is_deleted": opts.IsDeletedBranch.IsTrue()})
	}
	return cond
}

func CountBranches(ctx context.Context, opts FindBranchOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.Cond()).Count(&Branch{})
}

func FindBranches(ctx context.Context, opts FindBranchOptions) (BranchList, int64, error) {
	sess := db.GetEngine(ctx).Where(opts.Cond())
	if opts.PageSize > 0 && !opts.IsListAll() {
		sess = db.SetSessionPagination(sess, &opts.ListOptions)
	}

	var branches []*Branch
	total, err := sess.FindAndCount(&branches)
	if err != nil {
		return nil, 0, err
	}
	return branches, total, err
}

func FindBranchNames(ctx context.Context, opts FindBranchOptions) ([]string, error) {
	sess := db.GetEngine(ctx).Select("name").Where(opts.Cond())
	if opts.PageSize > 0 {
		sess = db.SetSessionPagination(sess, &opts.ListOptions)
	}
	var branches []string
	if err := sess.Table("branch").Find(&branches); err != nil {
		return nil, err
	}
	return branches, nil
}

func GetDeletedBranches(ctx context.Context, repoID int64) (BranchList, error) {
	branches, _, err := FindBranches(ctx, FindBranchOptions{
		ListOptions: db.ListOptions{
			PageSize: -1,
		},
		RepoID:          repoID,
		IsDeletedBranch: util.OptionalBoolTrue,
	})
	return branches, err
}
