// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"code.gitea.io/gitea/modules/util"
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

// canGithubEventMatch check if the input Github event can match any Gitea event.
func canGithubEventMatch(evt *jobparser.Event, triggedEvent webhook_module.HookEventType) bool {
	switch evt.Name {
	case githubEventCreate:
		return triggedEvent == webhook_module.HookEventCreate
	case githubEventDelete:
		return triggedEvent == webhook_module.HookEventDelete
	case githubEventFork:
		return triggedEvent == webhook_module.HookEventFork
	case githubEventPush:
		return triggedEvent == webhook_module.HookEventPush
	case githubEventRegistryPackage:
		return triggedEvent == webhook_module.HookEventPackage
	case githubEventRelease:
		return triggedEvent == webhook_module.HookEventRelease

	case githubEventIssues:
		return util.SliceContains([]webhook_module.HookEventType{
			webhook_module.HookEventIssues,
			webhook_module.HookEventIssueAssign,
			webhook_module.HookEventIssueLabel,
			webhook_module.HookEventIssueMilestone,
		}, triggedEvent)

	case githubEventIssueComment:
		return triggedEvent == webhook_module.HookEventIssueComment

	case githubEventPullRequest, githubEventPullRequestTarget:
		return util.SliceContains([]webhook_module.HookEventType{
			webhook_module.HookEventPullRequest,
			webhook_module.HookEventPullRequestSync,
			webhook_module.HookEventPullRequestAssign,
			webhook_module.HookEventPullRequestLabel,
		}, triggedEvent)

	case githubEventPullRequestComment:
		return triggedEvent == webhook_module.HookEventPullRequestComment

	case githubEventPullRequestReview:
		return util.SliceContains([]webhook_module.HookEventType{
			webhook_module.HookEventPullRequestReviewApproved,
			webhook_module.HookEventPullRequestComment,
			webhook_module.HookEventPullRequestReviewRejected,
		}, triggedEvent)

	case githubEventPullRequestReviewComment:
		return triggedEvent == webhook_module.HookEventPullRequestComment

	default:
		return false
	}
}
