// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/xorm"
)

// PullRequestsOptions holds the options for PRs
type PullRequestsOptions struct {
	db.ListOptions
	State       string
	SortType    string
	Labels      []string
	MilestoneID int64
}

func listPullRequestStatement(ctx context.Context, baseRepoID int64, opts *PullRequestsOptions) (*xorm.Session, error) {
	sess := db.GetEngine(ctx).Where("pull_request.base_repo_id=?", baseRepoID)

	sess.Join("INNER", "issue", "pull_request.issue_id = issue.id")
	switch opts.State {
	case "closed", "open":
		sess.And("issue.is_closed=?", opts.State == "closed")
	}

	if labelIDs, err := base.StringsToInt64s(opts.Labels); err != nil {
		return nil, err
	} else if len(labelIDs) > 0 {
		sess.Join("INNER", "issue_label", "issue.id = issue_label.issue_id").
			In("issue_label.label_id", labelIDs)
	}

	if opts.MilestoneID > 0 {
		sess.And("issue.milestone_id=?", opts.MilestoneID)
	}

	return sess, nil
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

	if len(p.Units) < 1 {
		return false
	}

	prs, err := GetUnmergedPullRequestsByHeadInfo(ctx, p.Units[0].RepoID, branch)
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
func PullRequests(ctx context.Context, baseRepoID int64, opts *PullRequestsOptions) ([]*PullRequest, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}

	countSession, err := listPullRequestStatement(ctx, baseRepoID, opts)
	if err != nil {
		log.Error("listPullRequestStatement: %v", err)
		return nil, 0, err
	}
	maxResults, err := countSession.Count(new(PullRequest))
	if err != nil {
		log.Error("Count PRs: %v", err)
		return nil, maxResults, err
	}

	findSession, err := listPullRequestStatement(ctx, baseRepoID, opts)
	applySorts(findSession, opts.SortType, 0)
	if err != nil {
		log.Error("listPullRequestStatement: %v", err)
		return nil, maxResults, err
	}
	findSession = db.SetSessionPagination(findSession, opts)
	prs := make([]*PullRequest, 0, opts.PageSize)
	return prs, maxResults, findSession.Find(&prs)
}

// PullRequestList defines a list of pull requests
type PullRequestList []*PullRequest

func (prs PullRequestList) LoadAttributes(ctx context.Context) error {
	if len(prs) == 0 {
		return nil
	}

	// Load issues.
	issueIDs := prs.GetIssueIDs()
	issues := make([]*Issue, 0, len(issueIDs))
	if err := db.GetEngine(ctx).
		Where("id > 0").
		In("id", issueIDs).
		Find(&issues); err != nil {
		return fmt.Errorf("find issues: %w", err)
	}

	set := make(map[int64]*Issue)
	for i := range issues {
		set[issues[i].ID] = issues[i]
	}
	for _, pr := range prs {
		pr.Issue = set[pr.IssueID]
		/*
			Old code:
			pr.Issue.PullRequest = pr // panic here means issueIDs and prs are not in sync

			It's worth panic because it's almost impossible to happen under normal use.
			But in integration testing, an asynchronous task could read a database that has been reset.
			So returning an error would make more sense, let the caller has a choice to ignore it.
		*/
		if pr.Issue == nil {
			return fmt.Errorf("issues and prs may be not in sync: cannot find issue %v for pr %v: %w", pr.IssueID, pr.ID, util.ErrNotExist)
		}
		pr.Issue.PullRequest = pr
	}
	return nil
}

// GetIssueIDs returns all issue ids
func (prs PullRequestList) GetIssueIDs() []int64 {
	issueIDs := make([]int64, 0, len(prs))
	for i := range prs {
		issueIDs = append(issueIDs, prs[i].IssueID)
	}
	return issueIDs
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
