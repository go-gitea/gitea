// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

const (
	GithubEventPullRequest              = "pull_request"
	GithubEventPullRequestTarget        = "pull_request_target"
	GithubEventPullRequestReviewComment = "pull_request_review_comment"
	GithubEventPullRequestReview        = "pull_request_review"
	GithubEventRegistryPackage          = "registry_package"
	GithubEventCreate                   = "create"
	GithubEventDelete                   = "delete"
	GithubEventFork                     = "fork"
	GithubEventPush                     = "push"
	GithubEventIssues                   = "issues"
	GithubEventIssueComment             = "issue_comment"
	GithubEventRelease                  = "release"
	GithubEventPullRequestComment       = "pull_request_comment"
	GithubEventGollum                   = "gollum"
)

// canGithubEventMatch check if the input Github event can match any Gitea event.
func canGithubEventMatch(eventName string, triggedEvent webhook_module.HookEventType) bool {
	switch eventName {
	case GithubEventRegistryPackage:
		return triggedEvent == webhook_module.HookEventPackage

	// See https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#gollum
	case GithubEventGollum:
		return triggedEvent == webhook_module.HookEventWiki

	case GithubEventIssues:
		switch triggedEvent {
		case webhook_module.HookEventIssues,
			webhook_module.HookEventIssueAssign,
			webhook_module.HookEventIssueLabel,
			webhook_module.HookEventIssueMilestone:
			return true

		default:
			return false
		}

	case GithubEventPullRequest, GithubEventPullRequestTarget:
		switch triggedEvent {
		case webhook_module.HookEventPullRequest,
			webhook_module.HookEventPullRequestSync,
			webhook_module.HookEventPullRequestAssign,
			webhook_module.HookEventPullRequestLabel:
			return true

		default:
			return false
		}

	case GithubEventPullRequestReview:
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
