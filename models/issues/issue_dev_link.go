// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"strconv"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/timeutil"
)

type IssueDevLinkType int

const (
	IssueDevLinkTypeBranch IssueDevLinkType = iota + 1
	IssueDevLinkTypePullRequest
)

type IssueDevLink struct {
	ID           int64 `xorm:"pk autoincr"`
	IssueID      int64 `xorm:"INDEX"`
	LinkType     IssueDevLinkType
	LinkedRepoID int64              `xorm:"INDEX"` // it can link to self repo or other repo
	LinkIndex    string             // branch name, pull request number or commit sha
	CreatedUnix  timeutil.TimeStamp `xorm:"INDEX created"`

	LinkedRepo    *repo_model.Repository `xorm:"-"`
	PullRequest   *PullRequest           `xorm:"-"`
	Branch        *git_model.Branch      `xorm:"-"`
	DisplayBranch bool                   `xorm:"-"`
}

func init() {
	db.RegisterModel(new(IssueDevLink))
}

// IssueDevLinks represents a list of issue development links
type IssueDevLinks []*IssueDevLink

// FindIssueDevLinksByIssueID returns a list of issue development links by issue ID
func FindIssueDevLinksByIssueID(ctx context.Context, issueID int64) (IssueDevLinks, error) {
	links := make(IssueDevLinks, 0, 5)
	return links, db.GetEngine(ctx).Where("issue_id = ?", issueID).Find(&links)
}

func FindDevLinksByBranch(ctx context.Context, repoID, linkedRepoID int64, branchName string) (IssueDevLinks, error) {
	links := make(IssueDevLinks, 0, 5)
	return links, db.GetEngine(ctx).
		Join("INNER", "issue", "issue_dev_link.issue_id = issue.id").
		Where("link_type = ? AND link_index = ? AND linked_repo_id = ?",
			IssueDevLinkTypeBranch, branchName, linkedRepoID).
		And("issue.repo_id=?", repoID).
		Find(&links)
}

func CreateIssueDevLink(ctx context.Context, link *IssueDevLink) error {
	_, err := db.GetEngine(ctx).Insert(link)
	return err
}

func DeleteIssueDevLinkByBranchName(ctx context.Context, repoID int64, branchName string) error {
	_, err := db.GetEngine(ctx).
		Where("linked_repo_id = ? AND link_type = ? AND link_index = ?",
			repoID, IssueDevLinkTypeBranch, branchName).
		Delete(new(IssueDevLink))
	return err
}

func DeleteIssueDevLinkByPullRequestID(ctx context.Context, pullID int64) error {
	pullIDStr := strconv.FormatInt(pullID, 10)
	_, err := db.GetEngine(ctx).Where("link_type = ? AND link_index = ?", IssueDevLinkTypePullRequest, pullIDStr).
		Delete(new(IssueDevLink))
	return err
}
