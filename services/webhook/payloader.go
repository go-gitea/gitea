// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	webhook_model "code.gitea.io/gitea/models/webhook"
	api "code.gitea.io/gitea/modules/structs"
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
	Review(*api.PullRequestPayload, webhook_model.HookEventType) (api.Payloader, error)
	Repository(*api.RepositoryPayload) (api.Payloader, error)
	Release(*api.ReleasePayload) (api.Payloader, error)
}

func convertPayloader(s PayloadConvertor, p api.Payloader, event webhook_model.HookEventType) (api.Payloader, error) {
	switch event {
	case webhook_model.HookEventCreate:
		return s.Create(p.(*api.CreatePayload))
	case webhook_model.HookEventDelete:
		return s.Delete(p.(*api.DeletePayload))
	case webhook_model.HookEventFork:
		return s.Fork(p.(*api.ForkPayload))
	case webhook_model.HookEventIssues, webhook_model.HookEventIssueAssign, webhook_model.HookEventIssueLabel, webhook_model.HookEventIssueMilestone:
		return s.Issue(p.(*api.IssuePayload))
	case webhook_model.HookEventIssueComment, webhook_model.HookEventPullRequestComment:
		pl, ok := p.(*api.IssueCommentPayload)
		if ok {
			return s.IssueComment(pl)
		}
		return s.PullRequest(p.(*api.PullRequestPayload))
	case webhook_model.HookEventPush:
		return s.Push(p.(*api.PushPayload))
	case webhook_model.HookEventPullRequest, webhook_model.HookEventPullRequestAssign, webhook_model.HookEventPullRequestLabel,
		webhook_model.HookEventPullRequestMilestone, webhook_model.HookEventPullRequestSync:
		return s.PullRequest(p.(*api.PullRequestPayload))
	case webhook_model.HookEventPullRequestReviewApproved, webhook_model.HookEventPullRequestReviewRejected, webhook_model.HookEventPullRequestReviewComment:
		return s.Review(p.(*api.PullRequestPayload), event)
	case webhook_model.HookEventRepository:
		return s.Repository(p.(*api.RepositoryPayload))
	case webhook_model.HookEventRelease:
		return s.Release(p.(*api.ReleasePayload))
	}
	return s, nil
}
