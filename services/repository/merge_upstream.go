// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"

	git_model "code.gitea.io/gitea/models/git"
	issue_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/pull"
)

type UpstreamDivergingInfo struct {
	BaseIsNewer   bool
	CommitsBehind int
	CommitsAhead  int
}

func MergeUpstream(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, branch string) (mergeStyle string, err error) {
	if err = repo.MustNotBeArchived(); err != nil {
		return "", err
	}
	if err = repo.GetBaseRepo(ctx); err != nil {
		return "", err
	}
	err = git.Push(ctx, repo.BaseRepo.RepoPath(), git.PushOptions{
		Remote: repo.RepoPath(),
		Branch: fmt.Sprintf("%s:%s", branch, branch),
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
		BaseBranch: branch,
	}
	fakeIssue.PullRequest = fakePR
	err = pull.Update(ctx, fakePR, doer, "merge upstream", false)
	if err != nil {
		return "", err
	}
	return "merge", nil
}

func GetUpstreamDivergingInfo(ctx context.Context, repo *repo_model.Repository, branch string) (*UpstreamDivergingInfo, error) {
	if !repo.IsFork {
		return nil, util.NewInvalidArgumentErrorf("repo is not a fork")
	}

	if repo.IsArchived {
		return nil, util.NewInvalidArgumentErrorf("repo is archived")
	}

	if err := repo.GetBaseRepo(ctx); err != nil {
		return nil, err
	}

	forkBranch, err := git_model.GetBranch(ctx, repo.ID, branch)
	if err != nil {
		return nil, err
	}

	baseBranch, err := git_model.GetBranch(ctx, repo.BaseRepo.ID, branch)
	if err != nil {
		return nil, err
	}

	info := &UpstreamDivergingInfo{}
	if forkBranch.CommitID == baseBranch.CommitID {
		return info, nil
	}

	// TODO: if the fork repo has new commits, this call will fail:
	// exit status 128 - fatal: Invalid symmetric difference expression aaaaaaaaaaaa...bbbbbbbbbbbb
	// so at the moment, we are not able to handle this case, should be improved in the future
	diff, err := git.GetDivergingCommits(ctx, repo.BaseRepo.RepoPath(), baseBranch.CommitID, forkBranch.CommitID)
	if err != nil {
		info.BaseIsNewer = baseBranch.UpdatedUnix > forkBranch.UpdatedUnix
		return info, nil
	}
	info.CommitsBehind, info.CommitsAhead = diff.Behind, diff.Ahead
	return info, nil
}
