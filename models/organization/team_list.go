// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"

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
	// IncludeVisibilities, when combined with UserID, also returns teams whose
	// visibility is in this list, even if UserID is not a member. Typical values:
	//   - {"limited","public"} for org members
	//   - {"public"} for signed-in users who are not org members
	// Leave empty to return only teams the user is a member of.
	IncludeVisibilities []string
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

	switch {
	case opts.UserID > 0 && len(opts.IncludeVisibilities) > 0:
		cond = cond.And(builder.Or(
			builder.Eq{"team_user.uid": opts.UserID},
			builder.In("`team`.visibility", opts.IncludeVisibilities),
		))
	case opts.UserID > 0:
		cond = cond.And(builder.Eq{"team_user.uid": opts.UserID})
	case len(opts.IncludeVisibilities) > 0:
		cond = cond.And(builder.In("`team`.visibility", opts.IncludeVisibilities))
	}

	return cond
}

// VisibleTeamVisibilitiesFor returns the visibility tiers a viewer is entitled
// to list in addition to teams they are a direct member of. Pass true for
// isOrgMember when the viewer belongs to the parent organization; otherwise
// pass true for isSignedIn for any other authenticated user. Returns nil for
// anonymous viewers (caller should then skip the search entirely or pass the
// result to IncludeVisibilities as-is).
func VisibleTeamVisibilitiesFor(isOrgMember, isSignedIn bool) []string {
	switch {
	case isOrgMember:
		return []string{TeamVisibilityLimited, TeamVisibilityPublic}
	case isSignedIn:
		return []string{TeamVisibilityPublic}
	default:
		return nil
	}
}

// SearchTeam search for teams. Caller is responsible to check permissions.
func SearchTeam(ctx context.Context, opts *SearchTeamOptions) (TeamList, int64, error) {
	sess := db.GetEngine(ctx)

	opts.SetDefaultValues()
	cond := opts.toCond()

	if opts.UserID > 0 {
		if len(opts.IncludeVisibilities) > 0 {
			sess = sess.Join("LEFT", "team_user", "team_user.team_id = team.id AND team_user.uid = ?", opts.UserID)
		} else {
			sess = sess.Join("INNER", "team_user", "team_user.team_id = team.id")
		}
	}
	// When UserID is zero but IncludeVisibilities is set, no team_user join is
	// needed — the WHERE clause already restricts to the requested tier(s).
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
	if len(teamIDs) == 0 {
		return teams, nil
	}
	return teams, db.GetEngine(ctx).Where(builder.In("`id`", teamIDs)).Find(&teams)
}
