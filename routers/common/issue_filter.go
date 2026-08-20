// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"context"

	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"
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
	Doer       *user_model.User
	PublicOnly bool
	OwnerName  string
	TeamName   string
}

// SearchIssuesRepoIDs resolves the repository filter of an issue search. allPublic makes the indexer
// match everything its own is_public covers (modules/indexer/issues/util.go), so repoIDs omits those.
func SearchIssuesRepoIDs(ctx context.Context, opts SearchIssuesRepoIDsOptions) (repoIDs []int64, allPublic bool, err error) {
	searchOpts := repo_model.SearchRepoOptions{
		Private:     opts.Doer != nil,
		Collaborate: optional.None[bool](),
		Actor:       opts.Doer,
	}
	searchOpts.ApplyPublicOnly(opts.PublicOnly)
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
	cond := repo_model.SearchRepositoryCondition(searchOpts)
	if allPublic {
		if !searchOpts.Private {
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
