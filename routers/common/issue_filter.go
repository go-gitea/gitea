// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"context"

	"gitea.dev/models/db"
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
	IsSigned   bool
	PublicOnly bool
	OwnerName  string
	TeamName   string
}

// SearchIssuesRepoIDs resolves the repository filter of an issue search.
// allPublic means "plus every public repository", which the indexer matches itself.
func SearchIssuesRepoIDs(ctx context.Context, o SearchIssuesRepoIDsOptions) (repoIDs []int64, allPublic bool, err error) {
	opts := repo_model.SearchRepoOptions{
		Private:     false,
		AllPublic:   true,
		TopicOnly:   false,
		Collaborate: optional.None[bool](),
		// This needs to be a column that is not nil in fixtures or
		// MySQL will return different results when sorting by null in some cases
		OrderBy: db.SearchOrderByAlphabetically,
		Actor:   o.Doer,
	}
	if o.IsSigned {
		opts.Private = true
		opts.AllLimited = true
	}
	opts.ApplyPublicOnly(o.PublicOnly)
	if o.OwnerName != "" {
		owner, err := user_model.GetUserByName(ctx, o.OwnerName)
		if err != nil {
			return nil, false, err
		}
		opts.OwnerID = owner.ID
		opts.AllLimited = false
		opts.AllPublic = false
		opts.Collaborate = optional.Some(false)
	}
	if o.TeamName != "" {
		if o.OwnerName == "" {
			return nil, false, util.NewInvalidArgumentErrorf("owner organisation is required for filtering on team")
		}
		team, err := organization.GetTeam(ctx, opts.OwnerID, o.TeamName)
		if err != nil {
			return nil, false, err
		}
		opts.TeamID = team.ID
	}

	if opts.AllPublic {
		allPublic = true
		opts.AllPublic = false               // set it false to avoid returning too many repos, we could filter by indexer
		opts.IsPrivate = optional.Some(true) // enumerating public repos too would scale the ID list with the instance
	}
	repoIDs, _, err = repo_model.SearchRepositoryIDs(ctx, opts)
	if err != nil {
		return nil, false, err
	}
	if len(repoIDs) == 0 {
		// no repos found, don't let the indexer return all repos
		repoIDs = []int64{0}
	}

	return repoIDs, allPublic, nil
}
