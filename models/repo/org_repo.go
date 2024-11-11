// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"xorm.io/builder"
)

// GetOrgRepositories get repos belonging to the given organization
func GetOrgRepositories(ctx context.Context, orgID int64) (RepositoryList, error) {
	var orgRepos []*Repository
	return orgRepos, db.GetEngine(ctx).Where("owner_id = ?", orgID).Find(&orgRepos)
}

type SearchTeamRepoOptions struct {
	db.ListOptions
	TeamID int64
}

// GetRepositories returns paginated repositories in team of organization.
func GetTeamRepositories(ctx context.Context, opts *SearchTeamRepoOptions) (RepositoryList, error) {
	sess := db.GetEngine(ctx)
	if opts.TeamID > 0 {
		sess = sess.In("id",
			builder.Select("repo_id").
				From("team_repo").
				Where(builder.Eq{"team_id": opts.TeamID}),
		)
	}
	if opts.PageSize > 0 {
		sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	var repos []*Repository
	return repos, sess.OrderBy("repository.name").
		Find(&repos)
}
