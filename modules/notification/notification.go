// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package notification

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
)

// NotifyReceiver defines an interface to notify receiver
type NotifyReceiver interface {
	Run()
	NotifyCreateIssueComment(*models.User, *models.Repository,
		*models.Issue, *models.Comment)
	NotifyNewIssue(*models.Issue)
	NotifyCloseIssue(*models.Issue, *models.User)
	NotifyMergePullRequest(*models.PullRequest, *models.User, *git.Repository)
	NotifyNewPullRequest(*models.PullRequest)
}

var (
	notifyReceivers []NotifyReceiver
)

// RegisterReceiver providers method to receive notify messages
func RegisterReceiver(receiver NotifyReceiver) {
	go receiver.Run()
	notifyReceivers = append(notifyReceivers, receiver)
}

// NotifyCreateIssueComment notifies issue comment related message to receivers
func NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	for _, receiver := range notifyReceivers {
		receiver.NotifyCreateIssueComment(doer, repo, issue, comment)
	}
}

// NotifyNewIssue notifies new issue to receivers
func NotifyNewIssue(issue *models.Issue) {
	for _, receiver := range notifyReceivers {
		receiver.NotifyNewIssue(issue)
	}
}

// NotifyCloseIssue notifies close issue to receivers
func NotifyCloseIssue(issue *models.Issue, doer *models.User) {
	for _, receiver := range notifyReceivers {
		receiver.NotifyCloseIssue(issue, doer)
	}
}

// NotifyMergePullRequest notifies merge pull request to receivers
func NotifyMergePullRequest(pr *models.PullRequest, doer *models.User, baseGitRepo *git.Repository) {
	for _, receiver := range notifyReceivers {
		receiver.NotifyMergePullRequest(pr, doer, baseGitRepo)
	}
}

// NotifyNewPullRequest notifies new pull request to receivers
func NotifyNewPullRequest(pr *models.PullRequest) {
	for _, receiver := range notifyReceivers {
		receiver.NotifyNewPullRequest(pr)
	}
}
