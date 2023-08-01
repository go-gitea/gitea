// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"

	"github.com/gobwas/glob"
	"github.com/pkg/errors"
)

// MergeRequiredContextsCommitStatus returns a commit status state for given required contexts
func MergeRequiredContextsCommitStatus(commitStatuses []*git_model.CommitStatus, commitStatus *git_model.CommitStatus, requiredContexts []string) structs.CommitStatusState {
	// matchedCount is the number of `CommitStatus.Context` that match any context of `requiredContexts`
	matchedCount := 0
	returnedStatus := structs.CommitStatusSuccess

	if len(requiredContexts) > 0 {
		requiredContextsGlob := make(map[string]glob.Glob, len(requiredContexts))
		for _, ctx := range requiredContexts {
			if gp, err := glob.Compile(ctx); err != nil {
				log.Error("glob.Compile %s failed. Error: %v", ctx, err)
			} else {
				requiredContextsGlob[ctx] = gp
			}
		}

		for _, commitStatus := range commitStatuses {
			var targetStatus structs.CommitStatusState
			for _, gp := range requiredContextsGlob {
				if gp.Match(commitStatus.Context) {
					targetStatus = commitStatus.State
					matchedCount++
					break
				}
			}

			if targetStatus.IsValid() && targetStatus.NoBetterThan(returnedStatus) {
				returnedStatus = targetStatus
			}
		}
	}

	if matchedCount == 0 {
		if commitStatus != nil {
			return commitStatus.State
		}
		return structs.CommitStatusSuccess
	}

	return returnedStatus
}

// IsPullCommitStatusPass returns if all required status checks PASS
func IsPullCommitStatusPass(ctx context.Context, pr *issues_model.PullRequest) (bool, error) {
	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		return false, errors.Wrap(err, "GetFirstMatchProtectedBranchRule")
	}
	if pb == nil || !pb.EnableStatusCheck {
		return true, nil
	}

	state, err := GetPullRequestCommitStatusState(ctx, pr)
	if err != nil {
		return false, err
	}
	return state.IsSuccess(), nil
}

// GetPullRequestCommitStatusState returns pull request merged commit status state
func GetPullRequestCommitStatusState(ctx context.Context, pr *issues_model.PullRequest) (structs.CommitStatusState, error) {
	// Ensure HeadRepo is loaded
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return 0, errors.Wrap(err, "LoadHeadRepo")
	}

	// check if all required status checks are successful
	headGitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, pr.HeadRepo.RepoPath())
	if err != nil {
		return 0, errors.Wrap(err, "OpenRepository")
	}
	defer closer.Close()

	if pr.Flow == issues_model.PullRequestFlowGithub && !headGitRepo.IsBranchExist(pr.HeadBranch) {
		return 0, errors.New("Head branch does not exist, can not merge")
	}
	if pr.Flow == issues_model.PullRequestFlowAGit && !git.IsReferenceExist(ctx, headGitRepo.Path, pr.GetGitRefName()) {
		return 0, errors.New("Head branch does not exist, can not merge")
	}

	var sha string
	if pr.Flow == issues_model.PullRequestFlowGithub {
		sha, err = headGitRepo.GetBranchCommitID(pr.HeadBranch)
	} else {
		sha, err = headGitRepo.GetRefCommitID(pr.GetGitRefName())
	}
	if err != nil {
		return 0, err
	}

	if err := pr.LoadBaseRepo(ctx); err != nil {
		return 0, errors.Wrap(err, "LoadBaseRepo")
	}

	commitStatuses, commitStatus, _, err := git_model.GetLatestCommitStatuses(ctx, pr.BaseRepo.ID, sha, db.ListOptions{})
	if err != nil {
		return 0, errors.Wrap(err, "GetLatestCommitStatuses")
	}

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		return 0, errors.Wrap(err, "LoadProtectedBranch")
	}
	var requiredContexts []string
	if pb != nil {
		requiredContexts = pb.StatusCheckContexts
	}

	return MergeRequiredContextsCommitStatus(commitStatuses, commitStatus, requiredContexts), nil
}
