// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
)

// Notifier defines an interface to notify receiver
type Notifier interface {
	Run()
	NotifyCreateIssueComment(*models.User, *models.Repository,
		*models.Issue, *models.Comment)
	NotifyNewIssue(*models.Issue)
	NotifyCloseIssue(*models.Issue, *models.User)
	NotifyMergePullRequest(*models.PullRequest, *models.User, *git.Repository)
	NotifyNewPullRequest(*models.PullRequest)
	NotifyUpdateComment(*models.User, *models.Comment, string)
	NotifyDeleteComment(*models.User, *models.Comment)
	NotifyDeleteRepository(doer *models.User, repo *models.Repository)
	NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository)
	NotifyNewRelease(rel *models.Release)
	NotifyUpdateRelease(doer *models.User, rel *models.Release)
	NotifyDeleteRelease(doer *models.User, rel *models.Release)
}
