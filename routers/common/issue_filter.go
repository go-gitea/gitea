// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"context"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

func ParseIssueFilterStateIsClosed(state string) optional.Option[bool] {
	switch state {
	case "all":
		return optional.None[bool]()
	case "closed":
		return optional.Some(true)
	case "", "open":
		return optional.Some(false)
	default:
		return optional.Some(false) // unknown state, undefined behavior
	}
}

func ParseIssueFilterTypeIsPull(typ string) optional.Option[bool] {
	return optional.FromMapLookup(map[string]bool{"pulls": true, "issues": false}, typ)
}

type SearchIssuesRepoIDsOptions struct {
	Doer      *user_model.User
	OwnerName string
	TeamName  string
	IsPull    optional.Option[bool] // None searches both issues and pull requests
}

// SearchIssuesRepoIDs resolves the repository filter of an issue search. allPublic makes the indexer
// match everything its own is_public covers (modules/indexer/issues/util.go), so repoIDs omits those.
func SearchIssuesRepoIDs(ctx context.Context, opts SearchIssuesRepoIDsOptions) (repoIDs []int64, allPublic bool, err error) {
	searchOpts := repo_model.SearchRepoOptions{
		Private:     opts.Doer != nil,
		Collaborate: optional.None[bool](),
		Actor:       opts.Doer,
	}
	if opts.OwnerName != "" {
		owner, err := user_model.GetUserByName(ctx, opts.OwnerName)
		if err != nil {
			return nil, false, err
		}
		searchOpts.OwnerID = owner.ID
		searchOpts.Collaborate = optional.Some(false)
	}
	if opts.TeamName != "" {
		if opts.OwnerName == "" {
			return nil, false, util.NewInvalidArgumentErrorf("owner organisation is required for filtering on team")
		}
		team, err := organization.GetTeam(ctx, searchOpts.OwnerID, opts.TeamName)
		if err != nil {
			return nil, false, err
		}
		searchOpts.TeamID = team.ID
	}

	// SearchRepoOptions.AllPublic and AllLimited only apply under an owner filter, so the indexer covers them
	allPublic = opts.OwnerName == ""
	units := []unit.Type{unit.TypeIssues, unit.TypePullRequests}
	if opts.IsPull.Has() {
		units = []unit.Type{util.Iif(opts.IsPull.Value(), unit.TypePullRequests, unit.TypeIssues)}
	}
	// the indexer takes one repository filter for both kinds, so an untyped search needs both units readable
	searchOpts.UnitType = units[0]
	cond := repo_model.SearchRepositoryCondition(searchOpts)
	if opts.Doer != nil && !opts.Doer.IsAdmin {
		for _, u := range units[1:] {
			cond = cond.And(repo_model.AccessibleRepositoryCondition(opts.Doer, u))
		}
	}
	cond = cond.And(builder.In("`repository`.id", builder.Select("repo_id").From("repo_unit").Where(builder.In("type", units))))
	if allPublic {
		if !user_model.DoerCanViewPrivate(opts.Doer) {
			return []int64{0}, allPublic, nil // sees nothing beyond is_public, so skip the query
		}
		cond = cond.And(repo_model.NotPublicRepoUnderPublicOwnerCond()) // enumerating them scales with the instance
	}
	repoIDs, err = repo_model.SearchRepositoryIDsByCondition(ctx, cond)
	if err != nil {
		return nil, false, err
	}
	if len(repoIDs) == 0 {
		// no repos found, don't let the indexer return all repos
		repoIDs = []int64{0}
	}

	return repoIDs, allPublic, nil
}

// FilterReadableIssues drops the issues and pulls whose unit the doer cannot read. Only the public repositories
// the indexer matches by itself need it, the repoIDs from SearchIssuesRepoIDs are already filtered.
func FilterReadableIssues(ctx context.Context, doer *user_model.User, issues issues_model.IssueList, repoIDs []int64) (issues_model.IssueList, error) {
	if _, err := issues.LoadRepositories(ctx); err != nil {
		return nil, err
	}
	checked := container.SetOf(repoIDs...)
	perms := make(map[int64]access_model.Permission)
	readable := make(issues_model.IssueList, 0, len(issues))
	for _, issue := range issues {
		if checked.Contains(issue.RepoID) {
			readable = append(readable, issue)
			continue
		}
		perm, ok := perms[issue.RepoID]
		if !ok {
			var err error
			if perm, err = access_model.GetDoerRepoPermission(ctx, issue.Repo, doer); err != nil {
				return nil, err
			}
			perms[issue.RepoID] = perm
		}
		if perm.CanReadIssuesOrPulls(issue.IsPull) {
			readable = append(readable, issue)
		}
	}
	return readable, nil
}
