// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"errors"
	"fmt"
	"slices"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/container"
	"gitea.dev/modules/gitrepo"
	"gitea.dev/modules/glob"
	"gitea.dev/modules/log"
)

// MergeRequiredContextsCommitStatus returns a commit status state for given required contexts
func MergeRequiredContextsCommitStatus(commitStatuses []*git_model.CommitStatus, requiredContexts []string) commitstatus.CommitStatusState {
	if len(commitStatuses) == 0 {
		return commitstatus.CommitStatusPending
	}

	if len(requiredContexts) == 0 {
		return git_model.CalcCommitStatus(commitStatuses).State
	}

	requiredContextsGlob := make(map[string]glob.Glob, len(requiredContexts))
	for _, ctx := range requiredContexts {
		if gp, err := glob.Compile(ctx); err != nil {
			log.Error("glob.Compile %s failed. Error: %v", ctx, err)
		} else {
			requiredContextsGlob[ctx] = gp
		}
	}

	requiredCommitStatuses := make([]*git_model.CommitStatus, 0, len(commitStatuses))
	allRequiredContextsMatched := true
	for _, gp := range requiredContextsGlob {
		requiredContextMatched := false
		for _, commitStatus := range commitStatuses {
			if gp.Match(commitStatus.Context) {
				requiredCommitStatuses = append(requiredCommitStatuses, commitStatus)
				requiredContextMatched = true
			}
		}
		allRequiredContextsMatched = allRequiredContextsMatched && requiredContextMatched
	}
	if len(requiredCommitStatuses) == 0 {
		return commitstatus.CommitStatusPending
	}

	returnedStatus := git_model.CalcCommitStatus(requiredCommitStatuses).State
	if allRequiredContextsMatched {
		return returnedStatus
	}

	if returnedStatus == commitstatus.CommitStatusFailure {
		return commitstatus.CommitStatusFailure
	}
	// even if part of success, return pending
	return commitstatus.CommitStatusPending
}

// IsPullCommitStatusPass returns if all required status checks PASS
func IsPullCommitStatusPass(ctx context.Context, pr *issues_model.PullRequest) (bool, error) {
	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		return false, fmt.Errorf("GetFirstMatchProtectedBranchRule: %w", err)
	}
	if pb == nil {
		return true, nil
	}
	if !pb.EnableStatusCheck {
		// The branch's own status check is off, but required scoped checks (mandated by the owner or instance admin) still gate the merge.
		if err := pr.LoadBaseRepo(ctx); err != nil {
			return false, err
		}
		required, err := EffectiveRequiredContexts(ctx, pr.BaseRepo, pb)
		if err != nil {
			return false, err
		}
		if len(required) == 0 {
			// With none in effect there is nothing to enforce, so don't block
			return true, nil
		}
	}

	state, err := GetPullRequestCommitStatusState(ctx, pr)
	if err != nil {
		return false, err
	}
	return state.IsSuccess(), nil
}

// GetPullRequestCommitStatusState returns pull request merged commit status state
func GetPullRequestCommitStatusState(ctx context.Context, pr *issues_model.PullRequest) (commitstatus.CommitStatusState, error) {
	// Ensure HeadRepo is loaded
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return "", fmt.Errorf("LoadHeadRepo: %w", err)
	}

	// check if all required status checks are successful
	headGitRepo, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, pr.HeadRepo)
	if err != nil {
		return "", fmt.Errorf("OpenRepository: %w", err)
	}
	defer closer.Close()

	if pr.Flow == issues_model.PullRequestFlowGithub {
		if exist, err := git_model.IsBranchExist(ctx, pr.HeadRepo.ID, pr.HeadBranch); err != nil {
			return "", fmt.Errorf("IsBranchExist: %w", err)
		} else if !exist {
			return "", errors.New("Head branch does not exist, can not merge")
		}
	}
	if pr.Flow == issues_model.PullRequestFlowAGit && !gitrepo.IsReferenceExist(ctx, pr.HeadRepo, pr.GetGitHeadRefName()) {
		return "", errors.New("Head branch does not exist, can not merge")
	}

	var sha string
	if pr.Flow == issues_model.PullRequestFlowGithub {
		sha, err = headGitRepo.GetBranchCommitID(pr.HeadBranch)
	} else {
		sha, err = headGitRepo.GetRefCommitID(pr.GetGitHeadRefName())
	}
	if err != nil {
		return "", err
	}

	if err := pr.LoadBaseRepo(ctx); err != nil {
		return "", fmt.Errorf("LoadBaseRepo: %w", err)
	}

	commitStatuses, err := git_model.GetLatestCommitStatus(ctx, pr.BaseRepo.ID, sha, db.ListOptionsAll)
	if err != nil {
		return "", fmt.Errorf("GetLatestCommitStatus: %w", err)
	}

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		return "", fmt.Errorf("LoadProtectedBranch: %w", err)
	}
	requiredContexts, err := EffectiveRequiredContexts(ctx, pr.BaseRepo, pb)
	if err != nil {
		return "", err
	}

	return MergeRequiredContextsCommitStatus(commitStatuses, requiredContexts), nil
}

// EffectiveRequiredContexts returns the required status-check contexts for a PR head:
//  1. every required scoped workflow's status-check patterns effective for the repo (always)
//  2. the branch protection's own configured contexts, only when its status check is enabled
func EffectiveRequiredContexts(ctx context.Context, repo *repo_model.Repository, pb *git_model.ProtectedBranch) ([]string, error) {
	if pb == nil {
		return nil, nil
	}

	sources, err := actions_model.GetEffectiveScopedWorkflowSources(ctx, repo.OwnerID)
	if err != nil {
		return nil, fmt.Errorf("GetEffectiveScopedWorkflowSources: %w", err)
	}

	// Every required scoped workflow's admin-authored status-check patterns, matched must-present-and-pass:
	// a required scoped check that posts no matching status blocks the merge.
	seen := make(container.Set[string])
	var scoped []string
	for _, source := range sources {
		for _, cfg := range source.WorkflowConfigs {
			if !cfg.Required {
				continue
			}
			for _, p := range cfg.Patterns {
				if seen.Add(p) {
					scoped = append(scoped, p)
				}
			}
		}
	}

	slices.Sort(scoped) // sort for stable output

	// With the branch protection's own status check disabled, only the required scoped checks (mandated by the owner or instance admin) gate the merge.
	if !pb.EnableStatusCheck {
		return scoped, nil
	}

	// Status check enabled: the rule's configured contexts, then the scoped patterns not already among them.
	required := slices.Clone(pb.StatusCheckContexts)
	for _, p := range scoped {
		if !slices.Contains(pb.StatusCheckContexts, p) {
			required = append(required, p)
		}
	}
	return required, nil
}
