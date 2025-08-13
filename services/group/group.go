package group

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	"code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

func NewGroup(ctx context.Context, g *group_model.Group) (err error) {
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
		return
	}

	if err = RecalculateGroupAccess(ctx, g, true); err != nil {
		return
	}

	return committer.Commit()
}

func MoveRepositoryToGroup(ctx context.Context, repo *repo_model.Repository, newGroupID int64, groupSortOrder int) error {
	sess := db.GetEngine(ctx)
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

func MoveGroupItem(ctx context.Context, itemID, newParent int64, isGroup bool, newPos int) (err error) {
	var committer db.Committer
	ctx, committer, err = db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	var parentGroup *group_model.Group
	parentGroup, err = group_model.GetGroupByID(ctx, newParent)
	if err != nil {
		return err
	}
	err = parentGroup.LoadSubgroups(ctx, false)
	if err != nil {
		return err
	}
	if isGroup {
		var group *group_model.Group
		group, err = group_model.GetGroupByID(ctx, itemID)
		if err != nil {
			return err
		}
		if newPos < 0 {
			newPos = len(parentGroup.Subgroups)
		}
		if group.ParentGroupID != newParent || group.SortOrder != newPos {
			if err = group_model.MoveGroup(ctx, group, newParent, newPos); err != nil {
				return err
			}
			if err = RecalculateGroupAccess(ctx, group, false); err != nil {
				return err
			}
		}
	} else {
		var repo *repo_model.Repository
		repo, err = repo_model.GetRepositoryByID(ctx, itemID)
		if err != nil {
			return err
		}
		if newPos < 0 {
			var repoCount int64
			repoCount, err = repo_model.CountRepository(ctx, repo_model.SearchRepoOptions{
				GroupID: newParent,
			})
			if err != nil {
				return err
			}
			newPos = int(repoCount)
		}
		if repo.GroupID != newParent || repo.GroupSortOrder != newPos {
			if err = MoveRepositoryToGroup(ctx, repo, newParent, newPos); err != nil {
				return err
			}
		}
	}
	return committer.Commit()
}
