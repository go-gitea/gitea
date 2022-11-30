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
	"code.gitea.io/gitea/modules/structs"

	"github.com/pkg/errors"
)

// MergeRequiredContextsCommitStatus returns a commit status state for given required contexts
func MergeRequiredContextsCommitStatus(commitStatuses []*git_model.CommitStatus, requiredContexts []string) structs.CommitStatusState {
	if len(requiredContexts) == 0 {
		status := git_model.CalcCommitStatus(commitStatuses)
		if status != nil {
			return status.State
		}
		return structs.CommitStatusSuccess
	}

	returnedStatus := structs.CommitStatusSuccess
	for _, ctx := range requiredContexts {
		var targetStatus structs.CommitStatusState
		for _, commitStatus := range commitStatuses {
			if commitStatus.Context == ctx {
				targetStatus = commitStatus.State
				break
			}
		}

		if targetStatus == "" {
			targetStatus = structs.CommitStatusPending
			commitStatuses = append(commitStatuses, &git_model.CommitStatus{
				State:       targetStatus,
				Context:     ctx,
				Description: "Pending",
			})
		}
		if targetStatus.NoBetterThan(returnedStatus) {
			returnedStatus = targetStatus
		}
	}
	return returnedStatus
}

// IsCommitStatusContextSuccess returns true if all required status check contexts succeed.
func IsCommitStatusContextSuccess(commitStatuses []*git_model.CommitStatus, requiredContexts []string) bool {
	// If no specific context is required, require that last commit status is a success
	if len(requiredContexts) == 0 {
		status := git_model.CalcCommitStatus(commitStatuses)
		if status == nil || status.State != structs.CommitStatusSuccess {
			return false
		}
		return true
	}

	for _, ctx := range requiredContexts {
		var found bool
		for _, commitStatus := range commitStatuses {
			if commitStatus.Context == ctx {
				if commitStatus.State != structs.CommitStatusSuccess {
					return false
				}

				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// IsPullCommitStatusPass returns if all required status checks PASS
func IsPullCommitStatusPass(ctx context.Context, pr *issues_model.PullRequest) (bool, error) {
	if err := pr.LoadProtectedBranch(ctx); err != nil {
		return false, errors.Wrap(err, "GetLatestCommitStatus")
	}
	if pr.ProtectedBranch == nil || !pr.ProtectedBranch.EnableStatusCheck {
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
		return "", errors.Wrap(err, "LoadHeadRepo")
	}

	// check if all required status checks are successful
	headGitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, pr.HeadRepo.RepoPath())
	if err != nil {
		return "", errors.Wrap(err, "OpenRepository")
	}
	defer closer.Close()

	if pr.Flow == issues_model.PullRequestFlowGithub && !headGitRepo.IsBranchExist(pr.HeadBranch) {
		return "", errors.New("Head branch does not exist, can not merge")
	}
	if pr.Flow == issues_model.PullRequestFlowAGit && !git.IsReferenceExist(ctx, headGitRepo.Path, pr.GetGitRefName()) {
		return "", errors.New("Head branch does not exist, can not merge")
	}

	var sha string
	if pr.Flow == issues_model.PullRequestFlowGithub {
		sha, err = headGitRepo.GetBranchCommitID(pr.HeadBranch)
	} else {
		sha, err = headGitRepo.GetRefCommitID(pr.GetGitRefName())
	}
	if err != nil {
		return "", err
	}

	if err := pr.LoadBaseRepo(ctx); err != nil {
		return "", errors.Wrap(err, "LoadBaseRepo")
	}

	commitStatuses, _, err := git_model.GetLatestCommitStatus(ctx, pr.BaseRepo.ID, sha, db.ListOptions{})
	if err != nil {
		return "", errors.Wrap(err, "GetLatestCommitStatus")
	}

	if err := pr.LoadProtectedBranch(ctx); err != nil {
		return "", errors.Wrap(err, "LoadProtectedBranch")
	}
	var requiredContexts []string
	if pr.ProtectedBranch != nil {
		requiredContexts = pr.ProtectedBranch.StatusCheckContexts
	}

	return MergeRequiredContextsCommitStatus(commitStatuses, requiredContexts), nil
}
