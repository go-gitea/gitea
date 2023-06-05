// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"

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

// GetTeamUsersByTeamID returns team users for a team
func GetTeamUsersByTeamID(ctx context.Context, teamID int64) ([]*TeamUser, error) {
	teamUsers := make([]*TeamUser, 0, 10)
	return teamUsers, db.GetEngine(ctx).
		Where("team_id=?", teamID).
		Find(&teamUsers)
}

// SearchMembersOptions holds the search options
type SearchMembersOptions struct {
	db.ListOptions
	TeamID int64
}

func (opts SearchMembersOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.TeamID > 0 {
		cond = cond.And(builder.Eq{"": opts.TeamID})
	}
	return cond
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

// IsUserInTeams returns if a user in some teams
func IsUserInTeams(ctx context.Context, userID int64, teamIDs []int64) (bool, error) {
	return db.GetEngine(ctx).Where("uid=?", userID).In("team_id", teamIDs).Exist(new(TeamUser))
}

// UsersInTeamsCount counts the number of users which are in userIDs and teamIDs
func UsersInTeamsCount(userIDs, teamIDs []int64) (int64, error) {
	var ids []int64
	if err := db.GetEngine(db.DefaultContext).In("uid", userIDs).In("team_id", teamIDs).
		Table("team_user").
		Cols("uid").GroupBy("uid").Find(&ids); err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}
