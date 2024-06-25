// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/xorm"
)

// PullRequestsOptions holds the options for PRs
type PullRequestsOptions struct {
	db.ListOptions
	State       string
	SortType    string
	Labels      []int64
	MilestoneID int64
}

func listPullRequestStatement(ctx context.Context, baseRepoID int64, opts *PullRequestsOptions) *xorm.Session {
	sess := db.GetEngine(ctx).Where("pull_request.base_repo_id=?", baseRepoID)

	sess.Join("INNER", "issue", "pull_request.issue_id = issue.id")
	switch opts.State {
	case "closed", "open":
		sess.And("issue.is_closed=?", opts.State == "closed")
	}

	if len(opts.Labels) > 0 {
		sess.Join("INNER", "issue_label", "issue.id = issue_label.issue_id").
			In("issue_label.label_id", opts.Labels)
	}

	if opts.MilestoneID > 0 {
		sess.And("issue.milestone_id=?", opts.MilestoneID)
	}

	return sess
}

// GetUnmergedPullRequestsByHeadInfo returns all pull requests that are open and has not been merged
func GetUnmergedPullRequestsByHeadInfo(ctx context.Context, repoID int64, branch string) ([]*PullRequest, error) {
	prs := make([]*PullRequest, 0, 2)
	sess := db.GetEngine(ctx).
		Join("INNER", "issue", "issue.id = pull_request.issue_id").
		Where("head_repo_id = ? AND head_branch = ? AND has_merged = ? AND issue.is_closed = ? AND flow = ?", repoID, branch, false, false, PullRequestFlowGithub)
	return prs, sess.Find(&prs)
}

// CanMaintainerWriteToBranch check whether user is a maintainer and could write to the branch
func CanMaintainerWriteToBranch(ctx context.Context, p access_model.Permission, branch string, user *user_model.User) bool {
	if p.CanWrite(unit.TypeCode) {
		return true
	}

	// the code below depends on units to get the repository ID, not ideal but just keep it for now
	firstUnitRepoID := p.GetFirstUnitRepoID()
	if firstUnitRepoID == 0 {
		return false
	}

	prs, err := GetUnmergedPullRequestsByHeadInfo(ctx, firstUnitRepoID, branch)
	if err != nil {
		return false
	}

	for _, pr := range prs {
		if pr.AllowMaintainerEdit {
			err = pr.LoadBaseRepo(ctx)
			if err != nil {
				continue
			}
			prPerm, err := access_model.GetUserRepoPermission(ctx, pr.BaseRepo, user)
			if err != nil {
				continue
			}
			if prPerm.CanWrite(unit.TypeCode) {
				return true
			}
		}
	}
	return false
}

// HasUnmergedPullRequestsByHeadInfo checks if there are open and not merged pull request
// by given head information (repo and branch)
func HasUnmergedPullRequestsByHeadInfo(ctx context.Context, repoID int64, branch string) (bool, error) {
	return db.GetEngine(ctx).
		Where("head_repo_id = ? AND head_branch = ? AND has_merged = ? AND issue.is_closed = ? AND flow = ?",
			repoID, branch, false, false, PullRequestFlowGithub).
		Join("INNER", "issue", "issue.id = pull_request.issue_id").
		Exist(&PullRequest{})
}

// GetUnmergedPullRequestsByBaseInfo returns all pull requests that are open and has not been merged
// by given base information (repo and branch).
func GetUnmergedPullRequestsByBaseInfo(ctx context.Context, repoID int64, branch string) ([]*PullRequest, error) {
	prs := make([]*PullRequest, 0, 2)
	return prs, db.GetEngine(ctx).
		Where("base_repo_id=? AND base_branch=? AND has_merged=? AND issue.is_closed=?",
			repoID, branch, false, false).
		OrderBy("issue.updated_unix DESC").
		Join("INNER", "issue", "issue.id=pull_request.issue_id").
		Find(&prs)
}

// GetPullRequestIDsByCheckStatus returns all pull requests according the special checking status.
func GetPullRequestIDsByCheckStatus(ctx context.Context, status PullRequestStatus) ([]int64, error) {
	prs := make([]int64, 0, 10)
	return prs, db.GetEngine(ctx).Table("pull_request").
		Where("status=?", status).
		Cols("pull_request.id").
		Find(&prs)
}

// PullRequests returns all pull requests for a base Repo by the given conditions
func PullRequests(ctx context.Context, baseRepoID int64, opts *PullRequestsOptions) (PullRequestList, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}

	countSession := listPullRequestStatement(ctx, baseRepoID, opts)
	maxResults, err := countSession.Count(new(PullRequest))
	if err != nil {
		log.Error("Count PRs: %v", err)
		return nil, maxResults, err
	}

	findSession := listPullRequestStatement(ctx, baseRepoID, opts)
	applySorts(findSession, opts.SortType, 0)
	findSession = db.SetSessionPagination(findSession, opts)
	prs := make([]*PullRequest, 0, opts.PageSize)
	return prs, maxResults, findSession.Find(&prs)
}

// PullRequestList defines a list of pull requests
type PullRequestList []*PullRequest

func (prs PullRequestList) getRepositoryIDs() []int64 {
	repoIDs := make(container.Set[int64])
	for _, pr := range prs {
		if pr.BaseRepo == nil && pr.BaseRepoID > 0 {
			repoIDs.Add(pr.BaseRepoID)
		}
		if pr.HeadRepo == nil && pr.HeadRepoID > 0 {
			repoIDs.Add(pr.HeadRepoID)
		}
	}
	return repoIDs.Values()
}

func (prs PullRequestList) LoadRepositories(ctx context.Context) error {
	repoIDs := prs.getRepositoryIDs()
	reposMap := make(map[int64]*repo_model.Repository, len(repoIDs))
	if err := db.GetEngine(ctx).
		In("id", repoIDs).
		Find(&reposMap); err != nil {
		return fmt.Errorf("find repos: %w", err)
	}
	for _, pr := range prs {
		if pr.BaseRepo == nil {
			pr.BaseRepo = reposMap[pr.BaseRepoID]
		}
		if pr.HeadRepo == nil {
			pr.HeadRepo = reposMap[pr.HeadRepoID]
			pr.isHeadRepoLoaded = true
		}
	}
	return nil
}

func (prs PullRequestList) LoadAttributes(ctx context.Context) error {
	if _, err := prs.LoadIssues(ctx); err != nil {
		return err
	}
	return nil
}

func (prs PullRequestList) LoadIssues(ctx context.Context) (IssueList, error) {
	if len(prs) == 0 {
		return nil, nil
	}

	// Load issues which are not loaded
	issueIDs := container.FilterSlice(prs, func(pr *PullRequest) (int64, bool) {
		return pr.IssueID, pr.Issue == nil && pr.IssueID > 0
	})
	issues := make(map[int64]*Issue, len(issueIDs))
	if err := db.GetEngine(ctx).
		In("id", issueIDs).
		Find(&issues); err != nil {
		return nil, fmt.Errorf("find issues: %w", err)
	}

	issueList := make(IssueList, 0, len(prs))
	for _, pr := range prs {
		if pr.Issue == nil {
			pr.Issue = issues[pr.IssueID]
			/*
				Old code:
				pr.Issue.PullRequest = pr // panic here means issueIDs and prs are not in sync

				It's worth panic because it's almost impossible to happen under normal use.
				But in integration testing, an asynchronous task could read a database that has been reset.
				So returning an error would make more sense, let the caller has a choice to ignore it.
			*/
			if pr.Issue == nil {
				return nil, fmt.Errorf("issues and prs may be not in sync: cannot find issue %v for pr %v: %w", pr.IssueID, pr.ID, util.ErrNotExist)
			}
		}
		pr.Issue.PullRequest = pr
		if pr.Issue.Repo == nil {
			pr.Issue.Repo = pr.BaseRepo
		}
		issueList = append(issueList, pr.Issue)
	}
	return issueList, nil
}

// GetIssueIDs returns all issue ids
func (prs PullRequestList) GetIssueIDs() []int64 {
	return container.FilterSlice(prs, func(pr *PullRequest) (int64, bool) {
		return pr.IssueID, pr.IssueID > 0
	})
}

// HasMergedPullRequestInRepo returns whether the user(poster) has merged pull-request in the repo
func HasMergedPullRequestInRepo(ctx context.Context, repoID, posterID int64) (bool, error) {
	return db.GetEngine(ctx).
		Join("INNER", "pull_request", "pull_request.issue_id = issue.id").
		Where("repo_id=?", repoID).
		And("poster_id=?", posterID).
		And("is_pull=?", true).
		And("pull_request.has_merged=?", true).
		Select("issue.id").
		Limit(1).
		Get(new(Issue))
}

// GetPullRequestByIssueIDs returns all pull requests by issue ids
func GetPullRequestByIssueIDs(ctx context.Context, issueIDs []int64) (PullRequestList, error) {
	prs := make([]*PullRequest, 0, len(issueIDs))
	return prs, db.GetEngine(ctx).
		Where("issue_id > 0").
		In("issue_id", issueIDs).
		Find(&prs)
}
