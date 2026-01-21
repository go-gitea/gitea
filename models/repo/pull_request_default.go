// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"
)

// ErrDefaultPRBaseBranchNotExist represents an error that branch with such name does not exist.
type ErrDefaultPRBaseBranchNotExist struct {
	RepoID     int64
	BranchName string
}

// IsErrDefaultPRBaseBranchNotExist checks if an error is an ErrDefaultPRBaseBranchNotExist.
func IsErrDefaultPRBaseBranchNotExist(err error) bool {
	_, ok := err.(ErrDefaultPRBaseBranchNotExist)
	return ok
}

func (err ErrDefaultPRBaseBranchNotExist) Error() string {
	return fmt.Sprintf("default PR base branch does not exist [repo_id: %d name: %s]", err.RepoID, err.BranchName)
}

func (err ErrDefaultPRBaseBranchNotExist) Unwrap() error {
	return util.ErrNotExist
}

// GetDefaultPRBaseBranch returns the preferred base branch for new pull requests.
// It falls back to the repository default branch when unset or invalid.
func (repo *Repository) GetDefaultPRBaseBranch(ctx context.Context) string {
	preferred := strings.TrimSpace(repo.DefaultPRBaseBranch)
	if preferred != "" {
		exists, err := isBranchNameExists(ctx, repo.ID, preferred)
		if err == nil && exists {
			return preferred
		}
	}
	return repo.DefaultBranch
}

// ValidateDefaultPRBaseBranch checks whether a preferred base branch is valid.
func (repo *Repository) ValidateDefaultPRBaseBranch(ctx context.Context, branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return nil
	}

	exists, err := isBranchNameExists(ctx, repo.ID, branch)
	if err != nil {
		return err
	}
	if !exists {
		return ErrDefaultPRBaseBranchNotExist{
			RepoID:     repo.ID,
			BranchName: branch,
		}
	}
	return nil
}

func isBranchNameExists(ctx context.Context, repoID int64, branchName string) (bool, error) {
	type branch struct {
		IsDeleted bool `xorm:"is_deleted"`
	}
	var b branch
	has, err := db.GetEngine(ctx).
		Where("repo_id = ?", repoID).
		And("name = ?", branchName).
		Get(&b)
	if err != nil {
		return false, err
	}
	if !has {
		return false, nil
	}
	return !b.IsDeleted, nil
}
