// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/usergroup"

	"xorm.io/builder"
)

// TeamUserGroup represents a binding between a team and a user group.
type TeamUserGroup struct {
	TeamID  int64 `xorm:"UNIQUE(s) INDEX"`
	GroupID int64 `xorm:"UNIQUE(s) INDEX"`
	OrgID   int64 `xorm:"INDEX"`
}

func init() {
	db.RegisterModel(new(TeamUserGroup))
}

// AddUserGroupToTeam adds a user group to a team.
func AddUserGroupToTeam(ctx context.Context, teamID, groupID, orgID int64) error {
	return db.Insert(ctx, &TeamUserGroup{
		TeamID:  teamID,
		GroupID: groupID,
		OrgID:   orgID,
	})
}

// RemoveUserGroupFromTeam removes a user group from a team.
func RemoveUserGroupFromTeam(ctx context.Context, teamID, groupID int64) error {
	_, err := db.GetEngine(ctx).Delete(&TeamUserGroup{TeamID: teamID, GroupID: groupID})
	return err
}

// IsUserGroupInTeam checks whether the group is assigned to the team.
func IsUserGroupInTeam(ctx context.Context, teamID, groupID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("team_id=?", teamID).
		And("group_id=?", groupID).
		Exist(new(TeamUserGroup))
}

// GetTeamUserGroupIDs returns group IDs assigned to a team.
func GetTeamUserGroupIDs(ctx context.Context, teamID int64) ([]int64, error) {
	groupIDs := make([]int64, 0, 10)
	if err := db.GetEngine(ctx).Table("team_user_group").
		Where("team_id=?", teamID).
		Cols("group_id").
		Find(&groupIDs); err != nil {
		return nil, err
	}
	return groupIDs, nil
}

// GetTeamUserGroupCounts returns the number of user groups assigned to each team.
func GetTeamUserGroupCounts(ctx context.Context, teamIDs []int64) (map[int64]int64, error) {
	counts := make(map[int64]int64, len(teamIDs))
	if len(teamIDs) == 0 {
		return counts, nil
	}

	type teamUserGroupCount struct {
		TeamID int64
		Count  int64
	}

	rows := make([]teamUserGroupCount, 0, len(teamIDs))
	if err := db.GetEngine(ctx).Table("team_user_group").
		In("team_id", teamIDs).
		Select("team_id, COUNT(*) AS count").
		GroupBy("team_id").
		Find(&rows); err != nil {
		return nil, err
	}

	for _, row := range rows {
		counts[row.TeamID] = row.Count
	}
	return counts, nil
}

// GetUserGroupIDsByTeamIDs returns group IDs assigned to teams.
func GetUserGroupIDsByTeamIDs(ctx context.Context, teamIDs []int64) ([]int64, error) {
	groupIDs := make([]int64, 0, 10)
	if len(teamIDs) == 0 {
		return groupIDs, nil
	}

	if err := db.GetEngine(ctx).Table("team_user_group").
		In("team_id", teamIDs).
		Distinct("group_id").
		Find(&groupIDs); err != nil {
		return nil, err
	}
	return groupIDs, nil
}

// GetTeamUserGroups returns user groups assigned to a team.
func GetTeamUserGroups(ctx context.Context, teamID int64) ([]*usergroup.UserGroup, error) {
	groups := make([]*usergroup.UserGroup, 0, 10)
	err := db.GetEngine(ctx).
		Join("INNER", "team_user_group", "team_user_group.group_id = user_group.id").
		Where("team_user_group.team_id=?", teamID).
		OrderBy("user_group.lower_name").
		Find(&groups)
	return groups, err
}

// GetTeamIDsByUserGroupIDs returns team IDs that include any of the group IDs.
func GetTeamIDsByUserGroupIDs(ctx context.Context, orgID int64, groupIDs []int64) ([]int64, error) {
	teamIDs := make([]int64, 0, 10)
	if len(groupIDs) == 0 {
		return teamIDs, nil
	}

	sess := db.GetEngine(ctx).Table("team_user_group").
		In("group_id", groupIDs).
		Distinct("team_id").
		Cols("team_id")
	if orgID > 0 {
		sess = sess.And(builder.Eq{"org_id": orgID})
	}
	if err := sess.Find(&teamIDs); err != nil {
		return nil, err
	}
	return teamIDs, nil
}

// GetTeamIDsByUserGroupID returns team IDs that include the group ID.
func GetTeamIDsByUserGroupID(ctx context.Context, groupID int64) ([]int64, error) {
	teamIDs := make([]int64, 0, 10)
	if err := db.GetEngine(ctx).Table("team_user_group").
		Where("group_id=?", groupID).
		Cols("team_id").
		Find(&teamIDs); err != nil {
		return nil, err
	}
	return teamIDs, nil
}

// TeamWithOrg bundles a Team together with its owning Organisation for display purposes.
type TeamWithOrg struct {
	Team *Team
	Org  *Organization
}

// GetTeamsWithOrgByUserGroupID returns all teams (with their org) that directly
// reference the given user group, ordered by org name then team name.
func GetTeamsWithOrgByUserGroupID(ctx context.Context, groupID int64) ([]*TeamWithOrg, error) {
	teams := make([]*Team, 0, 10)
	if err := db.GetEngine(ctx).
		Join("INNER", "team_user_group", "team_user_group.team_id = team.id").
		Where("team_user_group.group_id=?", groupID).
		OrderBy("team.org_id, team.lower_name").
		Find(&teams); err != nil {
		return nil, err
	}

	// Collect unique org IDs and fetch them in one query.
	orgIDSet := make(map[int64]struct{}, len(teams))
	for _, t := range teams {
		orgIDSet[t.OrgID] = struct{}{}
	}
	orgIDs := make([]int64, 0, len(orgIDSet))
	for id := range orgIDSet {
		orgIDs = append(orgIDs, id)
	}

	orgs := make([]*Organization, 0, len(orgIDs))
	if len(orgIDs) > 0 {
		if err := db.GetEngine(ctx).In("id", orgIDs).Find(&orgs); err != nil {
			return nil, err
		}
	}
	orgMap := make(map[int64]*Organization, len(orgs))
	for _, o := range orgs {
		orgMap[o.ID] = o
	}

	result := make([]*TeamWithOrg, 0, len(teams))
	for _, t := range teams {
		result = append(result, &TeamWithOrg{Team: t, Org: orgMap[t.OrgID]})
	}
	return result, nil
}

// GetAvailableUserGroupsForTeam returns all user groups not yet assigned to the given team,
// ordered by name, for display in the "add group" dropdown.
func GetAvailableUserGroupsForTeam(ctx context.Context, teamID int64) ([]*usergroup.UserGroup, error) {
	assigned, err := GetTeamUserGroupIDs(ctx, teamID)
	if err != nil {
		return nil, err
	}

	sess := db.GetEngine(ctx).OrderBy("lower_name")
	if len(assigned) > 0 {
		sess = sess.NotIn("id", assigned)
	}

	var groups []*usergroup.UserGroup
	return groups, sess.Find(&groups)
}
