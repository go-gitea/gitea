// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

func synchronizeRepoHeads(ctx context.Context, logger log.Logger, autofix bool) error {
	numRepos := 0
	numHeadsBroken := 0
	numDefaultBranchesBroken := 0
	numReposUpdated := 0
	err := iterateRepositories(ctx, func(repo *repo_model.Repository) error {
		numRepos++
		_, _, defaultBranchErr := git.NewCommand("rev-parse").AddDashesAndList(repo.DefaultBranch).RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath()})

		head, _, headErr := git.NewCommand("symbolic-ref", "--short", "HEAD").RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath()})

		// what we expect: default branch is valid, and HEAD points to it
		if headErr == nil && defaultBranchErr == nil && head == repo.DefaultBranch {
			return nil
		}

		if headErr != nil {
			numHeadsBroken++
		}
		if defaultBranchErr != nil {
			numDefaultBranchesBroken++
		}

		// if default branch is broken, let the user fix that in the UI
		if defaultBranchErr != nil {
			logger.Warn("Default branch for %s/%s doesn't point to a valid commit", repo.OwnerName, repo.Name)
			return nil
		}

		// if we're not autofixing, that's all we can do
		if !autofix {
			return nil
		}

		// otherwise, let's try fixing HEAD
		err := git.NewCommand("symbolic-ref").AddDashesAndList("HEAD", git.BranchPrefix+repo.DefaultBranch).Run(ctx, &git.RunOpts{Dir: repo.RepoPath()})
		if err != nil {
			logger.Warn("Failed to fix HEAD for %s/%s: %v", repo.OwnerName, repo.Name, err)
			return nil
		}
		numReposUpdated++
		return nil
	})
	if err != nil {
		logger.Critical("Error when fixing repo HEADs: %v", err)
	}

	if autofix {
		logger.Info("Out of %d repos, HEADs for %d are now fixed and HEADS for %d are still broken", numRepos, numReposUpdated, numDefaultBranchesBroken+numHeadsBroken-numReposUpdated)
	} else {
		if numHeadsBroken == 0 && numDefaultBranchesBroken == 0 {
			logger.Info("All %d repos have their HEADs in the correct state", numRepos)
		} else {
			if numHeadsBroken == 0 && numDefaultBranchesBroken != 0 {
				logger.Critical("Default branches are broken for %d/%d repos", numDefaultBranchesBroken, numRepos)
			} else if numHeadsBroken != 0 && numDefaultBranchesBroken == 0 {
				logger.Warn("HEADs are broken for %d/%d repos", numHeadsBroken, numRepos)
			} else {
				logger.Critical("Out of %d repos, HEADS are broken for %d and default branches are broken for %d", numRepos, numHeadsBroken, numDefaultBranchesBroken)
			}
		}
	}

	return err
}

func init() {
	Register(&Check{
		Title:     "Synchronize repo HEADs",
		Name:      "synchronize-repo-heads",
		IsDefault: true,
		Run:       synchronizeRepoHeads,
		Priority:  7,
	})
}
