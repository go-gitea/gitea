// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	webhook_module "code.gitea.io/gitea/modules/webhook"
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
	githubEventGollum                   = "gollum"
)

// canGithubEventMatch check if the input Github event can match any Gitea event.
func canGithubEventMatch(eventName string, triggedEvent webhook_module.HookEventType) bool {
	switch eventName {
	case githubEventRegistryPackage:
		return triggedEvent == webhook_module.HookEventPackage

	// See https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#gollum
	case githubEventGollum:
		return triggedEvent == webhook_module.HookEventWiki

	case githubEventIssues:
		switch triggedEvent {
		case webhook_module.HookEventIssues,
			webhook_module.HookEventIssueAssign,
			webhook_module.HookEventIssueLabel,
			webhook_module.HookEventIssueMilestone:
			return true

		default:
			return false
		}

	case githubEventPullRequest, githubEventPullRequestTarget:
		switch triggedEvent {
		case webhook_module.HookEventPullRequest,
			webhook_module.HookEventPullRequestSync,
			webhook_module.HookEventPullRequestAssign,
			webhook_module.HookEventPullRequestLabel:
			return true

		default:
			return false
		}

	case githubEventPullRequestReview:
		switch triggedEvent {
		case webhook_module.HookEventPullRequestReviewApproved,
			webhook_module.HookEventPullRequestReviewComment,
			webhook_module.HookEventPullRequestReviewRejected:
			return true

		default:
			return false
		}

	default:
		return eventName == string(triggedEvent)
	}
}
