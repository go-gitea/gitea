// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"

	api "code.gitea.io/sdk/gitea"
)

type webhookReceiver struct {
}

var (
	receiver notification.NotifyReceiver = &webhookReceiver{}
)

func init() {
	notification.RegisterReceiver(receiver)
}

func (w *webhookReceiver) Run() {
}

func (w *webhookReceiver) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	mode, _ := models.AccessLevel(doer.ID, repo)
	if err := models.PrepareWebhooks(repo, models.HookEventIssueComment, &api.IssueCommentPayload{
		Action:     api.HookIssueCommentCreated,
		Issue:      issue.APIFormat(),
		Comment:    comment.APIFormat(),
		Repository: repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err != nil {
		log.Error(2, "PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	} else {
		go models.HookQueue.Add(repo.ID)
	}
}

// NotifyNewIssue implements notification.Receiver
func (w *webhookReceiver) NotifyNewIssue(issue *models.Issue) {
	mode, _ := models.AccessLevel(issue.Poster.ID, issue.Repo)
	if err := models.PrepareWebhooks(issue.Repo, models.HookEventIssues, &api.IssuePayload{
		Action:     api.HookIssueOpened,
		Index:      issue.Index,
		Issue:      issue.APIFormat(),
		Repository: issue.Repo.APIFormat(mode),
		Sender:     issue.Poster.APIFormat(),
	}); err != nil {
		log.Error(4, "PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(issue.RepoID)
	}
}

// NotifyCloseIssue implements notification.Receiver
func (w *webhookReceiver) NotifyCloseIssue(issue *models.Issue, doer *models.User) {
	panic("not implements")
}

func (w *webhookReceiver) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User, baseGitRepo *git.Repository) {
	mode, _ := models.AccessLevel(doer.ID, pr.Issue.Repo)
	if err := models.PrepareWebhooks(pr.Issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
		Action:      api.HookIssueClosed,
		Index:       pr.Index,
		PullRequest: pr.APIFormat(),
		Repository:  pr.Issue.Repo.APIFormat(mode),
		Sender:      doer.APIFormat(),
	}); err != nil {
		log.Error(4, "PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(pr.Issue.Repo.ID)
	}

	l, err := baseGitRepo.CommitsBetweenIDs(pr.MergedCommitID, pr.MergeBase)
	if err != nil {
		log.Error(4, "CommitsBetweenIDs: %v", err)
		return
	}

	// It is possible that head branch is not fully sync with base branch for merge commits,
	// so we need to get latest head commit and append merge commit manually
	// to avoid strange diff commits produced.
	mergeCommit, err := baseGitRepo.GetBranchCommit(pr.BaseBranch)
	if err != nil {
		log.Error(4, "GetBranchCommit: %v", err)
		return
	}

	p := &api.PushPayload{
		Ref:        git.BranchPrefix + pr.BaseBranch,
		Before:     pr.MergeBase,
		After:      mergeCommit.ID.String(),
		CompareURL: setting.AppURL + pr.BaseRepo.ComposeCompareURL(pr.MergeBase, pr.MergedCommitID),
		Commits:    models.ListToPushCommits(l).ToAPIPayloadCommits(pr.BaseRepo.HTMLURL()),
		Repo:       pr.BaseRepo.APIFormat(mode),
		Pusher:     pr.HeadRepo.MustOwner().APIFormat(),
		Sender:     doer.APIFormat(),
	}
	if err := models.PrepareWebhooks(pr.BaseRepo, models.HookEventPush, p); err != nil {
		log.Error(4, "PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(pr.BaseRepo.ID)
	}
}

func (w *webhookReceiver) NotifyNewPullRequest(pr *models.PullRequest) {
	mode, _ := models.AccessLevel(pr.Issue.Poster.ID, pr.Issue.Repo)
	if err := models.PrepareWebhooks(pr.Issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
		Action:      api.HookIssueOpened,
		Index:       pr.Issue.Index,
		PullRequest: pr.APIFormat(),
		Repository:  pr.Issue.Repo.APIFormat(mode),
		Sender:      pr.Issue.Poster.APIFormat(),
	}); err != nil {
		log.Error(4, "PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(pr.Issue.Repo.ID)
	}
}
