// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"

	"xorm.io/builder"
)

func iteratePRs(ctx context.Context, repo *repo_model.Repository, each func(*repo_model.Repository, *issues_model.PullRequest) error) error {
	return db.Iterate(
		ctx,
		builder.Eq{"base_repo_id": repo.ID},
		func(ctx context.Context, bean *issues_model.PullRequest) error {
			return each(repo, bean)
		},
	)
}

func checkPRMergeBase(ctx context.Context, logger log.Logger, autofix bool) error {
	numRepos := 0
	numPRs := 0
	numPRsUpdated := 0
	err := iterateRepositories(ctx, func(repo *repo_model.Repository) error {
		numRepos++
		return iteratePRs(ctx, repo, func(repo *repo_model.Repository, pr *issues_model.PullRequest) error {
			numPRs++
			pr.BaseRepo = repo
			repoPath := repo.RepoPath()

			oldMergeBase := pr.MergeBase

			if !pr.HasMerged {
				var err error
				pr.MergeBase, _, err = git.NewCommand("merge-base").AddDashesAndList(pr.BaseBranch, pr.GetGitRefName()).RunStdString(ctx, &git.RunOpts{Dir: repoPath})
				if err != nil {
					var err2 error
					pr.MergeBase, _, err2 = git.NewCommand("rev-parse").AddDynamicArguments(git.BranchPrefix+pr.BaseBranch).RunStdString(ctx, &git.RunOpts{Dir: repoPath})
					if err2 != nil {
						logger.Warn("Unable to get merge base for PR ID %d, #%d onto %s in %s/%s. Error: %v & %v", pr.ID, pr.Index, pr.BaseBranch, pr.BaseRepo.OwnerName, pr.BaseRepo.Name, err, err2)
						return nil
					}
				}
			} else {
				parentsString, _, err := git.NewCommand("rev-list", "--parents", "-n", "1").AddDynamicArguments(pr.MergedCommitID).RunStdString(ctx, &git.RunOpts{Dir: repoPath})
				if err != nil {
					logger.Warn("Unable to get parents for merged PR ID %d, #%d onto %s in %s/%s. Error: %v", pr.ID, pr.Index, pr.BaseBranch, pr.BaseRepo.OwnerName, pr.BaseRepo.Name, err)
					return nil
				}
				parents := strings.Split(strings.TrimSpace(parentsString), " ")
				if len(parents) < 2 {
					return nil
				}

				refs := append([]string{}, parents[1:]...)
				refs = append(refs, pr.GetGitRefName())
				cmd := git.NewCommand("merge-base").AddDashesAndList(refs...)
				pr.MergeBase, _, err = cmd.RunStdString(ctx, &git.RunOpts{Dir: repoPath})
				if err != nil {
					logger.Warn("Unable to get merge base for merged PR ID %d, #%d onto %s in %s/%s. Error: %v", pr.ID, pr.Index, pr.BaseBranch, pr.BaseRepo.OwnerName, pr.BaseRepo.Name, err)
					return nil
				}
			}
			pr.MergeBase = strings.TrimSpace(pr.MergeBase)
			if pr.MergeBase != oldMergeBase {
				if autofix {
					if err := pr.UpdateCols(ctx, "merge_base"); err != nil {
						logger.Critical("Failed to update merge_base. ERROR: %v", err)
						return fmt.Errorf("Failed to update merge_base. ERROR: %w", err)
					}
				} else {
					logger.Info("#%d onto %s in %s/%s: MergeBase should be %s but is %s", pr.Index, pr.BaseBranch, pr.BaseRepo.OwnerName, pr.BaseRepo.Name, oldMergeBase, pr.MergeBase)
				}
				numPRsUpdated++
			}
			return nil
		})
	})

	if autofix {
		logger.Info("%d PR mergebases updated of %d PRs total in %d repos", numPRsUpdated, numPRs, numRepos)
	} else {
		if numPRsUpdated == 0 {
			logger.Info("All %d PRs in %d repos have a correct mergebase", numPRs, numRepos)
		} else if err == nil {
			logger.Critical("%d PRs with incorrect mergebases of %d PRs total in %d repos", numPRsUpdated, numPRs, numRepos)
			return fmt.Errorf("%d PRs with incorrect mergebases of %d PRs total in %d repos", numPRsUpdated, numPRs, numRepos)
		} else {
			logger.Warn("%d PRs with incorrect mergebases of %d PRs total in %d repos", numPRsUpdated, numPRs, numRepos)
		}
	}

	return err
}

func init() {
	Register(&Check{
		Title:     "Recalculate merge bases",
		Name:      "recalculate-merge-bases",
		IsDefault: false,
		Run:       checkPRMergeBase,
		Priority:  7,
	})
}
