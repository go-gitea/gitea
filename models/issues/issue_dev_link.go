// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

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
	ID            int64 `xorm:"pk autoincr"`
	IssueID       int64 `xorm:"INDEX"`
	LinkType      IssueDevLinkType
	LinkedRepoID  int64                  `xorm:"INDEX"` // it can link to self repo or other repo
	LinkID        int64                  // branch id in branch table or the pull request id(not issue if of the pull request)
	CreatedUnix   timeutil.TimeStamp     `xorm:"INDEX created"`
	Repo          *repo_model.Repository `xorm:"-"` // current repo of issue
	LinkedRepo    *repo_model.Repository `xorm:"-"`
	PullRequest   *PullRequest           `xorm:"-"`
	Branch        *git_model.Branch      `xorm:"-"`
	DisplayBranch bool                   `xorm:"-"`
}

func init() {
	db.RegisterModel(new(IssueDevLink))
}

func (i *IssueDevLink) BranchFullName() string {
	if i.Repo.ID == i.LinkedRepo.ID {
		return i.Branch.Name
	}
	return i.LinkedRepo.FullName() + ":" + i.Branch.Name
}

// IssueDevLinks represents a list of issue development links
type IssueDevLinks []*IssueDevLink

// FindIssueDevLinksByIssueID returns a list of issue development links by issue ID
func FindIssueDevLinksByIssueID(ctx context.Context, issueID int64) (IssueDevLinks, error) {
	links := make(IssueDevLinks, 0, 5)
	return links, db.GetEngine(ctx).Where("issue_id = ?", issueID).Find(&links)
}

func CreateIssueDevLink(ctx context.Context, link *IssueDevLink) error {
	_, err := db.GetEngine(ctx).Insert(link)
	return err
}
