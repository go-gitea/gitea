// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package action

import (
	"fmt"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
)

type actionReceiver struct {
}

var (
	receiver notification.NotifyReceiver = &actionReceiver{}
)

func ini() {
	notification.RegisterReceiver(receiver)
}

func (r *actionReceiver) Run() {}

func (r *actionReceiver) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	panic("not implementation")
}

func (r *actionReceiver) NotifyNewIssue(issue *models.Issue) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: issue.Poster.ID,
		ActUser:   issue.Poster,
		OpType:    models.ActionCreateIssue,
		Content:   fmt.Sprintf("%d|%s", issue.Index, issue.Title),
		RepoID:    issue.Repo.ID,
		Repo:      issue.Repo,
		IsPrivate: issue.Repo.IsPrivate,
	}); err != nil {
		log.Error(4, "NotifyWatchers: %v", err)
	}
}

func (r *actionReceiver) NotifyCloseIssue(issue *models.Issue, doer *models.User) {
	panic("not implementation")
}

func (r *actionReceiver) NotifyNewPullRequest(pr *models.PullRequest) {
	issue := pr.Issue
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: issue.Poster.ID,
		ActUser:   issue.Poster,
		OpType:    models.ActionCreatePullRequest,
		Content:   fmt.Sprintf("%d|%s", issue.Index, issue.Title),
		RepoID:    issue.Repo.ID,
		Repo:      issue.Repo,
		IsPrivate: issue.Repo.IsPrivate,
	}); err != nil {
		log.Error(4, "NotifyWatchers: %v", err)
	}
}

func (r *actionReceiver) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User, baseRepo *git.Repository) {
	if err := models.MergePullRequestAction(doer, pr.Issue.Repo, pr.Issue); err != nil {
		log.Error(4, "MergePullRequestAction [%d]: %v", pr.ID, err)
	}
}
