// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package indexer

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification/base"
)

type indexerNotifier struct {
}

var (
	_ base.Notifier = &indexerNotifier{}
)

// NewNotifier create a new indexerNotifier notifier
func NewNotifier() *indexerNotifier {
	return &indexerNotifier{}
}

func (r *indexerNotifier) Run() {
}

func (r *indexerNotifier) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
}

func (r *indexerNotifier) NotifyNewIssue(issue *models.Issue) {
	models.UpdateIssueIndexer(issue.ID)
}

func (r *indexerNotifier) NotifyCloseIssue(issue *models.Issue, doer *models.User) {

}

func (r *indexerNotifier) NotifyNewPullRequest(pr *models.PullRequest) {
	models.UpdateIssueIndexer(pr.Issue.ID)
}

func (r *indexerNotifier) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User, baseRepo *git.Repository) {
}

func (r *indexerNotifier) NotifyUpdateComment(doer *models.User, c *models.Comment, oldContent string) {
	if c.Type == models.CommentTypeComment {
		models.UpdateIssueIndexer(c.IssueID)
	}
}
