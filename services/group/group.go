// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"code.gitea.io/gitea/modules/gitrepo"
	"context"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

func NewGroup(ctx context.Context, g *group_model.Group) error {
	var err error
	if len(g.Name) == 0 {
		return util.NewInvalidArgumentErrorf("empty group name")
	}
	has, err := db.ExistByID[user_model.User](ctx, g.OwnerID)
	if err != nil {
		return err
	}
	if !has {
		return organization.ErrOrgNotExist{ID: g.OwnerID}
	}
	g.LowerName = strings.ToLower(g.Name)
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = db.Insert(ctx, g); err != nil {
		return err
	}

	if err = RecalculateGroupAccess(ctx, g, true); err != nil {
		return err
	}

	return committer.Commit()
}

func MoveRepositoryToGroup(ctx context.Context, repo *repo_model.Repository, newGroupID int64, groupSortOrder int) error {
	sess := db.GetEngine(ctx)
	if newGroupID > 0 {
		newGroup, err := group_model.GetGroupByID(ctx, newGroupID)
		if err != nil {
			return err
		}
		if newGroup.OwnerID != repo.OwnerID {
			return fmt.Errorf("repo[%d]'s ownerID is not equal to new parent group[%d]'s owner ID", repo.ID, newGroup.ID)
		}
	}
	repo.GroupID = newGroupID
	repo.GroupSortOrder = groupSortOrder
	cnt, err := sess.
		Table("repository").
		ID(repo.ID).
		MustCols("group_id").
		Update(repo)
	log.Info("updated %d rows?", cnt)
	return err
}

type MoveGroupOptions struct {
	NewParent, ItemID int64
	IsGroup           bool
	NewPos            int
}

func MoveGroupItem(ctx context.Context, opts MoveGroupOptions, doer *user_model.User) (err error) {
	var committer db.Committer
	ctx, committer, err = db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	var parentGroup *group_model.Group
	parentGroup, err = group_model.GetGroupByID(ctx, opts.NewParent)
	if err != nil {
		return err
	}
	canAccessNewParent, err := parentGroup.CanAccess(ctx, doer)
	if err != nil {
		return err
	}
	if !canAccessNewParent {
		return errors.New("cannot access new parent group")
	}

	err = parentGroup.LoadSubgroups(ctx, false)
	if err != nil {
		return err
	}
	if opts.IsGroup {
		var group *group_model.Group
		group, err = group_model.GetGroupByID(ctx, opts.ItemID)
		if err != nil {
			return err
		}
		if opts.NewPos < 0 {
			opts.NewPos = len(parentGroup.Subgroups)
		}
		if group.ParentGroupID != opts.NewParent || group.SortOrder != opts.NewPos {
			if err = group_model.MoveGroup(ctx, group, opts.NewParent, opts.NewPos); err != nil {
				return err
			}
			if err = RecalculateGroupAccess(ctx, group, false); err != nil {
				return err
			}
		}
	} else {
		var repo *repo_model.Repository
		repo, err = repo_model.GetRepositoryByID(ctx, opts.ItemID)
		if err != nil {
			return err
		}
		if opts.NewPos < 0 {
			var repoCount int64
			repoCount, err = repo_model.CountRepository(ctx, repo_model.SearchRepoOptions{
				GroupID: opts.NewParent,
			})
			if err != nil {
				return err
			}
			opts.NewPos = int(repoCount)
		}
		if repo.GroupID != opts.NewParent || repo.GroupSortOrder != opts.NewPos {
			if err = gitrepo.RenameRepository(ctx, repo, repo_model.StorageRepo(repo_model.RelativePath(repo.OwnerName, repo.Name, opts.NewParent))); err != nil {
				return err
			}
			if err = MoveRepositoryToGroup(ctx, repo, opts.NewParent, opts.NewPos); err != nil {
				return err
			}
		}
	}
	return committer.Commit()
}
