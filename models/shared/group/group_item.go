// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"
	"slices"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"

	"xorm.io/builder"
)

// Item - represents an item in a group, either a repository or a subgroup.
// used to display, for example, the group sidebar
type Item interface {
	Link() string
	Title() string
	Parent() Item
	Children(doer *user_model.User) []Item
	Avatar(ctx context.Context) string
	HasChildren(doer *user_model.User) bool
	IsGroup() bool
	ID() int64
	Sort() int
}

type groupItemGroup struct {
	Group *group_model.Group
}

func (g *groupItemGroup) Link() string {
	return g.Group.GroupLink()
}

func (g *groupItemGroup) Title() string {
	return g.Group.Name
}

func (g *groupItemGroup) Parent() Item {
	if g.Group.ParentGroupID == 0 {
		return nil
	}
	group, _ := group_model.GetGroupByID(context.TODO(), g.Group.ParentGroupID)
	return &groupItemGroup{group}
}

func (g *groupItemGroup) Children(doer *user_model.User) []Item {
	var items []Item
	repos := make([]*repo_model.Repository, 0)
	sess := db.GetEngine(context.TODO())
	err := sess.Table("repository").
		Where("group_id = ?", g.Group.ID).
		And(builder.In("id", repo_model.AccessibleRepoIDsQuery(doer))).
		Find(&repos)
	if err != nil {
		log.Error("%w", err)
		return make([]Item, 0)
	}
	err = g.Group.LoadAccessibleSubgroups(context.TODO(), false, doer)
	if err != nil {
		return make([]Item, 0)
	}
	slices.SortStableFunc(g.Group.Subgroups, func(a, b *group_model.Group) int {
		return a.SortOrder - b.SortOrder
	})
	slices.SortStableFunc(repos, func(a, b *repo_model.Repository) int {
		return a.GroupSortOrder - b.GroupSortOrder
	})
	for _, sg := range g.Group.Subgroups {
		items = append(items, &groupItemGroup{sg})
	}
	for _, r := range repos {
		items = append(items, &groupItemRepo{r})
	}
	return items
}

func (g *groupItemGroup) Avatar(ctx context.Context) string {
	return g.Group.AvatarLink(ctx)
}

func (g *groupItemGroup) HasChildren(doer *user_model.User) bool {
	return len(g.Children(doer)) > 0
}

func (g *groupItemGroup) IsGroup() bool {
	return true
}

func (g *groupItemGroup) ID() int64 {
	return g.Group.ID
}

func (g *groupItemGroup) Sort() int {
	return g.Group.SortOrder
}

func GetTopLevelGroupItemList(ctx context.Context, orgID int64, doer *user_model.User) []Item {
	var rootItems []Item
	var doerID int64
	if doer != nil {
		doerID = doer.ID
	}
	groups, err := group_model.FindGroupsByCond(ctx, &group_model.FindGroupsOptions{
		ParentGroupID: 0,
		ActorID:       doerID,
		OwnerID:       orgID,
	}, group_model.
		AccessibleGroupCondition(doer, unit.TypeInvalid, perm.AccessModeRead))
	if err != nil {
		return nil
	}
	repos := make([]*repo_model.Repository, 0)
	cond := builder.NewCond().
		Or(builder.Eq{"repository.group_id": 0}, builder.IsNull{"repository.group_id"}).
		And(builder.Eq{"repository.owner_id": orgID}).
		And(builder.In("repository.id", repo_model.AccessibleRepoIDsQuery(doer)))
	sess := db.GetEngine(ctx)
	err = sess.Table("repository").Where(cond).Find(&repos)
	if err != nil {
		return nil
	}
	slices.SortStableFunc(groups, func(a, b *group_model.Group) int {
		return a.SortOrder - b.SortOrder
	})
	slices.SortStableFunc(repos, func(a, b *repo_model.Repository) int {
		return a.GroupSortOrder - b.GroupSortOrder
	})
	for _, g := range groups {
		rootItems = append(rootItems, &groupItemGroup{g})
	}
	for _, r := range repos {
		rootItems = append(rootItems, &groupItemRepo{r})
	}
	return rootItems
}

func ItemHasChild(ctx context.Context, it Item, other int64, doer *user_model.User) bool {
	for _, item := range it.Children(doer) {
		if ItemHasChild(ctx, item, other, doer) {
			return true
		}
	}
	return it.ID() == other
}
