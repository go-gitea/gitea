// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	webhook_model "code.gitea.io/gitea/models/webhook"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
)

func TestPullRequestCodeCommentWebhook(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

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
	assert.NoError(t, hook.UpdateEvent())
	assert.NoError(t, webhook_model.CreateWebhook(t.Context(), hook))

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pr.LoadIssue(t.Context()))
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 4})
	assert.NoError(t, comment.LoadPoster(t.Context()))

	hookTask := &webhook_model.HookTask{HookID: hook.ID, EventType: webhook_module.HookEventPullRequestReviewComment}
	unittest.AssertNotExistsBean(t, hookTask)

	NewNotifier().PullRequestCodeComment(t.Context(), pr, comment, nil)

	unittest.AssertExistsAndLoadBean(t, hookTask)
}
