// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
)

// Notifier defines an interface to notify receiver
type Notifier interface {
	Run()

	NotifyCreateRepository(doer *models.User, u *models.User, repo *models.Repository)
	NotifyMigrateRepository(doer *models.User, u *models.User, repo *models.Repository)
	NotifyDeleteRepository(doer *models.User, repo *models.Repository)
	NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository)

	NotifyNewIssue(*models.Issue)
	NotifyIssueChangeStatus(*models.User, *models.Issue, bool)
	NotifyIssueChangeMilestone(doer *models.User, issue *models.Issue)
	NotifyIssueChangeAssignee(doer *models.User, issue *models.Issue, removed bool)
	NotifyIssueChangeContent(doer *models.User, issue *models.Issue, oldContent string)
	NotifyIssueClearLabels(doer *models.User, issue *models.Issue)
	NotifyIssueChangeTitle(doer *models.User, issue *models.Issue, oldTitle string)
	NotifyIssueChangeLabels(doer *models.User, issue *models.Issue,
		addedLabels []*models.Label, removedLabels []*models.Label)

	NotifyNewPullRequest(*models.PullRequest)
	NotifyMergePullRequest(*models.PullRequest, *models.User, *git.Repository)
	NotifyPullRequestReview(*models.PullRequest, *models.Review, *models.Comment)

	NotifyCreateIssueComment(*models.User, *models.Repository,
		*models.Issue, *models.Comment)
	NotifyUpdateComment(*models.User, *models.Comment, string)
	NotifyDeleteComment(*models.User, *models.Comment)

	NotifyNewRelease(rel *models.Release)
	NotifyUpdateRelease(doer *models.User, rel *models.Release)
	NotifyDeleteRelease(doer *models.User, rel *models.Release)
}
