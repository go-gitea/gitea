package group

import (
	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"context"
)

type groupItemRepo struct {
	Repo *repo_model.Repository
}

func (repo *groupItemRepo) Link() string {
	return repo.Repo.Link()
}

func (repo *groupItemRepo) Title() string {
	return repo.Repo.Name
}

func (repo *groupItemRepo) Parent() GroupItem {
	if repo.Repo.GroupID == 0 {
		return nil
	}
	group, _ := group_model.GetGroupByID(db.DefaultContext, repo.Repo.GroupID)
	return &groupItemGroup{group}
}

func (repo *groupItemRepo) Children(doer *user_model.User) []GroupItem {
	return []GroupItem{}
}

func (repo *groupItemRepo) Avatar(ctx context.Context) string {
	return repo.Repo.AvatarLink(ctx)
}

func (repo *groupItemRepo) IsGroup() bool {
	return false
}

func (repo *groupItemRepo) HasChildren(doer *user_model.User) bool { return false }

func (repo *groupItemRepo) ID() int64 {
	return repo.Repo.ID
}

func (repo *groupItemRepo) Sort() int {
	return repo.Repo.GroupSortOrder
}
