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

// userOrgTeamGroupBuilder returns group ids where user's teams can access.
func userOrgTeamGroupBuilder(userID int64) *builder.Builder {
	return builder.Select("`repo_group_team`.group_id").
		From("repo_group_team").
		Join("INNER", "team_user", "`team_user`.team_id = `repo_group_team`.team_id").
		Where(builder.Eq{"`team_user`.uid": userID})
}

func UserOrgTeamPermCond(idStr string, userID int64, level perm.AccessMode) builder.Cond {
	selCond := userOrgTeamGroupBuilder(userID)
	selCond = selCond.InnerJoin("team", "`team`.id = `repo_group_team`.team_id").
		And(builder.Or(builder.Gte{"`team`.authorize": level}, builder.Gte{"`repo_group_team`.access_mode": level}))
	return builder.In(idStr, selCond)
}

// UserOrgTeamGroupCond returns a condition to select ids of groups that a user's team can access
func UserOrgTeamGroupCond(idStr string, userID int64) builder.Cond {
	return builder.In(idStr, userOrgTeamGroupBuilder(userID))
}

// userOrgTeamUnitGroupCond returns a condition to select group ids where user's teams can access the special unit.
func userOrgTeamUnitGroupCond(idStr string, userID int64, unitType unit.Type) builder.Cond {
	return builder.Or(builder.In(
		idStr, userOrgTeamUnitGroupBuilder(userID, unitType)))
}

// userOrgTeamUnitGroupBuilder returns group ids where user's teams can access the special unit.
func userOrgTeamUnitGroupBuilder(userID int64, unitType unit.Type) *builder.Builder {
	return userOrgTeamGroupBuilder(userID).
		Join("INNER", "team_unit", "`team_unit`.team_id = `team_repo`.team_id").
		Where(builder.Eq{"`team_unit`.`type`": unitType}).
		And(builder.Gt{"`team_unit`.`access_mode`": int(perm.AccessModeNone)})
}

// AccessibleGroupCondition returns a condition that matches groups which a user can access via the specified unit
func AccessibleGroupCondition(user *user_model.User, unitType unit.Type) builder.Cond {
	cond := builder.NewCond()
	if user == nil || !user.IsRestricted || user.ID <= 0 {
		orgVisibilityLimit := []structs.VisibleType{structs.VisibleTypePrivate}
		if user == nil || user.ID <= 0 {
			orgVisibilityLimit = append(orgVisibilityLimit, structs.VisibleTypeLimited)
		}
		cond = cond.Or(builder.And(
			builder.Eq{"`repo_group`.is_private": false},
			builder.NotIn("`repo_group`.owner_id", builder.Select("id").From("`user`").Where(
				builder.And(
					builder.Eq{"type": user_model.UserTypeOrganization},
					builder.In("visibility", orgVisibilityLimit)),
			))))
	}
	if user != nil {
		if unitType == unit.TypeInvalid {
			cond = cond.Or(
				UserOrgTeamGroupCond("`repo_group`.id", user.ID),
			)
		} else {
			cond = cond.Or(
				userOrgTeamUnitGroupCond("`repo_group`.id", user.ID, unitType),
			)
		}
	}
	return cond
}
