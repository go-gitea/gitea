// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package indexer

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification/base"
)

type indexerNotifier struct {
	base.NullNotifier
}

var (
	_ base.Notifier = &indexerNotifier{}
)

// NewNotifier create a new indexerNotifier notifier
func NewNotifier() base.Notifier {
	return &indexerNotifier{}
}

func (r *indexerNotifier) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	if comment.Type == models.CommentTypeComment {
		models.UpdateIssueIndexer(issue)
	}
}

func (r *indexerNotifier) NotifyNewIssue(issue *models.Issue) {
	models.UpdateIssueIndexer(issue)
}

func (r *indexerNotifier) NotifyNewPullRequest(pr *models.PullRequest) {
	models.UpdateIssueIndexer(pr.Issue)
}

func (r *indexerNotifier) NotifyUpdateComment(doer *models.User, c *models.Comment, repoID int64, oldContent string) {
	if c.Type == models.CommentTypeComment {
		models.UpdateIssueIndexer(c.Issue)
	}
}

func (r *indexerNotifier) NotifyDeleteComment(doer *models.User, comment *models.Comment, repoID int64) {
	if comment.Type == models.CommentTypeComment {
		models.UpdateIssueIndexer(comment.Issue)
	}
}

func (r *indexerNotifier) NotifyDeleteRepository(doer *models.User, repo *models.Repository) {
	models.DeleteRepoIssueIndexer(repo)
}

func (r *indexerNotifier) NotifyIssueChangeContent(doer *models.User, issue *models.Issue, oldContent string) {
	models.UpdateIssueIndexer(issue)
}

func (r *indexerNotifier) NotifyIssueChangeTitle(doer *models.User, issue *models.Issue, oldTitle string) {
	models.UpdateIssueIndexer(issue)
}
