// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/nektos/act/pkg/jobparser"
)

const (
	githubEventPullRequest              = "pull_request"
	githubEventPullRequestTarget        = "pull_request_target"
	githubEventPullRequestReviewComment = "pull_request_review_comment"
	githubEventPullRequestReview        = "pull_request_review"
	githubEventRegistryPackage          = "registry_package"
	githubEventCreate                   = "create"
	githubEventDelete                   = "delete"
	githubEventFork                     = "fork"
	githubEventPush                     = "push"
	githubEventIssues                   = "issues"
	githubEventIssueComment             = "issue_comment"
	githubEventRelease                  = "release"
	githubEventPullRequestComment       = "pull_request_comment"
)

func convertFromGithubEvent(evt *jobparser.Event) string {
	switch evt.Name {
	case githubEventPullRequest, githubEventPullRequestTarget, githubEventPullRequestReview,
		githubEventPullRequestReviewComment:
		return string(webhook_module.HookEventPullRequest)
	case githubEventRegistryPackage:
		return string(webhook_module.HookEventPackage)
	case githubEventCreate, githubEventDelete, githubEventFork, githubEventPush,
		githubEventIssues, githubEventIssueComment, githubEventRelease, githubEventPullRequestComment:
		fallthrough
	default:
		return evt.Name
	}
}
