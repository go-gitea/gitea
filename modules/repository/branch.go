// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
)

// SyncBranches synchronizes branch table with repository branches
func SyncBranches(ctx context.Context, repo *repo_model.Repository, doerID int64, gitRepo *git.Repository) error {
	log.Debug("SyncBranches: in Repo[%d:%s/%s]", repo.ID, repo.OwnerName, repo.Name)

	const limit = 100
	var allBranches []string
	for page := 1; ; page++ {
		branches, _, err := gitRepo.GetBranchNames(page*limit, limit)
		if err != nil {
			return err
		}
		if len(branches) == 0 {
			break
		}
		allBranches = append(allBranches, branches...)
	}

	dbBranches, err := git_model.LoadAllBranches(ctx, repo.ID)
	if err != nil {
		return err
	}

	var toAdd []*git_model.Branch
	var toRemove []int64
	for _, branch := range allBranches {
		var found bool
		for _, dbBranch := range dbBranches {
			if branch == dbBranch.Name {
				found = true
				break
			}
		}
		if !found {
			commit, err := gitRepo.GetBranchCommit(branch)
			if err != nil {
				return err
			}
			toAdd = append(toAdd, &git_model.Branch{
				RepoID:     repo.ID,
				Name:       branch,
				Commit:     commit.ID.String(),
				PusherID:   doerID,
				CommitTime: timeutil.TimeStamp(commit.Author.When.Unix()),
			})
		}
	}

	for _, dbBranch := range dbBranches {
		var found bool
		for _, branch := range allBranches {
			if branch == dbBranch.Name {
				found = true
				break
			}
		}
		if !found {
			toRemove = append(toRemove, dbBranch.ID)
		}
	}

	if len(toAdd) <= 0 && len(toRemove) <= 0 {
		return nil
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		if len(toAdd) > 0 {
			err = db.Insert(ctx, toAdd)
			if err != nil {
				return err
			}
		}

		if len(toRemove) > 0 {
			err = git_model.DeleteBranches(ctx, repo.ID, doerID, toRemove)
			if err != nil {
				return err
			}
		}

		return nil
	})
}
