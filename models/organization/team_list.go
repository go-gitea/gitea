// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"
	"maps"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
	"gitea.dev/models/usergroup"
	"gitea.dev/modules/setting"

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
	db.SetSessionPagination(sess, opts)

	teams := make([]*Team, 0, opts.PageSize)
	count, err := sess.Where(cond).OrderBy("CASE WHEN name=? THEN '' ELSE lower_name END", OwnerTeamName).FindAndCount(&teams)
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
	teamMap := make(map[int64]*Team)
	if err := db.GetEngine(ctx).
		Join("INNER", "team_user", "team_user.team_id = team.id").
		Where("team.org_id = ?", orgID).
		And("team_user.uid=?", userID).
		Find(&teams); err != nil {
		return nil, err
	}
	for _, team := range teams {
		teamMap[team.ID] = team
	}
	if !setting.Service.EnableUserGroups {
		teams = make(TeamList, 0, len(teamMap))
		for _, team := range teamMap {
			teams = append(teams, team)
		}
		return teams, nil
	}

	groupIDs, err := usergroup.GetUserGroupIDsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(groupIDs) > 0 {
		ancestorIDs, err := usergroup.ExpandUserGroupIDsToAncestors(ctx, groupIDs)
		if err != nil {
			return nil, err
		}
		teamIDs, err := GetTeamIDsByUserGroupIDs(ctx, orgID, ancestorIDs)
		if err != nil {
			return nil, err
		}
		if len(teamIDs) > 0 {
			teamsByID, err := GetTeamsByIDs(ctx, teamIDs)
			if err != nil {
				return nil, err
			}
			maps.Copy(teamMap, teamsByID)
		}
	}

	teams = make(TeamList, 0, len(teamMap))
	for _, team := range teamMap {
		teams = append(teams, team)
	}
	return teams, nil
}

// GetUserRepoTeamsWithGroups returns all teams that grant the user access to the given
// repository, including teams joined via user groups (ancestor expansion).
func GetUserRepoTeamsWithGroups(ctx context.Context, orgID, userID, repoID int64) (TeamList, error) {
	teamMap := make(map[int64]*Team)

	// Direct team_user memberships with repo access.
	var directTeams []*Team
	if err := db.GetEngine(ctx).
		Join("INNER", "team_user", "team_user.team_id = team.id").
		Join("INNER", "team_repo", "team_repo.team_id = team.id").
		Where("team.org_id = ?", orgID).
		And("team_user.uid = ?", userID).
		And("team_repo.repo_id = ?", repoID).
		Find(&directTeams); err != nil {
		return nil, err
	}
	for _, t := range directTeams {
		teamMap[t.ID] = t
	}
	if !setting.Service.EnableUserGroups {
		result := make(TeamList, 0, len(teamMap))
		for _, t := range teamMap {
			result = append(result, t)
		}
		return result, nil
	}

	// Teams accessible via user group membership.
	userGroupIDs, err := usergroup.GetUserGroupIDsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(userGroupIDs) > 0 {
		// Team assigns ancestor group G; user is in descendant G2 of G → user qualifies.
		ancestorIDs, err := usergroup.ExpandUserGroupIDsToAncestors(ctx, userGroupIDs)
		if err != nil {
			return nil, err
		}
		var groupTeams []*Team
		if err := db.GetEngine(ctx).
			Join("INNER", "team_repo", "team_repo.team_id = team.id").
			Join("INNER", "team_user_group", "team_user_group.team_id = team.id").
			Where("team.org_id = ?", orgID).
			And("team_repo.repo_id = ?", repoID).
			In("team_user_group.group_id", ancestorIDs).
			Find(&groupTeams); err != nil {
			return nil, err
		}
		for _, t := range groupTeams {
			teamMap[t.ID] = t
		}
	}

	result := make(TeamList, 0, len(teamMap))
	for _, t := range teamMap {
		result = append(result, t)
	}
	return result, nil
}

// IsUserInAnyOrgTeamViaUserGroups returns true if the user belongs to any team
// of the given org solely through a user group (i.e. no direct team_user entry required).
func IsUserInAnyOrgTeamViaUserGroups(ctx context.Context, orgID, userID int64) (bool, error) {
	if !setting.Service.EnableUserGroups {
		return false, nil
	}
	userGroupIDs, err := usergroup.GetUserGroupIDsByUser(ctx, userID)
	if err != nil || len(userGroupIDs) == 0 {
		return false, err
	}
	// Teams assign ancestor groups; a user in a descendant group also qualifies.
	// So we need to find which ancestor groups the user's groups fall under.
	ancestorIDs, err := usergroup.ExpandUserGroupIDsToAncestors(ctx, userGroupIDs)
	if err != nil {
		return false, err
	}
	return db.GetEngine(ctx).
		Table("team").
		Join("INNER", "team_user_group", "team_user_group.team_id = team.id").
		Where("team.org_id = ?", orgID).
		In("team_user_group.group_id", ancestorIDs).
		Exist()
}

func GetTeamsByOrgIDs(ctx context.Context, orgIDs []int64) (TeamList, error) {
	teams := make([]*Team, 0, 10)
	return teams, db.GetEngine(ctx).Where(builder.In("org_id", orgIDs)).Find(&teams)
}

func GetTeamsByIDs(ctx context.Context, teamIDs []int64) (map[int64]*Team, error) {
	teams := make(map[int64]*Team, len(teamIDs))
	if len(teamIDs) == 0 {
		return teams, nil
	}
	return teams, db.GetEngine(ctx).Where(builder.In("`id`", teamIDs)).Find(&teams)
}
