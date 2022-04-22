// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
)

func getRepositoriesByForkID(e db.Engine, forkID int64) ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	return repos, e.
		Where("fork_id=?", forkID).
		Find(&repos)
}

// GetRepositoriesByForkID returns all repositories with given fork ID.
func GetRepositoriesByForkID(ctx context.Context, forkID int64) ([]*Repository, error) {
	return getRepositoriesByForkID(db.GetEngine(ctx), forkID)
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

// GetForks returns all the forks of the repository
func GetForks(repo *Repository, listOptions db.ListOptions) ([]*Repository, error) {
	if listOptions.Page == 0 {
		forks := make([]*Repository, 0, repo.NumForks)
		return forks, db.GetEngine(db.DefaultContext).Find(&forks, &Repository{ForkID: repo.ID})
	}

	sess := db.GetPaginatedSession(&listOptions)
	forks := make([]*Repository, 0, listOptions.PageSize)
	return forks, sess.Find(&forks, &Repository{ForkID: repo.ID})
}
