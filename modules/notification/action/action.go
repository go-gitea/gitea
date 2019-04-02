// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package action

import (
	"fmt"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
)

type actionNotifier struct {
	base.NullNotifier
}

var (
	_ base.Notifier = &actionNotifier{}
)

// NewNotifier returns a new actionNotifier
func NewNotifier() base.Notifier {
	return &actionNotifier{}
}

func (r *actionNotifier) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	act := &models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		Content:   fmt.Sprintf("%d|%s", issue.Index, strings.Split(comment.Content, "\n")[0]),
		RepoID:    repo.ID,
		Repo:      repo,
		Comment:   comment,
		CommentID: comment.ID,
		IsPrivate: repo.IsPrivate,
	}
	// Check comment type.
	switch comment.Type {
	case models.CommentTypeComment:
		act.OpType = models.ActionCommentIssue
	case models.CommentTypeReopen:
		act.OpType = models.ActionReopenIssue
		if issue.IsPull {
			act.OpType = models.ActionReopenPullRequest
		}
	case models.CommentTypeClose:
		act.OpType = models.ActionCloseIssue
		if issue.IsPull {
			act.OpType = models.ActionClosePullRequest
		}
	}

	if act.OpType > 0 {
		if err := models.NotifyWatchers(act); err != nil {
			log.Error(4, "notifyWatchers: %v", err)
		}
	}
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
	repo := pr.Issue.Repo
	issue := pr.Issue
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    models.ActionMergePullRequest,
		Content:   fmt.Sprintf("%d|%s", issue.Index, issue.Title),
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error(4, "NotifyWatchers: %v", err)
	}
}

func (r *actionNotifier) NotifyCreateRepository(doer *models.User, u *models.User, repo *models.Repository) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    models.ActionCreateRepo,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error(4, "NotifyWatchers: %v", err)
	}
}

func (r *actionNotifier) NotifyRepositoryChangedName(doer *models.User, oldRepoName string, repo *models.Repository) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    models.ActionRenameRepo,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		Content:   oldRepoName,
	}); err != nil {
		log.Error(4, "NotifyWatchers: %v", err)
	}
}

func (r *actionNotifier) NotifyRepositoryTransfered(doer *models.User, oldOwner *models.User, newRepo *models.Repository) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    models.ActionTransferRepo,
		RepoID:    newRepo.ID,
		Repo:      newRepo,
		IsPrivate: newRepo.IsPrivate,
		Content:   path.Join(oldOwner.Name, newRepo.Name),
	}); err != nil {
		log.Error(4, "NotifyWatchers: %v", err)
	}
}

func (r *actionNotifier) NotifyMigrateRepository(doer *models.User, u *models.User, repo *models.Repository) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    models.ActionCreateRepo,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error(4, "NotifyWatchers: %v", err)
	}
}

func (r *actionNotifier) NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    models.ActionCreateRepo,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error(4, "NotifyWatchers: %v", err)
	}
}

func (r *actionNotifier) NotifyRepoMirrorSync(opType models.ActionType, repo *models.Repository, refName string, data []byte) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: repo.OwnerID,
		ActUser:   repo.MustOwner(),
		OpType:    opType,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		RefName:   refName,
		Content:   string(data),
	}); err != nil {
		log.Error(4, "NotifyWatchers: %v", err)
	}
}

func (r *actionNotifier) NotifyCommitsPushed(pusher *models.User, opType models.ActionType, repo *models.Repository, refName string, data []byte) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: pusher.ID,
		ActUser:   pusher,
		OpType:    opType,
		Content:   string(data),
		RepoID:    repo.ID,
		Repo:      repo,
		RefName:   refName,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error(4, "NotifyWatchers: %v", err)
	}
}
