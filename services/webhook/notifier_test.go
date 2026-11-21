// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequestCodeCommentWebhook(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	hook := &webhook_model.Webhook{
		RepoID:      repo.ID,
		URL:         "https://www.example.com/pr-review",
		ContentType: webhook_model.ContentTypeJSON,
		IsActive:    true,
		Type:        webhook_module.GITEA,
		HookEvent: &webhook_module.HookEvent{
			HookEvents: webhook_module.HookEvents{
				webhook_module.HookEventPullRequestReview: true,
			},
		},
	}
	require.NoError(t, hook.UpdateEvent())
	require.NoError(t, webhook_model.CreateWebhook(t.Context(), hook))

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	require.NoError(t, pr.LoadIssue(t.Context()))
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 4})
	require.NoError(t, comment.LoadPoster(t.Context()))

	hookTaskKey := &webhook_model.HookTask{HookID: hook.ID, EventType: webhook_module.HookEventPullRequestReviewComment}
	unittest.AssertNotExistsBean(t, hookTaskKey)

	NewNotifier().PullRequestCodeComment(t.Context(), pr, comment, nil)

	task := unittest.AssertExistsAndLoadBean(t, hookTaskKey)
	var payload api.PullRequestPayload
	require.NoError(t, json.Unmarshal([]byte(task.PayloadContent), &payload))
	assert.Equal(t, api.HookIssueReviewed, payload.Action)
	assert.Equal(t, pr.Issue.Index, payload.Index)
	require.NotNil(t, payload.Sender)
	assert.Equal(t, comment.Poster.ID, payload.Sender.ID)
	require.NotNil(t, payload.PullRequest)
	assert.Equal(t, pr.Issue.Index, payload.PullRequest.Index)
	require.NotNil(t, payload.Review)
	assert.Equal(t, string(webhook_module.HookEventPullRequestReviewComment), payload.Review.Type)
	assert.Equal(t, comment.Content, payload.Review.Content)
}
