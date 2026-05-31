// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitea.dev/models/db"
	group_model "gitea.dev/models/group"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

func NewGroup(ctx context.Context, g *group_model.Group) error {
	var err error
	if len(g.Name) == 0 {
		return util.NewInvalidArgumentErrorf("empty group name")
	}
	owner, has, err := db.GetByID[user_model.User](ctx, g.OwnerID)
	if err != nil {
		return err
	}
	if !has {
		return organization.ErrOrgNotExist{ID: g.OwnerID}
	}
	g.OwnerName = owner.Name
	g.LowerName = strings.ToLower(g.Name)
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if g.ParentGroupID > 0 {
		ngrp, err := group_model.GetGroupByID(ctx, g.ParentGroupID)
		if err != nil {
			return err
		}
		if err = ngrp.LoadSubgroups(ctx, false); err != nil {
			return err
		}
		g.SortOrder = len(ngrp.Subgroups)
		gidChain, err := group_model.GetParentGroupIDChain(ctx, g.ParentGroupID)
		if err != nil {
			return err
		}
		if len(gidChain) >= 20 {
			return group_model.ErrGroupTooDeep{
				ID: g.ParentGroupID,
			}
		}
	} else {
		siblings, err := group_model.FindGroups(ctx, &group_model.FindGroupsOptions{
			ParentGroupID: 0,
			OwnerID:       g.OwnerID,
		})
		if err != nil {
			return err
		}
		g.SortOrder = len(siblings)
	}

	if err = db.Insert(ctx, g); err != nil {
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
	oldGroupID := repo.GroupID

	repo.GroupID = newGroupID
	repo.GroupSortOrder = groupSortOrder
	if _, err := sess.
		Table("repository").
		ID(repo.ID).
		MustCols("group_id", "group_sort_order").
		Update(repo); err != nil {
		return err
	}

	newSiblings, _, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		GroupID: repo.GroupID,
		OrderBy: "group_sort_order ASC",
	})
	if err != nil {
		return err
	}
	for i, newSibling := range newSiblings {
		newSibling.GroupSortOrder = i
		_, err = sess.
			Table("repository").
			ID(newSibling.ID).
			MustCols("group_id", "group_sort_order").
			Update(newSibling)
		if err != nil {
			return err
		}
	}

	// re-index items in old parent group
	if oldGroupID != repo.GroupID {
		prevSiblings, _, err := repo_model.SearchRepository(ctx,
			repo_model.SearchRepoOptions{
				GroupID: oldGroupID,
				OrderBy: "group_sort_order ASC",
			})
		if err != nil {
			return err
		}
		for i, prevSibling := range prevSiblings {
			prevSibling.GroupSortOrder = i
			_, err = sess.
				Table("repository").
				ID(prevSibling.ID).
				MustCols("group_id", "group_sort_order").
				Update(prevSibling)
			if err != nil {
				return err
			}
		}
	}

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
	if opts.NewParent > 0 {
		parentGroup, err = group_model.GetGroupByID(ctx, opts.NewParent)
		if err != nil {
			return err
		}
		canAccessNewParent, err := parentGroup.CanAccess(ctx, doer)
		if err != nil {
			return err
		}
		if !canAccessNewParent {
			return group_model.ErrUserDoesNotHaveAccessToGroup{
				GroupID: opts.NewParent,
				UserID:  doer.ID,
			}
		}

		err = parentGroup.LoadSubgroups(ctx, false)
		if err != nil {
			return err
		}
	}
	if opts.IsGroup {
		var group *group_model.Group
		group, err = group_model.GetGroupByID(ctx, opts.ItemID)
		if err != nil {
			return err
		}
		if opts.NewPos < 0 && parentGroup != nil {
			opts.NewPos = len(parentGroup.Subgroups)
		}
		if group.ParentGroupID != opts.NewParent || group.SortOrder != opts.NewPos {
			if parentGroup != nil && group.OwnerID != parentGroup.OwnerID {
				return util.NewInvalidArgumentErrorf("New parent group %d does not belong to same owner [ID: %d]", parentGroup.ID, parentGroup.OwnerID)
			}
			if err = group_model.MoveGroup(ctx, group, opts.NewParent, opts.NewPos); err != nil {
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
			if parentGroup != nil && repo.OwnerID != parentGroup.OwnerID {
				return util.NewInvalidArgumentErrorf("New parent group %d does not belong to same owner [ID: %d]", parentGroup.ID, parentGroup.OwnerID)
			}
			ndir := filepath.Dir(filepath.Join(setting.RepoRootPath, filepath.FromSlash(repo_model.RelativePath(repo.OwnerName, repo.Name, opts.NewParent))))
			_, err = os.Stat(ndir)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					if err = os.MkdirAll(ndir, 0o755); err != nil {
						return err
					}
				} else {
					return err
				}
			}
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
