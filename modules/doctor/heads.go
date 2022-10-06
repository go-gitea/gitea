// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
		runOpts := &git.RunOpts{Dir: repo.RepoPath()}

		head, _, headErr := git.NewCommand(ctx, "rev-parse", "HEAD").RunStdString(runOpts)
		defaultBranch, _, defaultBranchErr := git.NewCommand(ctx, "rev-parse", repo.DefaultBranch).RunStdString(runOpts)

		// what we expect: both HEAD and default branch point to the same commit
		if headErr == nil && defaultBranchErr == nil && head == defaultBranch {
			return nil
		}

		if headErr != nil {
			numHeadsBroken++
		}
		if defaultBranchErr != nil {
			numDefaultBranchesBroken++
		}

		// absolute failure: both HEAD and default branch point to invalid commits
		if headErr != nil && defaultBranchErr != nil {
			logger.Critical("Neither HEAD nor the default branch for %s/%s point to a valid commit", repo.OwnerName, repo.Name)
			return nil
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
		err := git.NewCommand(ctx, "switch", repo.DefaultBranch).Run(runOpts)
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
			logger.Info("All %d repos have their HEADs in the correct state")
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
