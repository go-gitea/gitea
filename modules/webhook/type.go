// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

// HookEventType is the type of a hook event
type HookEventType string

// Types of hook events
const (
	HookEventCreate                    HookEventType = "create"
	HookEventDelete                    HookEventType = "delete"
	HookEventFork                      HookEventType = "fork"
	HookEventPush                      HookEventType = "push"
	HookEventIssues                    HookEventType = "issues"
	HookEventIssueAssign               HookEventType = "issue_assign"
	HookEventIssueLabel                HookEventType = "issue_label"
	HookEventIssueMilestone            HookEventType = "issue_milestone"
	HookEventIssueComment              HookEventType = "issue_comment"
	HookEventPullRequest               HookEventType = "pull_request"
	HookEventPullRequestAssign         HookEventType = "pull_request_assign"
	HookEventPullRequestLabel          HookEventType = "pull_request_label"
	HookEventPullRequestMilestone      HookEventType = "pull_request_milestone"
	HookEventPullRequestComment        HookEventType = "pull_request_comment"
	HookEventPullRequestReviewApproved HookEventType = "pull_request_review_approved"
	HookEventPullRequestReviewRejected HookEventType = "pull_request_review_rejected"
	HookEventPullRequestReviewComment  HookEventType = "pull_request_review_comment"
	HookEventPullRequestSync           HookEventType = "pull_request_sync"
	HookEventPullRequestReviewRequest  HookEventType = "pull_request_review_request"
	HookEventWiki                      HookEventType = "wiki"
	HookEventRepository                HookEventType = "repository"
	HookEventRelease                   HookEventType = "release"
	HookEventPackage                   HookEventType = "package"
	HookEventStatus                    HookEventType = "status"
	// once a new event added here, please also added to AllEvents() function

	// FIXME: This event should be a group of pull_request_review_xxx events
	HookEventPullRequestReview HookEventType = "pull_request_review"
	// Actions event only
	HookEventSchedule    HookEventType = "schedule"
	HookEventWorkflowJob HookEventType = "workflow_job"
)

func AllEvents() []HookEventType {
	return []HookEventType{
		HookEventCreate,
		HookEventDelete,
		HookEventFork,
		HookEventPush,
		HookEventIssues,
		HookEventIssueAssign,
		HookEventIssueLabel,
		HookEventIssueMilestone,
		HookEventIssueComment,
		HookEventPullRequest,
		HookEventPullRequestAssign,
		HookEventPullRequestLabel,
		HookEventPullRequestMilestone,
		HookEventPullRequestComment,
		HookEventPullRequestReviewApproved,
		HookEventPullRequestReviewRejected,
		HookEventPullRequestReviewComment,
		HookEventPullRequestSync,
		HookEventPullRequestReviewRequest,
		HookEventWiki,
		HookEventRepository,
		HookEventRelease,
		HookEventPackage,
		HookEventStatus,
		HookEventWorkflowJob,
	}
}

// Event returns the HookEventType as an event string
func (h HookEventType) Event() string {
	switch h {
	case HookEventIssues, HookEventIssueAssign, HookEventIssueLabel, HookEventIssueMilestone:
		return "issues"
	case HookEventPullRequest, HookEventPullRequestAssign, HookEventPullRequestLabel, HookEventPullRequestMilestone,
		HookEventPullRequestSync, HookEventPullRequestReviewRequest:
		return "pull_request"
	case HookEventIssueComment, HookEventPullRequestComment:
		return "issue_comment"
	case HookEventPullRequestReviewApproved:
		return "pull_request_approved"
	case HookEventPullRequestReviewRejected:
		return "pull_request_rejected"
	case HookEventPullRequestReviewComment:
		return "pull_request_comment"
	default:
		return string(h)
	}
}

func (h HookEventType) IsPullRequest() bool {
	return h.Event() == "pull_request"
}

// HookType is the type of a webhook
type HookType = string

// Types of webhooks
const (
	GITEA      HookType = "gitea"
	GOGS       HookType = "gogs"
	SLACK      HookType = "slack"
	DISCORD    HookType = "discord"
	DINGTALK   HookType = "dingtalk"
	TELEGRAM   HookType = "telegram"
	MSTEAMS    HookType = "msteams"
	FEISHU     HookType = "feishu"
	MATRIX     HookType = "matrix"
	WECHATWORK HookType = "wechatwork"
	PACKAGIST  HookType = "packagist"
)

// HookStatus is the status of a web hook
type HookStatus int

// Possible statuses of a web hook
const (
	HookStatusNone HookStatus = iota
	HookStatusSucceed
	HookStatusFail
)
