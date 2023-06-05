// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

// PayloadConvertor defines the interface to convert system webhook payload to external payload
type PayloadConvertor interface {
	api.Payloader
	Create(*api.CreatePayload) (api.Payloader, error)
	Delete(*api.DeletePayload) (api.Payloader, error)
	Fork(*api.ForkPayload) (api.Payloader, error)
	Issue(*api.IssuePayload) (api.Payloader, error)
	IssueComment(*api.IssueCommentPayload) (api.Payloader, error)
	Push(*api.PushPayload) (api.Payloader, error)
	PullRequest(*api.PullRequestPayload) (api.Payloader, error)
	Review(*api.PullRequestPayload, webhook_module.HookEventType) (api.Payloader, error)
	Repository(*api.RepositoryPayload) (api.Payloader, error)
	Release(*api.ReleasePayload) (api.Payloader, error)
	Wiki(*api.WikiPayload) (api.Payloader, error)
}

func convertPayloader(s PayloadConvertor, p api.Payloader, event webhook_module.HookEventType) (api.Payloader, error) {
	switch event {
	case webhook_module.HookEventCreate:
		return s.Create(p.(*api.CreatePayload))
	case webhook_module.HookEventDelete:
		return s.Delete(p.(*api.DeletePayload))
	case webhook_module.HookEventFork:
		return s.Fork(p.(*api.ForkPayload))
	case webhook_module.HookEventIssues, webhook_module.HookEventIssueAssign, webhook_module.HookEventIssueLabel, webhook_module.HookEventIssueMilestone:
		return s.Issue(p.(*api.IssuePayload))
	case webhook_module.HookEventIssueComment, webhook_module.HookEventPullRequestComment:
		pl, ok := p.(*api.IssueCommentPayload)
		if ok {
			return s.IssueComment(pl)
		}
		return s.PullRequest(p.(*api.PullRequestPayload))
	case webhook_module.HookEventPush:
		return s.Push(p.(*api.PushPayload))
	case webhook_module.HookEventPullRequest, webhook_module.HookEventPullRequestAssign, webhook_module.HookEventPullRequestLabel,
		webhook_module.HookEventPullRequestMilestone, webhook_module.HookEventPullRequestSync, webhook_module.HookEventPullRequestReviewRequest:
		return s.PullRequest(p.(*api.PullRequestPayload))
	case webhook_module.HookEventPullRequestReviewApproved, webhook_module.HookEventPullRequestReviewRejected, webhook_module.HookEventPullRequestReviewComment:
		return s.Review(p.(*api.PullRequestPayload), event)
	case webhook_module.HookEventRepository:
		return s.Repository(p.(*api.RepositoryPayload))
	case webhook_module.HookEventRelease:
		return s.Release(p.(*api.ReleasePayload))
	case webhook_module.HookEventWiki:
		return s.Wiki(p.(*api.WikiPayload))
	}
	return s, nil
}
