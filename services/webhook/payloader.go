// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"bytes"
	"fmt"
	"net/http"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

// payloadConvertor defines the interface to convert system payload to webhook payload
type payloadConvertor[T any] interface {
	Create(*api.CreatePayload) (T, error)
	Delete(*api.DeletePayload) (T, error)
	Fork(*api.ForkPayload) (T, error)
	Issue(*api.IssuePayload) (T, error)
	IssueComment(*api.IssueCommentPayload) (T, error)
	Push(*api.PushPayload) (T, error)
	PullRequest(*api.PullRequestPayload) (T, error)
	Review(*api.PullRequestPayload, webhook_module.HookEventType) (T, error)
	Repository(*api.RepositoryPayload) (T, error)
	Release(*api.ReleasePayload) (T, error)
	Wiki(*api.WikiPayload) (T, error)
	Package(*api.PackagePayload) (T, error)
}

func convertUnmarshalledJSON[T, P any](convert func(P) (T, error), data []byte) (t T, err error) {
	var p P
	if err = json.Unmarshal(data, &p); err != nil {
		return t, fmt.Errorf("could not unmarshal payload: %w", err)
	}
	return convert(p)
}

func newPayload[T any](rc payloadConvertor[T], data []byte, event webhook_module.HookEventType) (t T, err error) {
	switch event {
	case webhook_module.HookEventCreate:
		return convertUnmarshalledJSON(rc.Create, data)
	case webhook_module.HookEventDelete:
		return convertUnmarshalledJSON(rc.Delete, data)
	case webhook_module.HookEventFork:
		return convertUnmarshalledJSON(rc.Fork, data)
	case webhook_module.HookEventIssues, webhook_module.HookEventIssueAssign, webhook_module.HookEventIssueLabel, webhook_module.HookEventIssueMilestone:
		return convertUnmarshalledJSON(rc.Issue, data)
	case webhook_module.HookEventIssueComment, webhook_module.HookEventPullRequestComment:
		// previous code sometimes sent s.PullRequest(p.(*api.PullRequestPayload))
		// however I couldn't find in notifier.go such a payload with an HookEvent***Comment event

		// History (most recent first):
		//  - refactored in https://github.com/go-gitea/gitea/pull/12310
		//  - assertion added in https://github.com/go-gitea/gitea/pull/12046
		//  - issue raised in https://github.com/go-gitea/gitea/issues/11940#issuecomment-645713996
		//    > That's because for HookEventPullRequestComment event, some places use IssueCommentPayload and others use PullRequestPayload

		// In modules/actions/workflows.go:183 the type assertion is always payload.(*api.IssueCommentPayload)
		return convertUnmarshalledJSON(rc.IssueComment, data)
	case webhook_module.HookEventPush:
		return convertUnmarshalledJSON(rc.Push, data)
	case webhook_module.HookEventPullRequest, webhook_module.HookEventPullRequestAssign, webhook_module.HookEventPullRequestLabel,
		webhook_module.HookEventPullRequestMilestone, webhook_module.HookEventPullRequestSync, webhook_module.HookEventPullRequestReviewRequest:
		return convertUnmarshalledJSON(rc.PullRequest, data)
	case webhook_module.HookEventPullRequestReviewApproved, webhook_module.HookEventPullRequestReviewRejected, webhook_module.HookEventPullRequestReviewComment:
		return convertUnmarshalledJSON(func(p *api.PullRequestPayload) (T, error) {
			return rc.Review(p, event)
		}, data)
	case webhook_module.HookEventRepository:
		return convertUnmarshalledJSON(rc.Repository, data)
	case webhook_module.HookEventRelease:
		return convertUnmarshalledJSON(rc.Release, data)
	case webhook_module.HookEventWiki:
		return convertUnmarshalledJSON(rc.Wiki, data)
	case webhook_module.HookEventPackage:
		return convertUnmarshalledJSON(rc.Package, data)
	}
	return t, fmt.Errorf("newPayload unsupported event: %s", event)
}

func newJSONRequest[T any](pc payloadConvertor[T], w *webhook_model.Webhook, t *webhook_model.HookTask, withDefaultHeaders bool) (*http.Request, []byte, error) {
	payload, err := newPayload(pc, []byte(t.PayloadContent), t.EventType)
	if err != nil {
		return nil, nil, err
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, nil, err
	}

	method := w.HTTPMethod
	if method == "" {
		method = http.MethodPost
	}

	req, err := http.NewRequest(method, w.URL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if withDefaultHeaders {
		return req, body, addDefaultHeaders(req, []byte(w.Secret), t, body)
	}
	return req, body, nil
}
