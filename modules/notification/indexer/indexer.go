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
func NewNotifier() base.Notifier {
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

func (r *indexerNotifier) NotifyDeleteComment(doer *models.User, comment *models.Comment) {
	if comment.Type == models.CommentTypeComment {
		models.UpdateIssueIndexer(comment.IssueID)
	}
}

func (r *indexerNotifier) NotifyDeleteRepository(doer *models.User, repo *models.Repository) {
	models.DeleteRepoFromIndexer(repo)
}

func (r *indexerNotifier) NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository) {

}

func (r *indexerNotifier) NotifyNewRelease(rel *models.Release) {
}

func (r *indexerNotifier) NotifyUpdateRelease(doer *models.User, rel *models.Release) {
}

func (r *indexerNotifier) NotifyDeleteRelease(doer *models.User, rel *models.Release) {
}

func (r *indexerNotifier) NotifyChangeMilestone(doer *models.User, issue *models.Issue) {
}

func (r *indexerNotifier) NotifyIssueChangeContent(doer *models.User, issue *models.Issue, oldContent string) {
}

func (r *indexerNotifier) NotifyIssueClearLabels(doer *models.User, issue *models.Issue) {
}

func (r *indexerNotifier) NotifyIssueChangeTitle(doer *models.User, issue *models.Issue, oldTitle string) {
}

func (r *indexerNotifier) NotifyIssueChangeLabels(doer *models.User, issue *models.Issue,
	addedLabels []*models.Label, removedLabels []*models.Label) {
}
