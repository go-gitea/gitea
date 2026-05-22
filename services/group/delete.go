// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	repo_service "code.gitea.io/gitea/services/repository"
)

func DeleteGroup(ctx context.Context, doer *user_model.User, gid int64, recursive bool) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	sess := db.GetEngine(ctx)

	toDelete, err := group_model.GetGroupByID(ctx, gid)
	if err != nil {
		return err
	}

	childGroups, err := group_model.FindGroups(ctx, &group_model.FindGroupsOptions{
		ParentGroupID: gid,
	})
	if err != nil {
		return err
	}
	repos, cnt, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		GroupID: gid,
	})
	if err != nil {
		return err
	}
	if recursive {
		for _, repo := range repos {
			if err = repo_service.DeleteRepository(ctx, doer, repo, true); err != nil {
				return err
			}
		}
		for _, child := range childGroups {
			if err = DeleteGroup(ctx, doer, child.ID, recursive); err != nil {
				return err
			}
		}
	} else {
		// move all repos in the deleted group to its immediate parent
		_, inParent, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
			GroupID: toDelete.ParentGroupID,
		})
		if err != nil {
			return err
		}
		if cnt > 0 {
			for i, repo := range repos {
				repo.GroupID = toDelete.ParentGroupID
				repo.GroupSortOrder = int(inParent + int64(i) + 1)
			}
			if _, err = sess.Where("group_id = ?", gid).Update(&repos); err != nil {
				return err
			}
		}

		// move all child groups to the deleted group's immediate parent
		if len(childGroups) > 0 {
			inParent, err = group_model.CountGroups(ctx, &group_model.FindGroupsOptions{
				ParentGroupID: toDelete.ParentGroupID,
			})
			if err != nil {
				return err
			}
			for i, group := range childGroups {
				group.ParentGroupID = toDelete.ParentGroupID
				group.SortOrder = int(inParent) + i
			}
			if _, err = sess.Where("parent_group_id = ?", gid).Update(&childGroups); err != nil {
				return err
			}
		}
	}

	// finally, delete the group itself
	if _, err = sess.ID(gid).Delete(new(group_model.Group)); err != nil {
		return err
	}
	return committer.Commit()
}
