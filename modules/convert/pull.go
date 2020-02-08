// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	api "code.gitea.io/gitea/modules/structs"
)

// ToAPIPullRequest assumes following fields have been assigned with valid values:
// Required - Issue
// Optional - Merger
func ToAPIPullRequest(pr *models.PullRequest) *api.PullRequest {
	var (
		baseBranch *git.Branch
		headBranch *git.Branch
		baseCommit *git.Commit
		headCommit *git.Commit
		err        error
	)

	if err = pr.Issue.LoadRepo(); err != nil {
		log.Error("pr.Issue.LoadRepo[%d]: %v", pr.ID, err)
		return nil
	}

	apiIssue := pr.Issue.APIFormat()
	if pr.BaseRepo == nil {
		pr.BaseRepo, err = models.GetRepositoryByID(pr.BaseRepoID)
		if err != nil {
			log.Error("GetRepositoryById[%d]: %v", pr.ID, err)
			return nil
		}
	}
	if pr.HeadRepoID != 0 && pr.HeadRepo == nil {
		pr.HeadRepo, err = models.GetRepositoryByID(pr.HeadRepoID)
		if err != nil && !models.IsErrRepoNotExist(err) {
			log.Error("GetRepositoryById[%d]: %v", pr.ID, err)
			return nil

		}
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
		Comments:  apiIssue.Comments,
		HTMLURL:   pr.Issue.HTMLURL(),
		DiffURL:   pr.Issue.DiffURL(),
		PatchURL:  pr.Issue.PatchURL(),
		HasMerged: pr.HasMerged,
		MergeBase: pr.MergeBase,
		Deadline:  apiIssue.Deadline,
		Created:   pr.Issue.CreatedUnix.AsTimePtr(),
		Updated:   pr.Issue.UpdatedUnix.AsTimePtr(),
	}
	baseBranch, err = repo_module.GetBranch(pr.BaseRepo, pr.BaseBranch)
	if err != nil {
		if git.IsErrBranchNotExist(err) {
			apiPullRequest.Base = nil
		} else {
			log.Error("GetBranch[%s]: %v", pr.BaseBranch, err)
			return nil
		}
	} else {
		apiBaseBranchInfo := &api.PRBranchInfo{
			Name:       pr.BaseBranch,
			Ref:        pr.BaseBranch,
			RepoID:     pr.BaseRepoID,
			Repository: pr.BaseRepo.APIFormat(models.AccessModeNone),
		}
		baseCommit, err = baseBranch.GetCommit()
		if err != nil {
			if git.IsErrNotExist(err) {
				apiBaseBranchInfo.Sha = ""
			} else {
				log.Error("GetCommit[%s]: %v", baseBranch.Name, err)
				return nil
			}
		} else {
			apiBaseBranchInfo.Sha = baseCommit.ID.String()
		}
		apiPullRequest.Base = apiBaseBranchInfo
	}

	if pr.HeadRepo != nil {
		headBranch, err = repo_module.GetBranch(pr.HeadRepo, pr.HeadBranch)
		if err != nil {
			if git.IsErrBranchNotExist(err) {
				apiPullRequest.Head = nil
			} else {
				log.Error("GetBranch[%s]: %v", pr.HeadBranch, err)
				return nil
			}
		} else {
			apiHeadBranchInfo := &api.PRBranchInfo{
				Name:       pr.HeadBranch,
				Ref:        pr.HeadBranch,
				RepoID:     pr.HeadRepoID,
				Repository: pr.HeadRepo.APIFormat(models.AccessModeNone),
			}
			headCommit, err = headBranch.GetCommit()
			if err != nil {
				if git.IsErrNotExist(err) {
					apiHeadBranchInfo.Sha = ""
				} else {
					log.Error("GetCommit[%s]: %v", headBranch.Name, err)
					return nil
				}
			} else {
				apiHeadBranchInfo.Sha = headCommit.ID.String()
			}
			apiPullRequest.Head = apiHeadBranchInfo
		}
	} else {
		apiPullRequest.Head = &api.PRBranchInfo{
			Name:   pr.HeadBranch,
			Ref:    fmt.Sprintf("refs/pull/%d/head", pr.Index),
			RepoID: -1,
		}
	}

	if pr.Status != models.PullRequestStatusChecking {
		mergeable := !(pr.Status == models.PullRequestStatusConflict || pr.Status == models.PullRequestStatusError) && !pr.IsWorkInProgress()
		apiPullRequest.Mergeable = mergeable
	}
	if pr.HasMerged {
		apiPullRequest.Merged = pr.MergedUnix.AsTimePtr()
		apiPullRequest.MergedCommitID = &pr.MergedCommitID
		apiPullRequest.MergedBy = pr.Merger.APIFormat()
	}

	return apiPullRequest
}
