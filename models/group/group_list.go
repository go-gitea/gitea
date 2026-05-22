// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"

	"xorm.io/builder"
)

type RepoGroupList []*Group

func (groups RepoGroupList) LoadOwners(ctx context.Context) error {
	for _, g := range groups {
		if g.Owner == nil {
			err := g.LoadOwner(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func orgMembershipGroupBuilder(userID int64) *builder.Builder {
	return builder.Select("`org_user`.org_id").
		From("org_user").
		Where(builder.Eq{"`org_user`.uid": userID})
}

// MemberCond returns a cond that checks if a user is a member of a group
func MemberCond(idStr string, groupID int64, user *user_model.User) builder.Cond {
	if user == nil || user.ID <= 0 {
		return builder.Expr("1 = 0")
	}
	whereCond := builder.In("`repo_group`.owner_id", orgMembershipGroupBuilder(user.ID))
	if groupID > 0 {
		whereCond = whereCond.And(builder.Eq{idStr: groupID})
	}
	return whereCond
}

// AccessibleGroupCondition returns a condition that matches groups which a user can access via the specified unit
func AccessibleGroupCondition(user *user_model.User) builder.Cond {
	if user != nil && user.IsAdmin {
		return builder.Expr("1 = 1")
	}

	cond := builder.NewCond()
	if user == nil || !user.IsRestricted || user.ID <= 0 {
		orgVisibilityLimit := []int{int(structs.VisibleTypePrivate)}
		if user == nil || user.ID <= 0 {
			orgVisibilityLimit = append(orgVisibilityLimit, int(structs.VisibleTypeLimited))
		}
		condAnd := builder.And(
			builder.NotIn("`repo_group`.owner_id", builder.Select("`user`.`id`").From("`user`").Where(
				builder.And(
					builder.Eq{"type": user_model.UserTypeOrganization},
					builder.In("visibility", orgVisibilityLimit)),
			)))
		condAnd = condAnd.And(builder.NotIn("`repo_group`.visibility", orgVisibilityLimit))
		cond = cond.Or(condAnd)
	}
	if user != nil && user.ID > 0 {
		adminSubquery := builder.Dialect(db.BuilderDialect()).Select("1").
			From("`user`").
			Where(builder.Eq{"`user`.is_admin": true, "`user`.`id`": user.ID})
		cond = cond.Or(builder.In("`repo_group`.owner_id", orgMembershipGroupBuilder(user.ID)), builder.Exists(adminSubquery))
	}
	return cond
}
