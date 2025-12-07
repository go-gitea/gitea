// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"

	"xorm.io/builder"
)

// TeamRepo represents an team-repository relation.
type TeamRepo struct {
	ID     int64 `xorm:"pk autoincr"`
	OrgID  int64 `xorm:"INDEX"`
	TeamID int64 `xorm:"UNIQUE(s)"`
	RepoID int64 `xorm:"UNIQUE(s)"`
}

// HasTeamRepo returns true if given repository belongs to team.
func HasTeamRepo(ctx context.Context, orgID, teamID, repoID int64) bool {
	has, _ := db.GetEngine(ctx).
		Where("org_id=?", orgID).
		And("team_id=?", teamID).
		And("repo_id=?", repoID).
		Get(new(TeamRepo))
	return has
}

// AddTeamRepo adds a repo for an organization's team
func AddTeamRepo(ctx context.Context, orgID, teamID, repoID int64) error {
	_, err := db.GetEngine(ctx).Insert(&TeamRepo{
		OrgID:  orgID,
		TeamID: teamID,
		RepoID: repoID,
	})
	return err
}

// RemoveTeamRepo remove repository from team
func RemoveTeamRepo(ctx context.Context, teamID, repoID int64) error {
	_, err := db.DeleteByBean(ctx, &TeamRepo{
		TeamID: teamID,
		RepoID: repoID,
	})
	return err
}

// GetTeamsWithAccessToAnyRepoUnit returns all teams in an organization that have given access level to the repository special unit.
// This function is only used for finding some teams that can be used as branch protection allowlist or reviewers, it isn't really used for access control.
// FIXME: TEAM-UNIT-PERMISSION this logic is not complete, search the fixme keyword to see more details
func GetTeamsWithAccessToAnyRepoUnit(ctx context.Context, orgID, repoID int64, mode perm.AccessMode, unitType unit.Type, unitTypesMore ...unit.Type) (teams []*Team, err error) {
	teamIDs, err := getTeamIDsWithAccessToAnyRepoUnit(ctx, orgID, repoID, mode, unitType, unitTypesMore...)
	if err != nil {
		return nil, err
	}
	if len(teamIDs) == 0 {
		return teams, nil
	}
	err = db.GetEngine(ctx).Where(builder.In("id", teamIDs)).OrderBy("team.name").Find(&teams)
	return teams, err
}

func getTeamIDsWithAccessToAnyRepoUnit(ctx context.Context, orgID, repoID int64, mode perm.AccessMode, unitType unit.Type, unitTypesMore ...unit.Type) (teamIDs []int64, err error) {
	sub := builder.Select("team_id").From("team_unit").
		Where(builder.Expr("team_unit.team_id = team.id")).
		And(builder.In("team_unit.type", append([]unit.Type{unitType}, unitTypesMore...))).
		And(builder.Expr("team_unit.access_mode >= ?", mode))

	err = db.GetEngine(ctx).
		Select("team.id").
		Table("team").
		Join("INNER", "team_repo", "team_repo.team_id = team.id").
		And("team_repo.org_id = ? AND team_repo.repo_id = ?", orgID, repoID).
		And(builder.Or(
			builder.Expr("team.authorize >= ?", mode),
			builder.In("team.id", sub),
		)).
		Find(&teamIDs)
	return teamIDs, err
}

func GetTeamUserIDsWithAccessToAnyRepoUnit(ctx context.Context, orgID, repoID int64, mode perm.AccessMode, unitType unit.Type, unitTypesMore ...unit.Type) (userIDs []int64, err error) {
	teamIDs, err := getTeamIDsWithAccessToAnyRepoUnit(ctx, orgID, repoID, mode, unitType, unitTypesMore...)
	if err != nil {
		return nil, err
	}
	if len(teamIDs) == 0 {
		return userIDs, nil
	}
	err = db.GetEngine(ctx).Table("team_user").Select("uid").Where(builder.In("team_id", teamIDs)).Find(&userIDs)
	return userIDs, err
}
