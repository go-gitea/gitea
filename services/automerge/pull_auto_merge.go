// Copyright 2019 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package automerge

import (
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	pullservice "code.gitea.io/gitea/services/pull"
)

// This package merges a previously scheduled pull request on successful status check.
// It is a separate package to avoid cyclic dependencies.

// MergeScheduledPullRequest merges a previously scheduled pull request when all checks succeeded
// Maybe FIXME: Move the whole check this function does into a separate go routine and just ping from here?
func MergeScheduledPullRequest(sha string, repo *models.Repository) (err error) {
	branches, err := git.GetBranchNamesForSha(sha, repo.RepoPath())
	if err != nil {
		return
	}

	for _, branch := range branches {
		// We get the branch name with a \n at the end which is not in the db so we strip it out
		branch = strings.Trim(branch, "\n")
		// Then get all prs for that branch

		// If the branch starts with "pull/*" we know we're dealing with a fork.
		// In that case, head and base branch are not in the same repo and we need to do some extra work
		// to get the pull request for this branch.
		// Each pull branch starts with refs/pull/ we then go from there to find the index of the pr and then
		// use that to get the pr.
		var pr *models.PullRequest
		err = nil // Could be filled with an error from an earlier run
		if strings.HasPrefix(branch, "refs/pull/") {

			parts := strings.Split(branch, "/")

			prIndex, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				return err
			}

			pr, err = models.GetPullRequestByIndex(repo.ID, prIndex)
		} else {
			pr, err = models.GetPullRequestByHeadBranch(branch, repo)
		}
		if err != nil {
			// If there is no pull request for this branch, we don't try to merge it.
			if models.IsErrPullRequestNotExist(err) {
				continue
			}
			return err
		}
		if pr.HasMerged {
			log.Info("PR scheduled for auto merge is already merged [ID: %d", pr.ID)
			return nil
		}

		// Check if there is a scheduled pr in the db
		exists, scheduledPRM, err := models.GetScheduledMergeRequestByPullID(pr.ID)
		if err != nil {
			return err
		}
		if !exists {
			log.Info("No scheduled pull request merge exists for this pr [PRID: %d]", pr.ID)
			return nil
		}

		// Get all checks for this pr
		// We get the latest sha commit hash again to handle the case where the check of a previous push
		// did not succeed or was not finished yet.

		if err = pr.LoadHeadRepo(); err != nil {
			return err
		}

		headGitRepo, err := git.OpenRepository(pr.HeadRepo.RepoPath())
		if err != nil {
			return err
		}
		defer headGitRepo.Close()

		headBranchExist := headGitRepo.IsBranchExist(pr.HeadBranch)

		if pr.HeadRepo == nil || !headBranchExist {
			log.Info("Head branch of auto merge pr does not exist [HeadRepoID: %d, Branch: %s, PRID: %d]", pr.HeadRepoID, pr.HeadBranch, pr.ID)
			return nil
		}

		// Check if all checks succeeded
		pass, err := pullservice.IsPullCommitStatusPass(pr)
		if err != nil {
			return err
		}
		if !pass {
			log.Info("Scheduled auto merge pr has unsuccessful status checks [PRID: %d, Commit: %s]", pr.ID, sha)
			return nil
		}

		// Merge if all checks succeeded
		doer, err := models.GetUserByID(scheduledPRM.UserID)
		if err != nil {
			return err
		}

		if err = pr.LoadBaseRepo(); err != nil {
			return err
		}

		baseGitRepo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
		if err != nil {
			return err
		}
		defer baseGitRepo.Close()

		if err = pullservice.Merge(pr, doer, baseGitRepo, scheduledPRM.MergeStyle, scheduledPRM.Message); err != nil {
			return err
		}

		// Remove the schedule from the db
		if err = models.RemoveScheduledMergeRequest(scheduledPRM); err != nil {
			return err
		}
	}
	return nil
}
