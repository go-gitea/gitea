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

	case githubEventIssues:
		// https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#issues
		if isEventActsTypesEmpty(evt.Acts) {
			return util.SliceContains([]webhook_module.HookEventType{
				webhook_module.HookEventIssues,
				webhook_module.HookEventIssueAssign,
				webhook_module.HookEventIssueLabel,
				webhook_module.HookEventIssueMilestone,
			}, triggedEvent)
		}
		switch triggedEvent {
		case webhook_module.HookEventIssues:
			return matchByActivityTypes(evt.Acts, githubActivityTypeEdited, githubActivityTypeOpened, githubActivityTypeClosed, githubActivityTypeReopened)
		case webhook_module.HookEventIssueAssign:
			return matchByActivityTypes(evt.Acts, githubActivityTypeAssigned, githubActivityTypeUnassigned)
		case webhook_module.HookEventIssueLabel:
			return matchByActivityTypes(evt.Acts, githubActivityTypeLabeled, githubActivityTypeUnlabeled)
		case webhook_module.HookEventIssueMilestone:
			return matchByActivityTypes(evt.Acts, githubActivityTypeMilestoned, githubActivityTypeDemilestoned)
		}
		return false

	case githubEventIssueComment:
		return triggedEvent == webhook_module.HookEventIssueComment

	case githubEventPullRequest:
		// https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#pull_request
		if isEventActsTypesEmpty(evt.Acts) {
			return util.SliceContains([]webhook_module.HookEventType{
				webhook_module.HookEventPullRequest,
				webhook_module.HookEventPullRequestSync,
			}, triggedEvent)
		}
		switch triggedEvent {
		case webhook_module.HookEventPullRequest:
			return matchByActivityTypes(evt.Acts, githubActivityTypeEdited, githubActivityTypeOpened, githubActivityTypeClosed, githubActivityTypeReopened)
		case webhook_module.HookEventPullRequestAssign:
			return matchByActivityTypes(evt.Acts, githubActivityTypeAssigned, githubActivityTypeUnassigned)
		case webhook_module.HookEventPullRequestLabel:
			return matchByActivityTypes(evt.Acts, githubActivityTypeLabeled, githubActivityTypeUnlabeled)
		case webhook_module.HookEventPullRequestSync:
			return matchByActivityTypes(evt.Acts, githubActivityTypeSynchronize)
		}
		return false

	case githubEventPullRequestComment:
		return triggedEvent == webhook_module.HookEventPullRequestComment

	case githubEventPullRequestReview:
		return util.SliceContains([]webhook_module.HookEventType{
			webhook_module.HookEventPullRequestReviewApproved,
			webhook_module.HookEventPullRequestReviewComment,
			webhook_module.HookEventPullRequestReviewRejected,
		}, triggedEvent)

	case githubEventPullRequestReviewComment:
		// TODO

	case githubEventPullRequestTarget:
		// TODO

	default:
		return false
	}

	return false
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
