// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	webhook_module "gitea.dev/modules/webhook"
	"gitea.dev/services/convert"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookService(t *testing.T) {
	unittest.PrepareTestEnv(t)
	t.Run("GetSlackHook", testWebhookGetSlackHook)
	t.Run("PrepareWebhooks", testWebhookPrepare)
	t.Run("AssigneeChanges", testWebhookAssigneeChanges)
	t.Run("PrepareBranchFilterMatch", testWebhookPrepareBranchFilterMatch)
	t.Run("PrepareBranchFilterNoMatch", testWebhookPrepareBranchFilterNoMatch)
	t.Run("WebhookUserMail", testWebhookUserMail)
	t.Run("CheckBranchFilter", testWebhookCheckBranchFilter)
	t.Run("PrepareTestWebhookIgnoresGates", testPrepareTestWebhookIgnoresGates)
}

func testWebhookAssigneeChanges(t *testing.T) {
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	assignee := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	for _, c := range []struct {
		name      string
		issueID   int64
		eventType webhook_module.HookEventType
	}{
		{"issue", 1, webhook_module.HookEventIssueAssign},
		{"pull request", 2, webhook_module.HookEventPullRequestAssign},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, removed := range []bool{false, true} {
				issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: c.issueID})
				require.NoError(t, issue.LoadAttributes(t.Context()))
				hook := &webhook_model.Webhook{
					RepoID:      issue.RepoID,
					URL:         "http://localhost/gitea-webhook-test-assignee-changes",
					ContentType: webhook_model.ContentTypeJSON,
					Events:      `{"send_everything":true}`,
					IsActive:    true,
				}
				require.NoError(t, db.Insert(t.Context(), hook))

				(&webhookNotifier{}).IssueChangeAssignee(t.Context(), doer, issue, assignee, removed, nil)

				task := unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{HookID: hook.ID, EventType: c.eventType})
				var payload struct {
					Action  api.HookIssueAction `json:"action"`
					Changes *api.ChangesPayload `json:"changes"`
				}
				require.NoError(t, json.Unmarshal([]byte(task.PayloadContent), &payload))
				require.NotNil(t, payload.Changes)
				if removed {
					assert.Equal(t, api.HookIssueUnassigned, payload.Action)
					assert.Empty(t, payload.Changes.AddedAssignees)
					require.Len(t, payload.Changes.RemovedAssignees, 1)
					assert.Equal(t, assignee.ID, payload.Changes.RemovedAssignees[0].ID)
				} else {
					assert.Equal(t, api.HookIssueAssigned, payload.Action)
					assert.Empty(t, payload.Changes.RemovedAssignees)
					require.Len(t, payload.Changes.AddedAssignees, 1)
					assert.Equal(t, assignee.ID, payload.Changes.AddedAssignees[0].ID)
				}
			}
		})
	}
}

func testWebhookGetSlackHook(t *testing.T) {
	w := &webhook_model.Webhook{
		Meta: `{"channel": "foo", "username": "username", "color": "blue"}`,
	}
	slackHook := GetSlackHook(w)
	assert.Equal(t, SlackMeta{
		Channel:  "foo",
		Username: "username",
		Color:    "blue",
	}, *slackHook)
}

func testWebhookPrepare(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	hook := &webhook_model.Webhook{
		RepoID:      repo.ID,
		URL:         "http://localhost/gitea-webhook-test-prepare_webhooks",
		ContentType: webhook_model.ContentTypeJSON,
		Events:      `{"push_only":true}`,
		IsActive:    true,
	}
	require.NoError(t, db.Insert(t.Context(), hook))

	hookTask := &webhook_model.HookTask{HookID: hook.ID, EventType: webhook_module.HookEventPush}
	unittest.AssertNotExistsBean(t, hookTask)
	err := PrepareWebhooks(t.Context(), EventSource{Repository: repo}, webhook_module.HookEventPush, &api.PushPayload{Commits: []*api.PayloadCommit{{}}})
	require.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)
}

func testWebhookPrepareBranchFilterMatch(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	hook := &webhook_model.Webhook{
		RepoID:      repo.ID,
		URL:         "http://localhost/gitea-webhook-test-branch_filter_match",
		ContentType: webhook_model.ContentTypeJSON,
		Events:      `{"push_only":true,"branch_filter":"{master,feature*}"}`,
		IsActive:    true,
	}
	require.NoError(t, db.Insert(t.Context(), hook))

	hookTask := &webhook_model.HookTask{HookID: hook.ID, EventType: webhook_module.HookEventPush}
	unittest.AssertNotExistsBean(t, hookTask)
	// this test also ensures that * doesn't handle / in any special way (like shell would)
	err := PrepareWebhooks(t.Context(), EventSource{Repository: repo}, webhook_module.HookEventPush, &api.PushPayload{Ref: "refs/heads/feature/7791", Commits: []*api.PayloadCommit{{}}})
	require.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)
}

func testWebhookPrepareBranchFilterNoMatch(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	hook := &webhook_model.Webhook{
		RepoID:      repo.ID,
		URL:         "http://localhost/gitea-webhook-test-branch_filter_no_match",
		ContentType: webhook_model.ContentTypeJSON,
		Events:      `{"push_only":true,"branch_filter":"{master,feature*}"}`,
		IsActive:    true,
	}
	require.NoError(t, db.Insert(t.Context(), hook))

	hookTask := &webhook_model.HookTask{HookID: hook.ID, EventType: webhook_module.HookEventPush}
	unittest.AssertNotExistsBean(t, hookTask)
	err := PrepareWebhooks(t.Context(), EventSource{Repository: repo}, webhook_module.HookEventPush, &api.PushPayload{Ref: "refs/heads/fix_weird_bug"})
	require.NoError(t, err)
	unittest.AssertNotExistsBean(t, hookTask)
}

func testWebhookUserMail(t *testing.T) {
	defer test.MockVariableValue(&setting.Service.NoReplyAddress, "no-reply.com")()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	assert.Equal(t, user.GetPlaceholderEmail(), convert.ToUser(t.Context(), user, nil).Email)
	assert.Equal(t, user.Email, convert.ToUser(t.Context(), user, user).Email)
}

func testWebhookCheckBranchFilter(t *testing.T) {
	cases := []struct {
		filter string
		ref    git.RefName
		match  bool
	}{
		{"", "any-ref", true},
		{"*", "any-ref", true},
		{"**", "any-ref", true},

		{"main", git.RefNameFromBranch("main"), true},
		{"main", git.RefNameFromTag("main"), false},

		{"feature/*", git.RefNameFromBranch("feature"), false},
		{"feature/*", git.RefNameFromBranch("feature/foo"), true},
		{"feature/*", git.RefNameFromTag("feature/foo"), false},

		{"{refs/heads/feature/*,refs/tags/release/*}", git.RefNameFromBranch("feature/foo"), true},
		{"{refs/heads/feature/*,refs/tags/release/*}", git.RefNameFromBranch("main"), false},
		{"{refs/heads/feature/*,refs/tags/release/*}", git.RefNameFromTag("release/bar"), true},
		{"{refs/heads/feature/*,refs/tags/release/*}", git.RefNameFromTag("dev"), false},
	}
	for _, v := range cases {
		assert.Equal(t, v.match, checkBranchFilter(v.filter, v.ref), "filter: %q ref: %q", v.filter, v.ref)
	}
}

func testPrepareTestWebhookIgnoresGates(t *testing.T) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	hook := &webhook_model.Webhook{
		RepoID:      repo.ID,
		URL:         "http://localhost/gitea-webhook-test-prepare_test_webhook",
		ContentType: webhook_model.ContentTypeJSON,
		IsActive:    true,
		HookEvent: &webhook_module.HookEvent{
			ChooseEvents: true,
			BranchFilter: "dev",
			HookEvents: webhook_module.HookEvents{
				webhook_module.HookEventWorkflowRun: true,
			},
		},
	}
	require.NoError(t, hook.UpdateEvent())
	require.NoError(t, db.Insert(t.Context(), hook))

	payload := &api.PushPayload{
		Ref:     "refs/heads/master",
		Commits: []*api.PayloadCommit{{}},
	}
	hookTask := &webhook_model.HookTask{HookID: hook.ID, EventType: webhook_module.HookEventPush}

	// Real deliveries stay gated: no push event + branch filter mismatch => nothing queued.
	unittest.AssertNotExistsBean(t, hookTask)
	require.NoError(t, PrepareWebhook(t.Context(), hook, webhook_module.HookEventPush, payload))
	unittest.AssertNotExistsBean(t, hookTask)

	// Manual test delivery always queues so the endpoint can be verified.
	require.NoError(t, PrepareTestWebhook(t.Context(), hook, webhook_module.HookEventPush, payload))
	unittest.AssertExistsAndLoadBean(t, hookTask)
}
