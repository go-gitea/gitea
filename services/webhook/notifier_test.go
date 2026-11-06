// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"
	notify_service "gitea.dev/services/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fireIssueEvents(t *testing.T, notifier notify_service.Notifier, issue *issues_model.Issue) {
	ctx := t.Context()
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	assignee := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 2})
	comment.Issue = issue
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})

	notifier.NewIssue(ctx, issue, nil)
	notifier.CreateIssueComment(ctx, doer, issue.Repo, issue, comment, nil)
	notifier.UpdateComment(ctx, doer, comment, "old")
	notifier.DeleteComment(ctx, doer, comment)
	notifier.IssueChangeLabels(ctx, doer, issue, []*issues_model.Label{label}, nil)
	notifier.IssueClearLabels(ctx, doer, issue)
	notifier.IssueChangeAssignee(ctx, doer, issue, assignee, false, nil)
	notifier.IssueChangeStatus(ctx, doer, "", issue, nil, true)
	notifier.IssueChangeTitle(ctx, doer, issue, "old title")
	notifier.IssueChangeContent(ctx, doer, issue, "old content")
	notifier.IssueChangeMilestone(ctx, doer, issue, 0)
	notifier.DeleteIssue(ctx, doer, issue)
	notifier.IssueChangeConfidential(ctx, doer, issue)
}

func TestWebhookNotifierConfidential(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	require.NoError(t, issue.LoadAttributes(t.Context()))
	hook := &webhook_model.Webhook{
		RepoID:      issue.RepoID,
		URL:         "http://localhost/gitea-webhook-test-confidential",
		ContentType: webhook_model.ContentTypeJSON,
		Events:      `{"send_everything":true}`,
		IsActive:    true,
	}
	require.NoError(t, db.Insert(t.Context(), hook))
	countTasks := func() int {
		return unittest.GetCount(t, &webhook_model.HookTask{HookID: hook.ID})
	}

	t.Run("Confidential", func(t *testing.T) {
		issue.IsConfidential = true
		_, err := db.GetEngine(t.Context()).ID(issue.ID).Cols("is_confidential").Update(issue)
		require.NoError(t, err)
		fireIssueEvents(t, &webhookNotifier{}, issue)
		assert.Zero(t, countTasks())
	})

	t.Run("PublicPayload", func(t *testing.T) {
		issue.IsConfidential = false
		(&webhookNotifier{}).IssueChangeTitle(t.Context(), issue.Poster, issue, "old title")
		task := unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{HookID: hook.ID, EventType: webhook_module.HookEventIssues})
		var payload api.IssuePayload
		require.NoError(t, json.Unmarshal([]byte(task.PayloadContent), &payload))
		require.NotNil(t, payload.Issue)
		assert.False(t, payload.Issue.Confidential)
		assert.Contains(t, task.PayloadContent, `"confidential": false`)
	})
}
