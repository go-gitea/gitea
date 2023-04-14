// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"

	"xorm.io/builder"
)

// GetRepositoriesByForkID returns all repositories with given fork ID.
func GetRepositoriesByForkID(ctx context.Context, forkID int64) ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	return repos, db.GetEngine(ctx).
		Where("fork_id=?", forkID).
		Find(&repos)
}

// GetForkedRepo checks if given user has already forked a repository with given ID.
func GetForkedRepo(ownerID, repoID int64) *Repository {
	repo := new(Repository)
	has, _ := db.GetEngine(db.DefaultContext).
		Where("owner_id=? AND fork_id=?", ownerID, repoID).
		Get(repo)
	if has {
		return repo
	}
	return nil
}

// HasForkedRepo checks if given user has already forked a repository with given ID.
func HasForkedRepo(ownerID, repoID int64) bool {
	has, _ := db.GetEngine(db.DefaultContext).
		Table("repository").
		Where("owner_id=? AND fork_id=?", ownerID, repoID).
		Exist()
	return has
}

// GetUserFork return user forked repository from this repository, if not forked return nil
func GetUserFork(ctx context.Context, repoID, userID int64) (*Repository, error) {
	var forkedRepo Repository
	has, err := db.GetEngine(ctx).Where("fork_id = ?", repoID).And("owner_id = ?", userID).Get(&forkedRepo)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return &forkedRepo, nil
}

// GetForksOptions represents options for GetForks
type GetForksOptions struct {
	ListOptions db.ListOptions
	Period      string
	SortType    string
}

// GetForks returns all the forks of the repository
func GetForks(repo *Repository, forkOptions GetForksOptions) ([]*Repository, error) {
	var forks []*Repository
	var sess db.Engine

	if forkOptions.ListOptions.Page == 0 {
		forks = make([]*Repository, 0, repo.NumForks)
		sess = db.GetEngine(db.DefaultContext)
	} else {
		sess = db.GetPaginatedSession(&forkOptions.ListOptions)
		forks = make([]*Repository, 0, forkOptions.ListOptions.PageSize)
	}

	switch forkOptions.Period {
	case "1month":
		sess.Where("repository.updated_unix >= ?", time.Now().AddDate(0, -1, 0).Unix())
	case "6month":
		sess.Where("repository.updated_unix >= ?", time.Now().AddDate(0, -6, 0).Unix())
	case "1year":
		sess.Where("repository.updated_unix >= ?", time.Now().AddDate(-1, 0, 0).Unix())
	case "2year":
		sess.Where("repository.updated_unix >= ?", time.Now().AddDate(-2, 0, 0).Unix())
	case "5year":
		sess.Where("repository.updated_unix >= ?", time.Now().AddDate(-5, 0, 0).Unix())
	}

	switch forkOptions.SortType {
	case "moststars":
		sess.Desc("repository.num_stars").Desc("repository.updated_unix")
	case "recentupdated":
		sess.Desc("repository.updated_unix")
	case "leastrecentupdated":
		sess.Asc("repository.updated_unix")
	case "openissue":
		sess.Desc("repository.num_issues").Desc("repository.updated_unix")
	case "openpr":
		sess.Desc("repository.num_pulls").Desc("repository.updated_unix")
	case "newest":
		sess.Desc("repository.created_unix")
	case "oldest":
		sess.Asc("repository.created_unix")
	default:
		sess.Desc("repository.num_stars").Desc("repository.updated_unix")
	}

	return forks, sess.Find(&forks, &Repository{ForkID: repo.ID})
}

// IncrementRepoForkNum increment repository fork number
func IncrementRepoForkNum(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `repository` SET num_forks=num_forks+1 WHERE id=?", repoID)
	return err
}

// DecrementRepoForkNum decrement repository fork number
func DecrementRepoForkNum(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE `repository` SET num_forks=num_forks-1 WHERE id=?", repoID)
	return err
}

// FindUserOrgForks returns the forked repositories for one user from a repository
func FindUserOrgForks(ctx context.Context, repoID, userID int64) ([]*Repository, error) {
	cond := builder.And(
		builder.Eq{"fork_id": repoID},
		builder.In("owner_id",
			builder.Select("org_id").
				From("org_user").
				Where(builder.Eq{"uid": userID}),
		),
	)

	var repos []*Repository
	return repos, db.GetEngine(ctx).Table("repository").Where(cond).Find(&repos)
}

// GetForksByUserAndOrgs return forked repos of the user and owned orgs
func GetForksByUserAndOrgs(ctx context.Context, user *user_model.User, repo *Repository) ([]*Repository, error) {
	var repoList []*Repository
	if user == nil {
		return repoList, nil
	}
	forkedRepo, err := GetUserFork(ctx, repo.ID, user.ID)
	if err != nil {
		return repoList, err
	}
	if forkedRepo != nil {
		repoList = append(repoList, forkedRepo)
	}
	orgForks, err := FindUserOrgForks(ctx, repo.ID, user.ID)
	if err != nil {
		return nil, err
	}
	repoList = append(repoList, orgForks...)
	return repoList, nil
}
