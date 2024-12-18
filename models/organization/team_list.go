// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"

	"xorm.io/builder"
)

type TeamList []*Team

func (t TeamList) LoadUnits(ctx context.Context) error {
	for _, team := range t {
		if err := team.LoadUnits(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (t TeamList) UnitMaxAccess(tp unit.Type) perm.AccessMode {
	maxAccess := perm.AccessModeNone
	for _, team := range t {
		if team.IsOwnerTeam() {
			return perm.AccessModeOwner
		}
		for _, teamUnit := range team.Units {
			if teamUnit.Type != tp {
				continue
			}
			if teamUnit.AccessMode > maxAccess {
				maxAccess = teamUnit.AccessMode
			}
		}
	}
	return maxAccess
}

// SearchTeamOptions holds the search options
type SearchTeamOptions struct {
	db.ListOptions
	UserID      int64
	Keyword     string
	OrgID       int64
	IncludeDesc bool
}

func (opts *SearchTeamOptions) toCond() builder.Cond {
	cond := builder.NewCond()

	if len(opts.Keyword) > 0 {
		lowerKeyword := strings.ToLower(opts.Keyword)
		var keywordCond builder.Cond = builder.Like{"lower_name", lowerKeyword}
		if opts.IncludeDesc {
			keywordCond = keywordCond.Or(builder.Like{"LOWER(description)", lowerKeyword})
		}
		cond = cond.And(keywordCond)
	}

	if opts.OrgID > 0 {
		cond = cond.And(builder.Eq{"`team`.org_id": opts.OrgID})
	}

	if opts.UserID > 0 {
		cond = cond.And(builder.Eq{"team_user.uid": opts.UserID})
	}

	return cond
}

// SearchTeam search for teams. Caller is responsible to check permissions.
func SearchTeam(ctx context.Context, opts *SearchTeamOptions) (TeamList, int64, error) {
	sess := db.GetEngine(ctx)

	opts.SetDefaultValues()
	cond := opts.toCond()

	if opts.UserID > 0 {
		sess = sess.Join("INNER", "team_user", "team_user.team_id = team.id")
	}
	sess = db.SetSessionPagination(sess, opts)

	teams := make([]*Team, 0, opts.PageSize)
	count, err := sess.Where(cond).OrderBy("lower_name").FindAndCount(&teams)
	if err != nil {
		return nil, 0, err
	}

	return teams, count, nil
}

// GetRepoTeams gets the list of teams that has access to the repository
func GetRepoTeams(ctx context.Context, orgID, repoID int64) (teams TeamList, err error) {
	return teams, db.GetEngine(ctx).
		Join("INNER", "team_repo", "team_repo.team_id = team.id").
		Where("team.org_id = ?", orgID).
		And("team_repo.repo_id=?", repoID).
		OrderBy("CASE WHEN name LIKE '" + OwnerTeamName + "' THEN '' ELSE name END").
		Find(&teams)
}

// GetUserOrgTeams returns all teams that user belongs to in given organization.
func GetUserOrgTeams(ctx context.Context, orgID, userID int64) (teams TeamList, err error) {
	return teams, db.GetEngine(ctx).
		Join("INNER", "team_user", "team_user.team_id = team.id").
		Where("team.org_id = ?", orgID).
		And("team_user.uid=?", userID).
		Find(&teams)
}

// GetUserRepoTeams returns user repo's teams
func GetUserRepoTeams(ctx context.Context, orgID, userID, repoID int64) (teams TeamList, err error) {
	return teams, db.GetEngine(ctx).
		Join("INNER", "team_user", "team_user.team_id = team.id").
		Join("INNER", "team_repo", "team_repo.team_id = team.id").
		Where("team.org_id = ?", orgID).
		And("team_user.uid=?", userID).
		And("team_repo.repo_id=?", repoID).
		Find(&teams)
}

func GetTeamsByOrgIDs(ctx context.Context, orgIDs []int64) (TeamList, error) {
	teams := make([]*Team, 0, 10)
	return teams, db.GetEngine(ctx).Where(builder.In("org_id", orgIDs)).Find(&teams)
}

func GetTeamsByIDs(ctx context.Context, teamIDs []int64) (map[int64]*Team, error) {
	teams := make(map[int64]*Team, len(teamIDs))
	return teams, db.GetEngine(ctx).Where(builder.In("`id`", teamIDs)).Find(&teams)
}
