// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package indexer

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
)

type indexerReceiver struct {
}

var (
	receiver notification.NotifyReceiver = &indexerReceiver{}
)

func ini() {
	notification.RegisterReceiver(receiver)
}

func (r *indexerReceiver) Run() {

}

func (r *indexerReceiver) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
}

func (r *indexerReceiver) NotifyNewIssue(issue *models.Issue) {
	models.UpdateIssueIndexer(issue.ID)
}

func (w *indexerReceiver) NotifyCloseIssue(issue *models.Issue, doer *models.User) {

}

func (w *indexerReceiver) NotifyNewPullRequest(pr *models.PullRequest) {
	models.UpdateIssueIndexer(pr.Issue.ID)
}

func (r *indexerReceiver) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User, baseRepo *git.Repository) {
}
