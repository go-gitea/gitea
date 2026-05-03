// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/services/convert"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookService(t *testing.T) {
	unittest.PrepareTestEnv(t)
	t.Run("GetSlackHook", testWebhookGetSlackHook)
	t.Run("PrepareWebhooks", testWebhookPrepare)
	t.Run("PrepareBranchFilterMatch", testWebhookPrepareBranchFilterMatch)
	t.Run("PrepareBranchFilterNoMatch", testWebhookPrepareBranchFilterNoMatch)
	t.Run("WebhookUserMail", testWebhookUserMail)
	t.Run("CheckBranchFilter", testWebhookCheckBranchFilter)
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
