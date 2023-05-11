// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import "code.gitea.io/gitea/modules/translation"

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
	HookEventWiki                      HookEventType = "wiki"
	HookEventRepository                HookEventType = "repository"
	HookEventRelease                   HookEventType = "release"
	HookEventPackage                   HookEventType = "package"
)

// Event returns the HookEventType as an event string
func (h HookEventType) Event() string {
	switch h {
	case HookEventCreate:
		return "create"
	case HookEventDelete:
		return "delete"
	case HookEventFork:
		return "fork"
	case HookEventPush:
		return "push"
	case HookEventIssues, HookEventIssueAssign, HookEventIssueLabel, HookEventIssueMilestone:
		return "issues"
	case HookEventPullRequest, HookEventPullRequestAssign, HookEventPullRequestLabel, HookEventPullRequestMilestone,
		HookEventPullRequestSync:
		return "pull_request"
	case HookEventIssueComment, HookEventPullRequestComment:
		return "issue_comment"
	case HookEventPullRequestReviewApproved:
		return "pull_request_approved"
	case HookEventPullRequestReviewRejected:
		return "pull_request_rejected"
	case HookEventPullRequestReviewComment:
		return "pull_request_comment"
	case HookEventWiki:
		return "wiki"
	case HookEventRepository:
		return "repository"
	case HookEventRelease:
		return "release"
	}
	return ""
}

func (h HookEventType) LocaleString(locale translation.Locale) string {
	switch h {
	case HookEventCreate:
		return locale.Tr("repo.settings.event_create")
	case HookEventDelete:
		return locale.Tr("repo.settings.event_delete")
	case HookEventFork:
		return locale.Tr("repo.settings.event_fork")
	case HookEventPush:
		return locale.Tr("repo.settings.event_push")
	case HookEventIssues:
		return locale.Tr("repo.settings.event_issues")
	case HookEventIssueAssign:
		return locale.Tr("repo.settings.event_issue_assign")
	case HookEventIssueLabel:
		return locale.Tr("repo.settings.event_issue_label")
	case HookEventIssueMilestone:
		return locale.Tr("repo.settings.event_issue_milestone")
	case HookEventIssueComment:
		return locale.Tr("repo.settings.event_issue_comment")
	case HookEventPullRequest:
		return locale.Tr("repo.settings.event_pull_request")
	case HookEventPullRequestAssign:
		return locale.Tr("repo.settings.event_pull_request_assign")
	case HookEventPullRequestLabel:
		return locale.Tr("repo.settings.event_pull_request_label")
	case HookEventPullRequestMilestone:
		return locale.Tr("repo.settings.event_pull_request_milestone")
	case HookEventPullRequestSync:
		return locale.Tr("repo.settings.event_pull_request_sync")
	case HookEventPullRequestComment:
		return locale.Tr("repo.settings.event_pull_request_comment")
	case HookEventPullRequestReviewApproved:
		return locale.Tr("repo.settings.event_pull_request_review_approved")
	case HookEventPullRequestReviewRejected:
		return locale.Tr("repo.settings.event_pull_request_review_rejected")
	case HookEventPullRequestReviewComment:
		return locale.Tr("repo.settings.event_pull_request_review_comment")
	case HookEventWiki:
		return locale.Tr("repo.settings.event_wiki")
	case HookEventRepository:
		return locale.Tr("repo.settings.event_repository")
	case HookEventRelease:
		return locale.Tr("repo.settings.event_release")
	}
	return locale.Tr("unknow")
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
