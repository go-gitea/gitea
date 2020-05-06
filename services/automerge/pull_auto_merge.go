// Copyright 2019 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package automerge

import (
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
	// First, get the branch associated with that commit sha
	r, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return
	}
	defer r.Close()
	commitID := git.MustIDFromString(sha)
	tree := git.NewTree(r, commitID)
	commit := &git.Commit{
		Tree: *tree,
		ID:   commitID,
	}
	branches, err := commit.GetBranchNames()
	if err != nil {
		return
	}

	for _, branch := range branches {
		// We get the branch name with a \n at the end which is not in the db so we strip it out
		branch = strings.Trim(branch, "\n")
		// Then get all prs for that branch
		pr, err := models.GetPullRequestByHeadBranch(branch, repo)
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

		// FIXME: Is headGitRepo the right thing to use here? Maybe we should get the git repo based on scheduledPRM.RepoID?

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
