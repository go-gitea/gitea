// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"

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

// AccessibleReposEnvironment operations involving the repositories that are
// accessible to a particular user
type AccessibleReposEnvironment interface {
	CountRepos(ctx context.Context) (int64, error)
	RepoIDs(ctx context.Context, page, pageSize int) ([]int64, error)
	Repos(ctx context.Context, page, pageSize int) (RepositoryList, error)
	MirrorRepos(ctx context.Context) (RepositoryList, error)
	AddKeyword(keyword string)
	SetSort(db.SearchOrderBy)
}

type accessibleReposEnv struct {
	org     *org_model.Organization
	user    *user_model.User
	team    *org_model.Team
	teamIDs []int64
	keyword string
	orderBy db.SearchOrderBy
}

// AccessibleReposEnv builds an AccessibleReposEnvironment for the repositories in `org`
// that are accessible to the specified user.
func AccessibleReposEnv(ctx context.Context, org *org_model.Organization, userID int64) (AccessibleReposEnvironment, error) {
	var user *user_model.User

	if userID > 0 {
		u, err := user_model.GetUserByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		user = u
	}

	teamIDs, err := org.GetUserTeamIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &accessibleReposEnv{
		org:     org,
		user:    user,
		teamIDs: teamIDs,
		orderBy: db.SearchOrderByRecentUpdated,
	}, nil
}

// AccessibleTeamReposEnv an AccessibleReposEnvironment for the repositories in `org`
// that are accessible to the specified team.
func AccessibleTeamReposEnv(org *org_model.Organization, team *org_model.Team) AccessibleReposEnvironment {
	return &accessibleReposEnv{
		org:     org,
		team:    team,
		orderBy: db.SearchOrderByRecentUpdated,
	}
}

func (env *accessibleReposEnv) cond() builder.Cond {
	cond := builder.NewCond()
	if env.team != nil {
		cond = cond.And(builder.Eq{"team_repo.team_id": env.team.ID})
	} else {
		if env.user == nil || !env.user.IsRestricted {
			cond = cond.Or(builder.Eq{
				"`repository`.owner_id":   env.org.ID,
				"`repository`.is_private": false,
			})
		}
		if len(env.teamIDs) > 0 {
			cond = cond.Or(builder.In("team_repo.team_id", env.teamIDs))
		}
	}
	if env.keyword != "" {
		cond = cond.And(builder.Like{"`repository`.lower_name", strings.ToLower(env.keyword)})
	}
	return cond
}

func (env *accessibleReposEnv) CountRepos(ctx context.Context) (int64, error) {
	repoCount, err := db.GetEngine(ctx).
		Join("INNER", "team_repo", "`team_repo`.repo_id=`repository`.id").
		Where(env.cond()).
		Distinct("`repository`.id").
		Count(&Repository{})
	if err != nil {
		return 0, fmt.Errorf("count user repositories in organization: %w", err)
	}
	return repoCount, nil
}

func (env *accessibleReposEnv) RepoIDs(ctx context.Context, page, pageSize int) ([]int64, error) {
	if page <= 0 {
		page = 1
	}

	repoIDs := make([]int64, 0, pageSize)
	return repoIDs, db.GetEngine(ctx).
		Table("repository").
		Join("INNER", "team_repo", "`team_repo`.repo_id=`repository`.id").
		Where(env.cond()).
		GroupBy("`repository`.id,`repository`."+strings.Fields(string(env.orderBy))[0]).
		OrderBy(string(env.orderBy)).
		Limit(pageSize, (page-1)*pageSize).
		Cols("`repository`.id").
		Find(&repoIDs)
}

func (env *accessibleReposEnv) Repos(ctx context.Context, page, pageSize int) (RepositoryList, error) {
	repoIDs, err := env.RepoIDs(ctx, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("GetUserRepositoryIDs: %w", err)
	}

	repos := make([]*Repository, 0, len(repoIDs))
	if len(repoIDs) == 0 {
		return repos, nil
	}

	return repos, db.GetEngine(ctx).
		In("`repository`.id", repoIDs).
		OrderBy(string(env.orderBy)).
		Find(&repos)
}

func (env *accessibleReposEnv) MirrorRepoIDs(ctx context.Context) ([]int64, error) {
	repoIDs := make([]int64, 0, 10)
	return repoIDs, db.GetEngine(ctx).
		Table("repository").
		Join("INNER", "team_repo", "`team_repo`.repo_id=`repository`.id AND `repository`.is_mirror=?", true).
		Where(env.cond()).
		GroupBy("`repository`.id, `repository`.updated_unix").
		OrderBy(string(env.orderBy)).
		Cols("`repository`.id").
		Find(&repoIDs)
}

func (env *accessibleReposEnv) MirrorRepos(ctx context.Context) (RepositoryList, error) {
	repoIDs, err := env.MirrorRepoIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("MirrorRepoIDs: %w", err)
	}

	repos := make([]*Repository, 0, len(repoIDs))
	if len(repoIDs) == 0 {
		return repos, nil
	}

	return repos, db.GetEngine(ctx).
		In("`repository`.id", repoIDs).
		Find(&repos)
}

func (env *accessibleReposEnv) AddKeyword(keyword string) {
	env.keyword = keyword
}

func (env *accessibleReposEnv) SetSort(orderBy db.SearchOrderBy) {
	env.orderBy = orderBy
}
