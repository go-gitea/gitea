// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package action

import (
	"fmt"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
)

type actionNotifier struct {
}

var (
	_ base.Notifier = &actionNotifier{}
)

// NewNotifier returns a new actionNotifier
func NewNotifier() base.Notifier {
	return &actionNotifier{}
}

func (r *actionNotifier) Run() {}

func (r *actionNotifier) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	panic("not implementation")
}

func (r *actionNotifier) NotifyNewIssue(issue *models.Issue) {
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

func (r *actionNotifier) NotifyCloseIssue(issue *models.Issue, doer *models.User) {
	panic("not implementation")
}

func (r *actionNotifier) NotifyNewPullRequest(pr *models.PullRequest) {
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

func (r *actionNotifier) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User, baseRepo *git.Repository) {
	if err := models.MergePullRequestAction(doer, pr.Issue.Repo, pr.Issue); err != nil {
		log.Error(4, "MergePullRequestAction [%d]: %v", pr.ID, err)
	}
}

func (r *actionNotifier) NotifyUpdateComment(doer *models.User, c *models.Comment, oldContent string) {
}

func (r *actionNotifier) NotifyDeleteComment(doer *models.User, c *models.Comment) {
	if err := models.UpdateCommentAction(c); err != nil {
		log.Error(4, "UpdateCommentAction [%d]: %v", c.ID, err)
	}
}

func (r *actionNotifier) NotifyDeleteRepository(doer *models.User, repo *models.Repository) {

}

func (r *actionNotifier) NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository) {

}

func (r *actionNotifier) NotifyNewRelease(rel *models.Release) {
}

func (r *actionNotifier) NotifyUpdateRelease(doer *models.User, rel *models.Release) {
}

func (r *actionNotifier) NotifyDeleteRelease(doer *models.User, rel *models.Release) {
}
