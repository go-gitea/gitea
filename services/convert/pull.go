// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"fmt"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// ToAPIPullRequest assumes following fields have been assigned with valid values:
// Required - Issue
// Optional - Merger
func ToAPIPullRequest(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User) *api.PullRequest {
	var (
		baseBranch *git.Branch
		headBranch *git.Branch
		baseCommit *git.Commit
		err        error
	)

	if err = pr.Issue.LoadRepo(ctx); err != nil {
		log.Error("pr.Issue.LoadRepo[%d]: %v", pr.ID, err)
		return nil
	}

	apiIssue := ToAPIIssue(ctx, pr.Issue)
	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("GetRepositoryById[%d]: %v", pr.ID, err)
		return nil
	}

	if err := pr.LoadHeadRepo(ctx); err != nil {
		log.Error("GetRepositoryById[%d]: %v", pr.ID, err)
		return nil
	}

	p, err := access_model.GetUserRepoPermission(ctx, pr.BaseRepo, doer)
	if err != nil {
		log.Error("GetUserRepoPermission[%d]: %v", pr.BaseRepoID, err)
		p.AccessMode = perm.AccessModeNone
	}

	apiPullRequest := &api.PullRequest{
		ID:        pr.ID,
		URL:       pr.Issue.HTMLURL(),
		Index:     pr.Index,
		Poster:    apiIssue.Poster,
		Title:     apiIssue.Title,
		Body:      apiIssue.Body,
		Labels:    apiIssue.Labels,
		Milestone: apiIssue.Milestone,
		Assignee:  apiIssue.Assignee,
		Assignees: apiIssue.Assignees,
		State:     apiIssue.State,
		IsLocked:  apiIssue.IsLocked,
		Comments:  apiIssue.Comments,
		HTMLURL:   pr.Issue.HTMLURL(),
		DiffURL:   pr.Issue.DiffURL(),
		PatchURL:  pr.Issue.PatchURL(),
		HasMerged: pr.HasMerged,
		MergeBase: pr.MergeBase,
		Mergeable: pr.Mergeable(ctx),
		Deadline:  apiIssue.Deadline,
		Created:   pr.Issue.CreatedUnix.AsTimePtr(),
		Updated:   pr.Issue.UpdatedUnix.AsTimePtr(),
		PinOrder:  apiIssue.PinOrder,

		AllowMaintainerEdit: pr.AllowMaintainerEdit,

		Base: &api.PRBranchInfo{
			Name:       pr.BaseBranch,
			Ref:        pr.BaseBranch,
			RepoID:     pr.BaseRepoID,
			Repository: ToRepo(ctx, pr.BaseRepo, p),
		},
		Head: &api.PRBranchInfo{
			Name:   pr.HeadBranch,
			Ref:    fmt.Sprintf("%s%d/head", git.PullPrefix, pr.Index),
			RepoID: -1,
		},
	}

	if err = pr.LoadRequestedReviewers(ctx); err != nil {
		log.Error("LoadRequestedReviewers[%d]: %v", pr.ID, err)
		return nil
	}
	for _, reviewer := range pr.RequestedReviewers {
		apiPullRequest.RequestedReviewers = append(apiPullRequest.RequestedReviewers, ToUser(ctx, reviewer, nil))
	}

	if pr.Issue.ClosedUnix != 0 {
		apiPullRequest.Closed = pr.Issue.ClosedUnix.AsTimePtr()
	}

	gitRepo, err := git.OpenRepository(ctx, pr.BaseRepo.RepoPath())
	if err != nil {
		log.Error("OpenRepository[%s]: %v", pr.BaseRepo.RepoPath(), err)
		return nil
	}
	defer gitRepo.Close()

	baseBranch, err = gitRepo.GetBranch(pr.BaseBranch)
	if err != nil && !git.IsErrBranchNotExist(err) {
		log.Error("GetBranch[%s]: %v", pr.BaseBranch, err)
		return nil
	}

	if err == nil {
		baseCommit, err = baseBranch.GetCommit()
		if err != nil && !git.IsErrNotExist(err) {
			log.Error("GetCommit[%s]: %v", baseBranch.Name, err)
			return nil
		}

		if err == nil {
			apiPullRequest.Base.Sha = baseCommit.ID.String()
		}
	}

	if pr.Flow == issues_model.PullRequestFlowAGit {
		gitRepo, err := git.OpenRepository(ctx, pr.BaseRepo.RepoPath())
		if err != nil {
			log.Error("OpenRepository[%s]: %v", pr.GetGitRefName(), err)
			return nil
		}
		defer gitRepo.Close()

		apiPullRequest.Head.Sha, err = gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			log.Error("GetRefCommitID[%s]: %v", pr.GetGitRefName(), err)
			return nil
		}
		apiPullRequest.Head.RepoID = pr.BaseRepoID
		apiPullRequest.Head.Repository = apiPullRequest.Base.Repository
		apiPullRequest.Head.Name = ""
	}

	if pr.HeadRepo != nil && pr.Flow == issues_model.PullRequestFlowGithub {
		p, err := access_model.GetUserRepoPermission(ctx, pr.HeadRepo, doer)
		if err != nil {
			log.Error("GetUserRepoPermission[%d]: %v", pr.HeadRepoID, err)
			p.AccessMode = perm.AccessModeNone
		}

		apiPullRequest.Head.RepoID = pr.HeadRepo.ID
		apiPullRequest.Head.Repository = ToRepo(ctx, pr.HeadRepo, p)

		headGitRepo, err := git.OpenRepository(ctx, pr.HeadRepo.RepoPath())
		if err != nil {
			log.Error("OpenRepository[%s]: %v", pr.HeadRepo.RepoPath(), err)
			return nil
		}
		defer headGitRepo.Close()

		headBranch, err = headGitRepo.GetBranch(pr.HeadBranch)
		if err != nil && !git.IsErrBranchNotExist(err) {
			log.Error("GetBranch[%s]: %v", pr.HeadBranch, err)
			return nil
		}

		if git.IsErrBranchNotExist(err) {
			headCommitID, err := headGitRepo.GetRefCommitID(apiPullRequest.Head.Ref)
			if err != nil && !git.IsErrNotExist(err) {
				log.Error("GetCommit[%s]: %v", pr.HeadBranch, err)
				return nil
			}
			if err == nil {
				apiPullRequest.Head.Sha = headCommitID
			}
		} else {
			commit, err := headBranch.GetCommit()
			if err != nil && !git.IsErrNotExist(err) {
				log.Error("GetCommit[%s]: %v", headBranch.Name, err)
				return nil
			}
			if err == nil {
				apiPullRequest.Head.Ref = pr.HeadBranch
				apiPullRequest.Head.Sha = commit.ID.String()
			}
		}
	}

	if len(apiPullRequest.Head.Sha) == 0 && len(apiPullRequest.Head.Ref) != 0 {
		baseGitRepo, err := git.OpenRepository(ctx, pr.BaseRepo.RepoPath())
		if err != nil {
			log.Error("OpenRepository[%s]: %v", pr.BaseRepo.RepoPath(), err)
			return nil
		}
		defer baseGitRepo.Close()
		refs, err := baseGitRepo.GetRefsFiltered(apiPullRequest.Head.Ref)
		if err != nil {
			log.Error("GetRefsFiltered[%s]: %v", apiPullRequest.Head.Ref, err)
			return nil
		} else if len(refs) == 0 {
			log.Error("unable to resolve PR head ref")
		} else {
			apiPullRequest.Head.Sha = refs[0].Object.String()
		}
	}

	if pr.HasMerged {
		apiPullRequest.Merged = pr.MergedUnix.AsTimePtr()
		apiPullRequest.MergedCommitID = &pr.MergedCommitID
		apiPullRequest.MergedBy = ToUser(ctx, pr.Merger, nil)
	}

	return apiPullRequest
}
