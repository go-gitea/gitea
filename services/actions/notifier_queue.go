// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/queue"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"
)

var actionsNotifyQueue *queue.WorkerPoolQueue[*actionsNotifyQueueItem]

type actionsNotifyQueueItem struct {
	Method        string
	RepoID        int64
	DoerID        int64
	Event         webhook_module.HookEventType
	Ref           string
	PullRequestID int64
	PayloadType   string
	PayloadJSON   []byte
}

func actionsNotifyQueueHandler(items ...*actionsNotifyQueueItem) []*actionsNotifyQueueItem {
	for _, item := range items {
		ctx := withMethod(graceful.GetManager().ShutdownContext(), item.Method)
		input, err := item.toNotifyInput(ctx)
		if err != nil {
			log.Error("Unable to restore actions notification %q for repo %d: %v", item.Method, item.RepoID, err)
			continue
		}
		if err := notify(ctx, input); err != nil {
			log.Error("an error occurred while executing the %s actions method: %v", item.Method, err)
		}
	}
	return nil
}

func (input *notifyInput) enqueue(ctx context.Context) error {
	item, err := input.toQueueItem(ctx)
	if err != nil {
		return err
	}
	return actionsNotifyQueue.Push(item)
}

func (input *notifyInput) toQueueItem(ctx context.Context) (*actionsNotifyQueueItem, error) {
	payloadType, payloadJSON, err := marshalActionsPayload(input.Payload)
	if err != nil {
		return nil, err
	}
	item := &actionsNotifyQueueItem{
		Method:      getMethod(ctx),
		RepoID:      input.Repo.ID,
		DoerID:      input.Doer.ID,
		Event:       input.Event,
		Ref:         input.Ref.String(),
		PayloadType: payloadType,
		PayloadJSON: payloadJSON,
	}
	if input.PullRequest != nil {
		item.PullRequestID = input.PullRequest.ID
	}
	return item, nil
}

func (item *actionsNotifyQueueItem) toNotifyInput(ctx context.Context) (*notifyInput, error) {
	repo, err := repo_model.GetRepositoryByID(ctx, item.RepoID)
	if err != nil {
		return nil, err
	}
	doer, err := loadActionsNotifyDoer(ctx, item.DoerID)
	if err != nil {
		return nil, err
	}
	payload, err := unmarshalActionsPayload(item.PayloadType, item.PayloadJSON)
	if err != nil {
		return nil, err
	}
	input := &notifyInput{
		Repo:    repo,
		Doer:    doer,
		Event:   item.Event,
		Ref:     git.RefName(item.Ref),
		Payload: payload,
	}
	if item.PullRequestID > 0 {
		pr, err := issues_model.GetPullRequestByID(ctx, item.PullRequestID)
		if err != nil {
			return nil, err
		}
		if err := pr.LoadIssue(ctx); err != nil {
			return nil, err
		}
		input.PullRequest = pr
		if input.Ref == "" {
			input.Ref = git.RefName(pr.GetGitHeadRefName())
		}
	}
	return input, nil
}

func loadActionsNotifyDoer(ctx context.Context, doerID int64) (*user_model.User, error) {
	if doerID == user_model.ActionsUserID {
		return user_model.NewActionsUser(), nil
	}
	return user_model.GetUserByID(ctx, doerID)
}

const (
	actionsPayloadTypeIssueComment = "issue_comment"
	actionsPayloadTypeIssue        = "issue"
	actionsPayloadTypePullRequest  = "pull_request"
	actionsPayloadTypeRepository   = "repository"
	actionsPayloadTypeFork         = "fork"
	actionsPayloadTypePush         = "push"
	actionsPayloadTypeCreate       = "create"
	actionsPayloadTypeDelete       = "delete"
	actionsPayloadTypeWiki         = "wiki"
	actionsPayloadTypeWorkflowRun  = "workflow_run"
	actionsPayloadTypeRelease      = "release"
	actionsPayloadTypePackage      = "package"
)

func marshalActionsPayload(payload api.Payloader) (string, []byte, error) {
	if payload == nil {
		return "", nil, nil
	}

	var payloadType string
	switch payload.(type) {
	case *api.IssueCommentPayload:
		payloadType = actionsPayloadTypeIssueComment
	case *api.IssuePayload:
		payloadType = actionsPayloadTypeIssue
	case *api.PullRequestPayload:
		payloadType = actionsPayloadTypePullRequest
	case *api.RepositoryPayload:
		payloadType = actionsPayloadTypeRepository
	case *api.ForkPayload:
		payloadType = actionsPayloadTypeFork
	case *api.PushPayload:
		payloadType = actionsPayloadTypePush
	case *api.CreatePayload:
		payloadType = actionsPayloadTypeCreate
	case *api.DeletePayload:
		payloadType = actionsPayloadTypeDelete
	case *api.WikiPayload:
		payloadType = actionsPayloadTypeWiki
	case *api.WorkflowRunPayload:
		payloadType = actionsPayloadTypeWorkflowRun
	case *api.ReleasePayload:
		payloadType = actionsPayloadTypeRelease
	case *api.PackagePayload:
		payloadType = actionsPayloadTypePackage
	default:
		return "", nil, fmt.Errorf("unsupported actions payload type %T", payload)
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", nil, err
	}
	return payloadType, payloadJSON, nil
}

func unmarshalActionsPayload(payloadType string, payloadJSON []byte) (api.Payloader, error) {
	if payloadType == "" {
		return nil, nil // nolint:nilnil // no payload
	}

	var payload api.Payloader
	switch payloadType {
	case actionsPayloadTypeIssueComment:
		payload = new(api.IssueCommentPayload)
	case actionsPayloadTypeIssue:
		payload = new(api.IssuePayload)
	case actionsPayloadTypePullRequest:
		payload = new(api.PullRequestPayload)
	case actionsPayloadTypeRepository:
		payload = new(api.RepositoryPayload)
	case actionsPayloadTypeFork:
		payload = new(api.ForkPayload)
	case actionsPayloadTypePush:
		payload = new(api.PushPayload)
	case actionsPayloadTypeCreate:
		payload = new(api.CreatePayload)
	case actionsPayloadTypeDelete:
		payload = new(api.DeletePayload)
	case actionsPayloadTypeWiki:
		payload = new(api.WikiPayload)
	case actionsPayloadTypeWorkflowRun:
		payload = new(api.WorkflowRunPayload)
	case actionsPayloadTypeRelease:
		payload = new(api.ReleasePayload)
	case actionsPayloadTypePackage:
		payload = new(api.PackagePayload)
	default:
		return nil, fmt.Errorf("unsupported actions payload type %q", payloadType)
	}

	if err := json.Unmarshal(payloadJSON, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
