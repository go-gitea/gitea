// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/usergroup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/container"

	"xorm.io/builder"
)

// TeamUser represents an team-user relation.
type TeamUser struct {
	ID     int64 `xorm:"pk autoincr"`
	OrgID  int64 `xorm:"INDEX"`
	TeamID int64 `xorm:"UNIQUE(s)"`
	UID    int64 `xorm:"UNIQUE(s)"`
}

// IsTeamMember returns true if given user is a member of team.
func IsTeamMember(ctx context.Context, orgID, teamID, userID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("org_id=?", orgID).
		And("team_id=?", teamID).
		And("uid=?", userID).
		Table("team_user").
		Exist()
}

// IsTeamMemberWithGroups returns true if given user is a member of team, including user groups.
func IsTeamMemberWithGroups(ctx context.Context, orgID, teamID, userID int64) (bool, error) {
	isMember, err := IsTeamMember(ctx, orgID, teamID, userID)
	if err != nil || isMember {
		return isMember, err
	}
	if !setting.Service.EnableUserGroups {
		return false, nil
	}

	groupIDs, err := GetTeamUserGroupIDs(ctx, teamID)
	if err != nil || len(groupIDs) == 0 {
		return false, err
	}

	effectiveGroupIDs, err := usergroup.ExpandUserGroupIDsToDescendants(ctx, groupIDs)
	if err != nil {
		return false, err
	}

	return db.GetEngine(ctx).
		Table("user_group_member").
		Where("user_id=?", userID).
		In("group_id", effectiveGroupIDs).
		Exist()
}

// GetUserGroupsInTeamForUser returns the (top-level) user groups assigned to the
// team that give the user effective membership. This is used to produce a clear
// warning message when a direct-add overlaps with group-based membership.
func GetUserGroupsInTeamForUser(ctx context.Context, teamID, userID int64) ([]*usergroup.UserGroup, error) {
	if !setting.Service.EnableUserGroups {
		return nil, nil
	}
	groupIDs, err := GetTeamUserGroupIDs(ctx, teamID)
	if err != nil || len(groupIDs) == 0 {
		return nil, err
	}

	// Expand to descendants so we can check every effective group.
	effectiveGroupIDs, err := usergroup.ExpandUserGroupIDsToDescendants(ctx, groupIDs)
	if err != nil {
		return nil, err
	}

	// Find which effective groups the user is directly in.
	var memberGroupIDs []int64
	if err := db.GetEngine(ctx).
		Table("user_group_member").
		Cols("group_id").
		Where("user_id=?", userID).
		In("group_id", effectiveGroupIDs).
		Find(&memberGroupIDs); err != nil {
		return nil, err
	}
	if len(memberGroupIDs) == 0 {
		return nil, nil
	}

	// Return only the top-level assigned groups that are ancestors of (or equal
	// to) the groups the user is actually in, to keep the message concise.
	ancestorIDs, err := usergroup.ExpandUserGroupIDsToAncestors(ctx, memberGroupIDs)
	if err != nil {
		return nil, err
	}
	ancestorSet := make(map[int64]struct{}, len(ancestorIDs))
	for _, id := range ancestorIDs {
		ancestorSet[id] = struct{}{}
	}

	var result []*usergroup.UserGroup
	for _, gid := range groupIDs {
		if _, ok := ancestorSet[gid]; ok {
			g, err := usergroup.GetUserGroupByID(ctx, gid)
			if err != nil {
				return nil, err
			}
			result = append(result, g)
		}
	}
	return result, nil
}

// SearchMembersOptions holds the search options
type SearchMembersOptions struct {
	db.ListOptions
	TeamID int64
}

// GetTeamMembers returns all members in given team of organization.
func GetTeamMembers(ctx context.Context, opts *SearchMembersOptions) ([]*user_model.User, error) {
	var members []*user_model.User
	sess := db.GetEngine(ctx)
	if opts.TeamID > 0 {
		sess = sess.In("id",
			builder.Select("uid").
				From("team_user").
				Where(builder.Eq{"team_id": opts.TeamID}),
		)
	}
	if opts.PageSize > 0 && opts.Page > 0 {
		sess = sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	if err := sess.OrderBy("full_name, name").Find(&members); err != nil {
		return nil, err
	}
	return members, nil
}

// GetTeamMembersWithGroups returns members in given team, including user group members.
func GetTeamMembersWithGroups(ctx context.Context, opts *SearchMembersOptions) ([]*user_model.User, error) {
	if opts.TeamID == 0 {
		return GetTeamMembers(ctx, opts)
	}
	if !setting.Service.EnableUserGroups {
		return GetTeamMembers(ctx, opts)
	}

	memberIDs := container.Set[int64]{}

	var directIDs []int64
	if err := db.GetEngine(ctx).Table("team_user").
		Where("team_id=?", opts.TeamID).
		Cols("uid").
		Find(&directIDs); err != nil {
		return nil, err
	}
	memberIDs.AddMultiple(directIDs...)

	groupIDs, err := GetTeamUserGroupIDs(ctx, opts.TeamID)
	if err != nil {
		return nil, err
	}

	if len(groupIDs) > 0 {
		groupMemberIDs, err := usergroup.GetEffectiveUserGroupMemberIDs(ctx, groupIDs)
		if err != nil {
			return nil, err
		}
		memberIDs.AddMultiple(groupMemberIDs...)
	}

	if len(memberIDs) == 0 {
		return nil, nil
	}

	sess := db.GetEngine(ctx).In("id", memberIDs.Values()).OrderBy(user_model.GetOrderByName())
	if opts.PageSize > 0 && opts.Page > 0 {
		sess = sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}

	var members []*user_model.User
	if err := sess.Find(&members); err != nil {
		return nil, err
	}
	return members, nil
}

// IsUserInTeams returns if a user in some teams
func IsUserInTeams(ctx context.Context, userID int64, teamIDs []int64) (bool, error) {
	return db.GetEngine(ctx).Where("uid=?", userID).In("team_id", teamIDs).Exist(new(TeamUser))
}

// IsUserInTeamsWithGroups returns true if a user is in any team or its assigned user groups.
func IsUserInTeamsWithGroups(ctx context.Context, userID int64, teamIDs []int64) (bool, error) {
	isMember, err := IsUserInTeams(ctx, userID, teamIDs)
	if err != nil || isMember {
		return isMember, err
	}
	if !setting.Service.EnableUserGroups {
		return false, nil
	}

	groupIDs, err := GetUserGroupIDsByTeamIDs(ctx, teamIDs)
	if err != nil || len(groupIDs) == 0 {
		return false, err
	}

	effectiveGroupIDs, err := usergroup.ExpandUserGroupIDsToDescendants(ctx, groupIDs)
	if err != nil {
		return false, err
	}

	return db.GetEngine(ctx).
		Table("user_group_member").
		Where("user_id=?", userID).
		In("group_id", effectiveGroupIDs).
		Exist()
}

// UsersInTeamsCount counts the number of users which are in userIDs and teamIDs
func UsersInTeamsCount(ctx context.Context, userIDs, teamIDs []int64) (int64, error) {
	var ids []int64
	if err := db.GetEngine(ctx).In("uid", userIDs).In("team_id", teamIDs).
		Table("team_user").
		Cols("uid").GroupBy("uid").Find(&ids); err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}
