// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// UpdateRepositoryOwnerNames updates repository owner_names (this should only be used when the ownerName has changed case)
func UpdateRepositoryOwnerNames(ctx context.Context, ownerID int64, ownerName string) error {
	if ownerID == 0 {
		return nil
	}
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if _, err := db.GetEngine(ctx).Where("owner_id = ?", ownerID).Cols("owner_name").Update(&Repository{
		OwnerName: ownerName,
	}); err != nil {
		return err
	}

	return committer.Commit()
}

// UpdateRepositoryUpdatedTime updates a repository's updated time
func UpdateRepositoryUpdatedTime(ctx context.Context, repoID int64, updateTime time.Time) error {
	_, err := db.GetEngine(ctx).Exec("UPDATE repository SET updated_unix = ? WHERE id = ?", updateTime.Unix(), repoID)
	return err
}

// UpdateRepositoryCols updates repository's columns
func UpdateRepositoryCols(ctx context.Context, repo *Repository, cols ...string) error {
	_, err := db.GetEngine(ctx).ID(repo.ID).Cols(cols...).Update(repo)
	return err
}

// UpdateRepositoryColsNoAutoTime updates repository's columns and but applies time change automatically
func UpdateRepositoryColsNoAutoTime(ctx context.Context, repo *Repository, cols ...string) error {
	_, err := db.GetEngine(ctx).ID(repo.ID).Cols(cols...).NoAutoTime().Update(repo)
	return err
}

// ErrReachLimitOfRepo represents a "ReachLimitOfRepo" kind of error.
type ErrReachLimitOfRepo struct {
	Limit int
}

// IsErrReachLimitOfRepo checks if an error is a ErrReachLimitOfRepo.
func IsErrReachLimitOfRepo(err error) bool {
	_, ok := err.(ErrReachLimitOfRepo)
	return ok
}

func (err ErrReachLimitOfRepo) Error() string {
	return fmt.Sprintf("user has reached maximum limit of repositories [limit: %d]", err.Limit)
}

func (err ErrReachLimitOfRepo) Unwrap() error {
	return util.ErrPermissionDenied
}

// ErrRepoAlreadyExist represents a "RepoAlreadyExist" kind of error.
type ErrRepoAlreadyExist struct {
	Uname string
	Name  string
}

// IsErrRepoAlreadyExist checks if an error is a ErrRepoAlreadyExist.
func IsErrRepoAlreadyExist(err error) bool {
	_, ok := err.(ErrRepoAlreadyExist)
	return ok
}

func (err ErrRepoAlreadyExist) Error() string {
	return fmt.Sprintf("repository already exists [uname: %s, name: %s]", err.Uname, err.Name)
}

func (err ErrRepoAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrRepoFilesAlreadyExist represents a "RepoFilesAlreadyExist" kind of error.
type ErrRepoFilesAlreadyExist struct {
	Uname string
	Name  string
}

// IsErrRepoFilesAlreadyExist checks if an error is a ErrRepoAlreadyExist.
func IsErrRepoFilesAlreadyExist(err error) bool {
	_, ok := err.(ErrRepoFilesAlreadyExist)
	return ok
}

func (err ErrRepoFilesAlreadyExist) Error() string {
	return fmt.Sprintf("repository files already exist [uname: %s, name: %s]", err.Uname, err.Name)
}

func (err ErrRepoFilesAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// CheckCreateRepository check if could created a repository
func CheckCreateRepository(ctx context.Context, doer, u *user_model.User, name string, overwriteOrAdopt bool) error {
	if !doer.CanCreateRepo() {
		return ErrReachLimitOfRepo{u.MaxRepoCreation}
	}

	if err := IsUsableRepoName(name); err != nil {
		return err
	}

	has, err := IsRepositoryModelOrDirExist(ctx, u, name)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %w", err)
	} else if has {
		return ErrRepoAlreadyExist{u.Name, name}
	}

	repoPath := RepoPath(u.Name, name)
	isExist, err := util.IsExist(repoPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", repoPath, err)
		return err
	}
	if !overwriteOrAdopt && isExist {
		return ErrRepoFilesAlreadyExist{u.Name, name}
	}
	return nil
}

// UpdateRepoSize updates the repository size, calculating it using getDirectorySize
func UpdateRepoSize(ctx context.Context, repoID, gitSize, lfsSize int64) error {
	_, err := db.GetEngine(ctx).ID(repoID).Cols("size", "git_size", "lfs_size").NoAutoTime().Update(&Repository{
		Size:    gitSize + lfsSize,
		GitSize: gitSize,
		LFSSize: lfsSize,
	})
	return err
}
