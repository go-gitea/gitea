// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	issue_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/pull"
)

// MergeUpstream merges the base repository's default branch into the fork repository's current branch.
func MergeUpstream(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, branch string) (mergeStyle string, err error) {
	if err = repo.MustNotBeArchived(); err != nil {
		return "", err
	}
	if err = repo.GetBaseRepo(ctx); err != nil {
		return "", err
	}
	err = git.Push(ctx, repo.BaseRepo.RepoPath(), git.PushOptions{
		Remote: repo.RepoPath(),
		Branch: fmt.Sprintf("%s:%s", repo.BaseRepo.DefaultBranch, branch),
		Env:    repo_module.PushingEnvironment(doer, repo),
	})
	if err == nil {
		return "fast-forward", nil
	}
	if !git.IsErrPushOutOfDate(err) && !git.IsErrPushRejected(err) {
		return "", err
	}

	// TODO: FakePR: it is somewhat hacky, but it is the only way to "merge" at the moment
	// ideally in the future the "merge" functions should be refactored to decouple from the PullRequest
	fakeIssue := &issue_model.Issue{
		ID:       -1,
		RepoID:   repo.ID,
		Repo:     repo,
		Index:    -1,
		PosterID: doer.ID,
		Poster:   doer,
		IsPull:   true,
	}
	fakePR := &issue_model.PullRequest{
		ID:         -1,
		Status:     issue_model.PullRequestStatusMergeable,
		IssueID:    -1,
		Issue:      fakeIssue,
		Index:      -1,
		HeadRepoID: repo.ID,
		HeadRepo:   repo,
		BaseRepoID: repo.BaseRepo.ID,
		BaseRepo:   repo.BaseRepo,
		HeadBranch: branch, // maybe HeadCommitID is not needed
		BaseBranch: repo.BaseRepo.DefaultBranch,
	}
	fakeIssue.PullRequest = fakePR
	err = pull.Update(ctx, fakePR, doer, "merge upstream", false)
	if err != nil {
		return "", err
	}
	return "merge", nil
}

// GetUpstreamDivergingInfo returns the information about the divergence between the fork repository's branch and the base repository's default branch.
func GetUpstreamDivergingInfo(ctx reqctx.RequestContext, forkRepo *repo_model.Repository, forkBranch string) (*BranchDivergingInfo, error) {
	if !forkRepo.IsFork {
		return nil, util.NewInvalidArgumentErrorf("repo is not a fork")
	}

	if forkRepo.IsArchived {
		return nil, util.NewInvalidArgumentErrorf("repo is archived")
	}

	if err := forkRepo.GetBaseRepo(ctx); err != nil {
		return nil, err
	}

	return GetBranchDivergingInfo(ctx, forkRepo.BaseRepo, forkRepo.BaseRepo.DefaultBranch, forkRepo, forkBranch)
}
