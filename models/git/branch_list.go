// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
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

func LoadAllBranches(ctx context.Context, repoID int64) ([]*Branch, error) {
	var branches []*Branch
	err := db.GetEngine(ctx).Where("repo_id=?", repoID).Find(&branches)
	return branches, err
}

type FindBranchOptions struct {
	db.ListOptions
	RepoID               int64
	IncludeDefaultBranch bool
	IncludeDeletedBranch bool
}

func FindBranches(ctx context.Context, opts FindBranchOptions) (BranchList, int64, error) {
	sess := db.GetEngine(ctx).Where("repo_id=?", opts.RepoID)
	if opts.PageSize > 0 {
		sess = db.SetSessionPagination(sess, &opts.ListOptions)
	}
	if !opts.IncludeDefaultBranch {
		sess = sess.And(builder.Neq{"name": builder.Select("default_branch").From("repository").Where(builder.Eq{"id": opts.RepoID})})
	}
	if !opts.IncludeDeletedBranch {
		sess.And(builder.Eq{"is_deleted": false})
	}
	var branches []*Branch
	total, err := sess.FindAndCount(&branches)
	if err != nil {
		return nil, 0, err
	}
	return branches, total, err
}
