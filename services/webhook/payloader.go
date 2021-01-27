// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"code.gitea.io/gitea/models"
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
	Review(*api.PullRequestPayload, models.HookEventType) (api.Payloader, error)
	Repository(*api.RepositoryPayload) (api.Payloader, error)
	Release(*api.ReleasePayload) (api.Payloader, error)
}

func convertPayloader(s PayloadConvertor, p api.Payloader, event models.HookEventType) (api.Payloader, error) {
	switch event {
	case models.HookEventCreate:
		return s.Create(p.(*api.CreatePayload))
	case models.HookEventDelete:
		return s.Delete(p.(*api.DeletePayload))
	case models.HookEventFork:
		return s.Fork(p.(*api.ForkPayload))
	case models.HookEventIssues, models.HookEventIssueAssign, models.HookEventIssueLabel, models.HookEventIssueMilestone:
		return s.Issue(p.(*api.IssuePayload))
	case models.HookEventIssueComment, models.HookEventPullRequestComment:
		pl, ok := p.(*api.IssueCommentPayload)
		if ok {
			return s.IssueComment(pl)
		}
		return s.PullRequest(p.(*api.PullRequestPayload))
	case models.HookEventPush:
		return s.Push(p.(*api.PushPayload))
	case models.HookEventPullRequest, models.HookEventPullRequestAssign, models.HookEventPullRequestLabel,
		models.HookEventPullRequestMilestone, models.HookEventPullRequestSync:
		return s.PullRequest(p.(*api.PullRequestPayload))
	case models.HookEventPullRequestReviewApproved, models.HookEventPullRequestReviewRejected, models.HookEventPullRequestReviewComment:
		return s.Review(p.(*api.PullRequestPayload), event)
	case models.HookEventRepository:
		return s.Repository(p.(*api.RepositoryPayload))
	case models.HookEventRelease:
		return s.Release(p.(*api.ReleasePayload))
	}
	return s, nil
}
