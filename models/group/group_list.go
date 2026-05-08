// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"

	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
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

func universalGroupPermBuilder(idStr string, userID, orgID int64, includeAdmin bool) builder.Cond {
	adminSubquery := builder.Select("1").
		From("`user`").
		Where(builder.Eq{"`user`.is_admin": true, "`user`.`id`": userID})
	eqCond := builder.Eq{"`team_user`.uid": userID}

	teamSubquery := builder.Select("`team`.id").From("`team`").
		Join("LEFT", "team_user", "team.id = team_user.team_id").
		Join("LEFT", "`user` as iu", "iu.`id` = `team_user`.uid").
		Where(builder.And(
			builder.Eq{"`team`.org_id": orgID},
			eqCond,
			builder.Gte{"`team`.authorize": perm.AccessModeOwner},
		))

	sq := builder.Select("`repo_group`.id").
		From("`repo_group`").
		Join("LEFT", "team", "`team`.org_id = `repo_group`.owner_id").
		Where(
			builder.And(
				builder.Eq{"`repo_group`.owner_id": orgID},
				builder.And(
					builder.In("`team`.id", teamSubquery),
				)))
	cond := builder.In(idStr, sq)
	if includeAdmin {
		cond = cond.Or(builder.Exists(adminSubquery))
	}
	return cond
}

// userOrgTeamGroupBuilder returns group ids where user's teams can access.
func userOrgTeamGroupBuilder(userID, orgID int64) *builder.Builder {
	return builder.Select("`repo_group_team`.group_id").
		From("repo_group_team").
		Join("INNER", "team_user", "`team_user`.team_id = `repo_group_team`.team_id").
		Where(builder.And(builder.Eq{"`team_user`.uid": userID}, builder.Eq{"`repo_group_team`.org_id": orgID}))
}

// UserOrgTeamPermCond returns a condition to select ids of groups that a user can access at the level described by `level`
func UserOrgTeamPermCond(idStr string, userID, orgID int64, level perm.AccessMode) builder.Cond {
	selCond := userOrgTeamGroupBuilder(userID, orgID)
	selCond = selCond.InnerJoin("team", "`team`.id = `repo_group_team`.team_id").
		And(builder.Or(builder.Gte{"`team`.authorize": level}, builder.Gte{"`repo_group_team`.access_mode": level}))
	return builder.In(idStr, selCond)
}

// UserOrgTeamGroupCond returns a condition to select ids of groups that a user's team can access
func UserOrgTeamGroupCond(idStr string, userID, orgID int64) builder.Cond {
	return builder.In(idStr, userOrgTeamGroupBuilder(userID, orgID))
}

// userOrgTeamUnitGroupCond returns a condition to select group ids where user's teams can access the special unit.
func userOrgTeamUnitGroupCond(idStr string, userID, orgID int64, unitType unit.Type) builder.Cond {
	return builder.Or(builder.In(
		idStr, userOrgTeamUnitGroupBuilder(userID, orgID, unitType)))
}

// userOrgTeamUnitGroupBuilder returns group ids where user's teams can access the special unit.
func userOrgTeamUnitGroupBuilder(userID, orgID int64, unitType unit.Type) *builder.Builder {
	return userOrgTeamGroupBuilder(userID, orgID).
		Join("INNER", "team_unit", "`team_unit`.team_id = `repo_group_team`.team_id").
		Where(builder.Eq{"`team_unit`.`type`": unitType}).
		And(builder.Gt{"`team_unit`.`access_mode`": int(perm.AccessModeNone)})
}

// AccessibleGroupCondition returns a condition that matches groups which a user can access via the specified unit
func AccessibleGroupCondition(user *user_model.User, orgID int64, unitType unit.Type, minMode perm.AccessMode) builder.Cond {
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
	if user != nil {
		cond = cond.Or(universalGroupPermBuilder("`repo_group`.id", user.ID, orgID, true))
		cond = cond.Or(UserOrgTeamPermCond("`repo_group`.id", user.ID, orgID, minMode))
		if unitType == unit.TypeInvalid {
			cond = cond.Or(
				UserOrgTeamGroupCond("`repo_group`.id", user.ID, orgID),
			)
		} else {
			cond = cond.Or(
				userOrgTeamUnitGroupCond("`repo_group`.id", user.ID, orgID, unitType),
			)
		}
	}
	return cond
}
