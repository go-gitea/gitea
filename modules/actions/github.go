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

const (
	githubActivityTypeOpened       = "opened"
	githubActivityTypeClosed       = "closed"
	githubActivityTypeReopened     = "reopened"
	githubActivityTypeEdited       = "edited"
	githubActivityTypeAssigned     = "assigned"
	githubActivityTypeUnassigned   = "unassigned"
	githubActivityTypeLabeled      = "labeled"
	githubActivityTypeUnlabeled    = "unlabeled"
	githubActivityTypeMilestoned   = "milestoned"
	githubActivityTypeDemilestoned = "demilestoned"

	githubActivityTypeSynchronize = "synchronize"

	githubActivityTypePublished = "published"
	githubActivityTypeCreated   = "created"
	githubActivityTypeDeleted   = "deleted"
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
	case githubEventRelease:
		return triggedEvent == webhook_module.HookEventRelease

		// TODO: handle rest events

	default:
		return false
	}

}

func isEventActsTypesEmpty(evtActs map[string][]string) bool {
	if len(evtActs) == 0 || len(evtActs["types"]) == 0 {
		return true
	}
	return false
}

func matchByActivityTypes(evtActs map[string][]string, actTypes ...string) bool {
	if isEventActsTypesEmpty(evtActs) {
		return false
	}

	evtActTypes := evtActs["types"]
	for _, actType := range actTypes {
		if util.SliceContainsString(evtActTypes, actType) {
			return true
		}
	}

	return false
}
