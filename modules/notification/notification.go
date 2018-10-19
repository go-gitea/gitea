// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package notification

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/notification/action"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/notification/indexer"
	"code.gitea.io/gitea/modules/notification/mail"
	"code.gitea.io/gitea/modules/notification/ui"
)

var (
	notifiers []base.Notifier
)

// RegisterNotifier providers method to receive notify messages
func RegisterNotifier(notifier base.Notifier) {
	go notifier.Run()
	notifiers = append(notifiers, notifier)
}

func init() {
	RegisterNotifier(ui.NewNotifier())
	RegisterNotifier(mail.NewNotifier())
	RegisterNotifier(indexer.NewNotifier())
	RegisterNotifier(action.NewNotifier())
}

// NotifyCreateIssueComment notifies issue comment related message to notifiers
func NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyCreateIssueComment(doer, repo, issue, comment)
	}
}

// NotifyNewIssue notifies new issue to notifiers
func NotifyNewIssue(issue *models.Issue) {
	for _, notifier := range notifiers {
		notifier.NotifyNewIssue(issue)
	}
}

// NotifyIssueChangeStatus notifies close or reopen issue to notifiers
func NotifyIssueChangeStatus(doer *models.User, issue *models.Issue, closeOrReopen bool) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeStatus(doer, issue, closeOrReopen)
	}
}

// NotifyMergePullRequest notifies merge pull request to notifiers
func NotifyMergePullRequest(pr *models.PullRequest, doer *models.User, baseGitRepo *git.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyMergePullRequest(pr, doer, baseGitRepo)
	}
}

// NotifyNewPullRequest notifies new pull request to notifiers
func NotifyNewPullRequest(pr *models.PullRequest) {
	for _, notifier := range notifiers {
		notifier.NotifyNewPullRequest(pr)
	}
}

// NotifyPullRequestReview notifies new pull request review
func NotifyPullRequestReview(pr *models.PullRequest, review *models.Review, comment *models.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestReview(pr, review, comment)
	}
}

// NotifyUpdateComment notifies update comment to notifiers
func NotifyUpdateComment(doer *models.User, c *models.Comment, oldContent string) {
	for _, notifier := range notifiers {
		notifier.NotifyUpdateComment(doer, c, oldContent)
	}
}

// NotifyDeleteComment notifies delete comment to notifiers
func NotifyDeleteComment(doer *models.User, c *models.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteComment(doer, c)
	}
}

// NotifyDeleteRepository notifies delete repository to notifiers
func NotifyDeleteRepository(doer *models.User, repo *models.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteRepository(doer, repo)
	}
}

// NotifyForkRepository notifies fork repository to notifiers
func NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyForkRepository(doer, oldRepo, repo)
	}
}

// NotifyNewRelease notifies new release to notifiers
func NotifyNewRelease(rel *models.Release) {
	for _, notifier := range notifiers {
		notifier.NotifyNewRelease(rel)
	}
}

// NotifyUpdateRelease notifies update release to notifiers
func NotifyUpdateRelease(doer *models.User, rel *models.Release) {
	for _, notifier := range notifiers {
		notifier.NotifyUpdateRelease(doer, rel)
	}
}

// NotifyDeleteRelease notifies delete release to notifiers
func NotifyDeleteRelease(doer *models.User, rel *models.Release) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteRelease(doer, rel)
	}
}

// NotifyIssueChangeMilestone notifies change milestone to notifiers
func NotifyIssueChangeMilestone(doer *models.User, issue *models.Issue) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeMilestone(doer, issue)
	}
}

// NotifyIssueChangeContent notifies change content to notifiers
func NotifyIssueChangeContent(doer *models.User, issue *models.Issue, oldContent string) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeContent(doer, issue, oldContent)
	}
}

// NotifyIssueChangeAssignee notifies change content to notifiers
func NotifyIssueChangeAssignee(doer *models.User, issue *models.Issue, removed bool) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeAssignee(doer, issue, removed)
	}
}

// NotifyIssueClearLabels notifies clear labels to notifiers
func NotifyIssueClearLabels(doer *models.User, issue *models.Issue) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueClearLabels(doer, issue)
	}
}

// NotifyIssueChangeTitle notifies change title to notifiers
func NotifyIssueChangeTitle(doer *models.User, issue *models.Issue, oldTitle string) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeTitle(doer, issue, oldTitle)
	}
}

// NotifyIssueChangeLabels notifies change labels to notifiers
func NotifyIssueChangeLabels(doer *models.User, issue *models.Issue,
	addedLabels []*models.Label, removedLabels []*models.Label) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeLabels(doer, issue, addedLabels, removedLabels)
	}
}

// NotifyCreateRepository notifies create repository to notifiers
func NotifyCreateRepository(doer *models.User, u *models.User, repo *models.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyCreateRepository(doer, u, repo)
	}
}

// NotifyMigrateRepository notifies migrate repository to notifiers
func NotifyMigrateRepository(doer *models.User, u *models.User, repo *models.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyMigrateRepository(doer, u, repo)
	}
}

// NotifyRepositoryChangedName notifies change repository name to notifiers
func NotifyRepositoryChangedName(doer *models.User, oldRepoName string, repo *models.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyRepositoryChangedName(doer, oldRepoName, repo)
	}
}

// NotifyRepositoryTransfered notifies transfer repository to notifiers
func NotifyRepositoryTransfered(doer *models.User, oldOwner *models.User, newRepo *models.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyRepositoryTransfered(doer, oldOwner, newRepo)
	}
}

// NotifyRepoMirrorSync notifies mirror repository sync to notifiers
func NotifyRepoMirrorSync(opType models.ActionType, repo *models.Repository, refName string, data []byte) {
	for _, notifier := range notifiers {
		notifier.NotifyRepoMirrorSync(opType, repo, refName, data)
	}
}

// NotifyCommitsPushed notifies when commits push to notifiers
func NotifyCommitsPushed(pusher *models.User, opType models.ActionType, repo *models.Repository, refName string, data []byte) {
	for _, notifier := range notifiers {
		notifier.NotifyCommitsPushed(pusher, opType, repo, refName, data)
	}
}
