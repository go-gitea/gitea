// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

const NestingLimit = 20

// Group represents a group of repositories for a user or organization
type Group struct {
	ID          int64 `xorm:"pk autoincr"`
	OwnerID     int64 `xorm:"INDEX NOT NULL"`
	OwnerName   string
	Owner       *user_model.User    `xorm:"-"`
	LowerName   string              `xorm:"TEXT NOT NULL"`
	Name        string              `xorm:"TEXT NOT NULL"`
	Description string              `xorm:"TEXT"`
	Visibility  structs.VisibleType `xorm:"NOT NULL DEFAULT 0"`
	Avatar      string              `xorm:"VARCHAR(64)"`

	ParentGroupID int64         `xorm:"INDEX DEFAULT NULL"`
	ParentGroup   *Group        `xorm:"-"`
	Subgroups     RepoGroupList `xorm:"-"`

	SortOrder int `xorm:"INDEX"`
}

// GroupLink returns the link to this group
func (g *Group) GroupLink() string {
	return setting.AppSubURL + "/" + url.PathEscape(g.OwnerName) + "/groups/" + strconv.FormatInt(g.ID, 10)
}

func (g *Group) OrgGroupLink() string {
	return setting.AppSubURL + "/org/" + url.PathEscape(g.OwnerName) + "/groups/" + strconv.FormatInt(g.ID, 10)
}

func (g *Group) UserGroupLink() string {
	return setting.AppSubURL + "/" + url.PathEscape(g.OwnerName) + "/-/groups/" + strconv.FormatInt(g.ID, 10)
}

func (Group) TableName() string { return "repo_group" }

func init() {
	db.RegisterModel(new(Group))
}

func (g *Group) doLoadSubgroups(ctx context.Context, recursive bool, cond builder.Cond, currentLevel int) error {
	if currentLevel >= NestingLimit {
		return ErrGroupTooDeep{
			g.ID,
		}
	}
	if g.Subgroups != nil {
		return nil
	}
	var err error
	g.Subgroups, err = FindGroupsByCond(ctx, &FindGroupsOptions{
		ParentGroupID: g.ID,
	}, cond)
	if err != nil {
		return err
	}
	slices.SortStableFunc(g.Subgroups, func(a, b *Group) int {
		return a.SortOrder - b.SortOrder
	})
	if recursive {
		for _, group := range g.Subgroups {
			err = group.doLoadSubgroups(ctx, recursive, cond, currentLevel+1)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *Group) LoadSubgroups(ctx context.Context, recursive bool) error {
	fgo := &FindGroupsOptions{
		ParentGroupID: g.ID,
	}
	return g.doLoadSubgroups(ctx, recursive, fgo.ToConds(), 0)
}

func (g *Group) LoadAccessibleSubgroups(ctx context.Context, recursive bool, doer *user_model.User, requireMember bool) error {
	cond := AccessibleGroupCondition(doer)
	if requireMember {
		cond = builder.And(MemberCond("`repo_group`.parent_group_id", g.ID, doer), cond)
	}
	return g.doLoadSubgroups(ctx, recursive, cond, 0)
}

func (g *Group) LoadAttributes(ctx context.Context) error {
	err := g.LoadOwner(ctx)
	if err != nil {
		return err
	}
	return g.LoadParentGroup(ctx)
}

func (g *Group) LoadParentGroup(ctx context.Context) error {
	if g.ParentGroup != nil {
		return nil
	}
	if g.ParentGroupID == 0 {
		return nil
	}
	parentGroup, err := GetGroupByID(ctx, g.ParentGroupID)
	if err != nil {
		return err
	}
	g.ParentGroup = parentGroup
	return nil
}

func (g *Group) LoadOwner(ctx context.Context) error {
	if g.Owner != nil {
		return nil
	}
	var err error
	g.Owner, err = user_model.GetUserByID(ctx, g.OwnerID)
	return err
}

func (g *Group) ShortName(length int) string {
	return util.EllipsisDisplayString(g.Name, length)
}

// Depth retrieves the depth/nesting level of this group
func (g *Group) Depth(ctx context.Context) (d int) {
	pgids, err := GetParentGroupIDChain(ctx, g.ID)
	if err != nil {
		return 0
	}
	return len(pgids) - 1
}

// DisplayLeftMargin generates a value for the left margin
// displayed on the frontend beside this group
func (g *Group) DisplayLeftMargin(ctx context.Context) string {
	return fmt.Sprintf("%drem", g.Depth(ctx)+1)
}
