package group

import (
	"context"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	repo_model "code.gitea.io/gitea/models/repo"
)

func DeleteGroup(ctx context.Context, gid int64) error {
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

	// remove team permissions and units for deleted group
	if _, err = sess.Where("group_id = ?", gid).Delete(new(group_model.GroupTeam)); err != nil {
		return err
	}
	if _, err = sess.Where("group_id = ?", gid).Delete(new(group_model.GroupUnit)); err != nil {
		return err
	}

	// move all repos in the deleted group to its immediate parent
	repos, cnt, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		GroupID: gid,
	})
	if err != nil {
		return err
	}
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
	childGroups, err := group_model.FindGroups(ctx, &group_model.FindGroupsOptions{
		ParentGroupID: gid,
	})
	if err != nil {
		return err
	}
	if len(childGroups) > 0 {
		inParent, err = group_model.CountGroups(ctx, &group_model.FindGroupsOptions{
			ParentGroupID: toDelete.ParentGroupID,
		})
		if err != nil {
			return err
		}
		for i, group := range childGroups {
			group.ParentGroupID = toDelete.ParentGroupID
			group.SortOrder = int(inParent) + i + 1
		}
		if _, err = sess.Where("parent_group_id = ?", gid).Update(&childGroups); err != nil {
			return err
		}
	}

	// finally, delete the group itself
	if _, err = sess.ID(gid).Delete(new(group_model.Group)); err != nil {
		return err
	}
	return committer.Commit()
}
