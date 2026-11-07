// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"errors"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"

	"xorm.io/builder"
)

// ShareForkTree reports whether both repositories descend from the same root repository.
func ShareForkTree(ctx context.Context, first, second *Repository) (bool, error) {
	rootID := func(repo *Repository) (int64, error) {
		visited := map[int64]struct{}{}
		for repo != nil {
			if _, ok := visited[repo.ID]; ok {
				return 0, errors.New("repository fork tree contains a cycle")
			}
			visited[repo.ID] = struct{}{}
			if !repo.IsFork {
				return repo.ID, nil
			}
			var err error
			repo, err = GetRepositoryByID(ctx, repo.ForkID)
			if err != nil {
				return 0, err
			}
		}
		return 0, errors.New("repository is required")
	}

	firstRootID, err := rootID(first)
	if err != nil {
		return false, err
	}
	secondRootID, err := rootID(second)
	return firstRootID == secondRootID, err
}

// GetRepositoriesByForkID returns all repositories with given fork ID.
func GetRepositoriesByForkID(ctx context.Context, forkID int64) ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	return repos, db.GetEngine(ctx).
		Where("fork_id=?", forkID).
		Find(&repos)
}

// GetForkedRepo checks if given user has already forked a repository with given ID.
func GetForkedRepo(ctx context.Context, ownerID, repoID int64) *Repository {
	repo := new(Repository)
	has, _ := db.GetEngine(ctx).
		Where("owner_id=? AND fork_id=?", ownerID, repoID).
		Get(repo)
	if has {
		return repo
	}
	return nil
}

// HasForkedRepo checks if given user has already forked a repository with given ID.
func HasForkedRepo(ctx context.Context, ownerID, repoID int64) bool {
	has, _ := db.GetEngine(ctx).
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
		return nil, nil //nolint:nilnil // return nil to indicate that the object does not exist
	}
	return &forkedRepo, nil
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
