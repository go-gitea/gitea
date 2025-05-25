// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/commitstatus"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"

	"github.com/gobwas/glob"
	"github.com/pkg/errors"
)

// MergeRequiredContextsCommitStatus returns a commit status state for given required contexts
func MergeRequiredContextsCommitStatus(commitStatuses []*git_model.CommitStatus, requiredContexts []string) commitstatus.CombinedStatusState {
	if len(requiredContexts) > 0 {
		requiredContextsGlob := make(map[string]glob.Glob, len(requiredContexts))
		for _, ctx := range requiredContexts {
			if gp, err := glob.Compile(ctx); err != nil {
				log.Error("glob.Compile %s failed. Error: %v", ctx, err)
			} else {
				requiredContextsGlob[ctx] = gp
			}
		}

		requiredCommitStatuses := make([]*git_model.CommitStatus, 0, len(commitStatuses))
		for _, gp := range requiredContextsGlob {
			for _, commitStatus := range commitStatuses {
				if gp.Match(commitStatus.Context) {
					requiredCommitStatuses = append(requiredCommitStatuses, commitStatus)
					break
				}
			}
		}
		if len(requiredCommitStatuses) > 0 {
			return git_model.CalcCombinedStatusState(requiredCommitStatuses)
		}
	}

	return git_model.CalcCombinedStatusState(commitStatuses)
}

// IsCommitStatusContextSuccess returns true if all required status check contexts succeed.
func IsCommitStatusContextSuccess(commitStatuses []*git_model.CommitStatus, requiredContexts []string) bool {
	// If no specific context is required, require that last commit status is a success
	if len(requiredContexts) == 0 {
		return git_model.CalcCombinedStatusState(commitStatuses) == commitstatus.CombinedStatusSuccess
	}

	for _, ctx := range requiredContexts {
		var found bool
		for _, commitStatus := range commitStatuses {
			if commitStatus.Context == ctx {
				if commitStatus.State != commitstatus.CommitStatusSuccess {
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
	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		return false, errors.Wrap(err, "GetLatestCommitStatus")
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
func GetPullRequestCommitStatusState(ctx context.Context, pr *issues_model.PullRequest) (commitstatus.CombinedStatusState, error) {
	// Ensure HeadRepo is loaded
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return "", errors.Wrap(err, "LoadHeadRepo")
	}

	// check if all required status checks are successful
	headGitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, pr.HeadRepo)
	if err != nil {
		return "", errors.Wrap(err, "OpenRepository")
	}
	defer closer.Close()

	if pr.Flow == issues_model.PullRequestFlowGithub && !gitrepo.IsBranchExist(ctx, pr.HeadRepo, pr.HeadBranch) {
		return "", errors.New("Head branch does not exist, can not merge")
	}
	if pr.Flow == issues_model.PullRequestFlowAGit && !gitrepo.IsReferenceExist(ctx, pr.HeadRepo, pr.GetGitRefName()) {
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

	commitStatuses, _, err := git_model.GetLatestCommitStatus(ctx, pr.BaseRepo.ID, sha, db.ListOptionsAll)
	if err != nil {
		return "", errors.Wrap(err, "GetLatestCommitStatus")
	}

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		return "", errors.Wrap(err, "LoadProtectedBranch")
	}
	var requiredContexts []string
	if pb != nil {
		requiredContexts = pb.StatusCheckContexts
	}

	return MergeRequiredContextsCommitStatus(commitStatuses, requiredContexts), nil
}
